package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/gorilla/mux"
	"github.com/pkg/browser"
	"github.com/stefanbenten/raiden-on-storj/raidenlib"
)

var password = "superStr0ng"
var passwordFile = "password.txt"
var keystorePath = "./keystore/"
var ethAddress = ""
var raidenEndpoint = "0.0.0.0:7709"
var raidenPID = 0
var active = false

func getChannelInfo() (info string, err error) {
	status, body, err := raidenlib.SendRequest("GET", "http://"+raidenEndpoint+"/api/v1/channels", "{}", "application/json")
	if status == http.StatusOK {
		return body, nil
	}
	if err == nil {
		err = errors.New(fmt.Sprintf("Channel Info Query failed with Status %v and error: %v", status, err))
	}
	return "", err
}

func prepareETHAddress() {
	var err error
	//Fetch or Generate Ethereum address
	if ethAddress == "" {
		ethAddress, err = raidenlib.LoadEthereumAddress(keystorePath, password, passwordFile)
		if err != nil {
			log.Println(err)
			log.Println("Generating new Address..")
			ethAddress = raidenlib.CreateEthereumAddress(keystorePath, password, passwordFile)
		}
		log.Printf("Using Ethereum Address %v", ethAddress)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	var channelinfos = ""
	switch r.Method {
	case "GET":
		{
			t, err := template.New("test").Parse(`
			<html>
			    <head>
			        <title>Storj Payment Demo</title>
				</head>
    			<body>
        			<h2>The ETH Address used for your Payments is: {{.EthereumAddress}}</h2>
        			<h3>using password: {{.Password}}</h3>
        			<form action="/" method="POST">
            			Satellite Payment Endpoint:<br>
            			<input name="endpoint" type="text" size=40 value="http://home.stefan-benten.de:7700/payments/"><br>
            			ETH Node Address:<br>
            			<input name="ethnode" type="text" size=40 value="http://home.stefan-benten.de:7701/"><br>
            			<hr>
						<button name="function" value="start" type="submit" {{if .Active}}disabled{{end}}>Start Payments!</button>
						<button name="function" value="stop" type="submit" {{if not .Active}}disabled{{end}}>Stop Payments!</button>
						<button name="function" value="close" type="submit" {{if not .Active}}disabled{{end}}>Close Channel!</button>
        			</form>
					<hr>
					{{if .ChannelInfo }}
					<h3> Channel Information: <h3>
					{{.ChannelInfo}}{{end}}
    			</body>
			</html>`)
			if raidenPID != 0 {
				channelinfos, err = getChannelInfo()
				if err != nil {
					log.Println(err)
					return
				}
			}
			//t, err := template.ParseFiles("./index.html")
			if err != nil {
				log.Println(err)
				return
			}

			//Create Website Data
			Data := struct {
				EthereumAddress string
				Password        string
				ChannelInfo     string
				Active          bool
			}{
				ethAddress,
				password,
				channelinfos,
				active,
			}
			//Show Website
			err = t.Execute(w, Data)
			if err != nil {
				log.Println(err)
				return
			}
		}
	case "POST":
		{
			endpoint := r.FormValue("endpoint")
			ethnode := r.FormValue("ethnode")
			function := r.FormValue("function")
			if function == "start" {
				active = true
			} else {
				active = false
			}

			if endpoint == "" || ethnode == "" {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Not all parameters provided, please check your request"))
				return
			}

			//Start Raiden Binary
			if raidenPID == 0 {
				raidenPID = raidenlib.StartRaidenBinary("./raiden-binary", keystorePath, passwordFile, ethAddress, ethnode, raidenEndpoint)
			}
			//Send Request to Satellite for starting payments
			status, body, err := raidenlib.SendRequest("GET", endpoint+path.Join(function, ethAddress), "", "application/json")
			if err != nil {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Issue starting payment system, please check the log files"))
				return
			}
			w.WriteHeader(status)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}
	}
}

func setupWebserver(addr string) {
	router := mux.NewRouter()
	//router.HandleFunc("/", getStatus).Methods("GET")
	router.HandleFunc("/", handleIndex).Methods("GET", "POST")
	err := http.ListenAndServe(addr, router)
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	skip := flag.Bool("direct", false, "Direct Payment Start with default Endpoints")
	override := flag.Bool("override", false, "Delete existing KeyStore and generate a new one")
	endpoint := flag.String("endpoint", "http://home.stefan-benten.de:7700/payments/", "Satellite Payment Endpoint")
	ethnode := flag.String("ethnode", "http://home.stefan-benten.de:7701/", "Ethereum Node Endpoint")
	raidenEndpoint = *flag.String("listen-raiden", "0.0.0.0:7709", "Listen Address for Raiden Endpoint")
	listen := *flag.String("listen", "0.0.0.0:7710", "Listen Address for Raiden Endpoint")
	keystorePath = *flag.String("keystore", "./keystore", "Keystore Path")
	password = *flag.String("password", "superStr0ng", "Password used for Keystore encryption")

	flag.Parse()

	if *override {
		err := os.RemoveAll(keystorePath)
		if err != nil {
			log.Fatalln("Could not delete keystore files, due to:", err)
		}
	}
	prepareETHAddress()
	if *skip {
		//Start Raiden Binary
		raidenlib.StartRaidenBinary("./raiden-binary", keystorePath, passwordFile, ethAddress, *ethnode, raidenEndpoint)
		_, _, err := raidenlib.SendRequest("GET", *endpoint+path.Join("start", ethAddress), "", "application/json")
		if err != nil {
			log.Fatalln(err)
		}
		active = true
	} else {
		//If not starting directly, open the interface
		log.Println("Opening Website for User Interaction")
		_ = browser.OpenURL("http://" + listen)
	}
	log.Printf("Starting Webserver on address: %v", listen)
	setupWebserver(listen)
}
