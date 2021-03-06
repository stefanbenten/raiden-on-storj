package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/pkg/browser"
	"github.com/stefanbenten/raiden-on-storj/raidenlib"
)

var password = "superStr0ng"
var passwordFile = "password.txt"
var keystorePath = "./keystore/"
var ethAddress = ""
var raidenEndpoint = "0.0.0.0:7709"
var version = "v0.100.2"
var satellite = "http://home.stefan-benten.de:7700/payments/"
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
		ethAddress, err = raidenlib.LoadEthereumAddress(keystorePath, passwordFile)
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
            			<input name="endpoint" type="text" size=40 value="http://home.stefan-benten.de:7700/payments/" {{if .Active}}readonly="readonly"{{end}}><br>
            			ETH Node Address:<br>
            			<input name="ethnode" type="text" size=40 value="http://home.stefan-benten.de:7701/" {{if .Active}}readonly="readonly"{{end}}><br>
            			<hr>
						<button name="function" value="start" type="submit" {{if .Active}}disabled{{end}}>Start Payments!</button>
						<button name="function" value="stop" type="submit" {{if not .Active}}disabled{{end}}>Stop Payments!</button>
						<!--{{if .ChannelInfo}}<button name="function" value="close" type="submit" {{if .Active}}disabled{{end}}>Close Channel!</button>{{end}}-->
        			</form>
					<hr>
					{{if .ChannelInfo }}
					<h3> Channel Information: <h3>
					{{.ChannelInfo}}{{end}}
    			</body>
			</html>`)

			if err != nil {
				log.Println(err)
				return
			}

			//Get Channel Info, if Raiden is running
			if raidenPID != 0 {
				channelinfos, err = getChannelInfo()
				if err != nil {
					log.Println(err)
					return
				}
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
			satellite = r.FormValue("endpoint")
			ethnode := r.FormValue("ethnode")
			function := r.FormValue("function")

			if function == "start" {
				active = true
			} else {
				active = false
			}

			if satellite == "" || ethnode == "" {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Not all parameters provided, please check your request"))
				return
			}

			//Start Raiden Binary if necessary
			if raidenPID == 0 {
				raidenPID = raidenlib.StartRaidenBinary("./raiden-binary", version, keystorePath, passwordFile, ethAddress, ethnode, raidenEndpoint)
				//if PID is still 0, there is an issue
				if raidenPID == 0 {
					log.Println("error: unable to start Raiden Binary")
					w.WriteHeader(500)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte("Issue starting payment system, please check the log files"))
					return
				}
			}
			//Send Request to Satellite for interaction
			status, body, err := raidenlib.SendRequest("GET", satellite+path.Join(function, ethAddress), "", "application/json")
			if err != nil {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Issue starting payment system, please check the log files"))
				return
			}
			log.Printf("Successfully executed channel request for payments: %v ", function)
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

func debugHandler() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGQUIT)
	for range sigs {
		log.Println("Got SIGQUIT, stopping payments")
		//TODO: Stop all payments before shutting down
	}
}

func main() {
	go debugHandler()

	direct := flag.Bool("direct", false, "Direct Payment Start with default Endpoints")
	skip := flag.Bool("skip", false, "Set to true if Raiden Binary is running already")
	override := flag.Bool("override", false, "Delete existing KeyStore and generate a new one")
	endpoint := flag.String("endpoint", "http://home.stefan-benten.de:7700/payments/", "Satellite Payment Endpoint")
	ethnode := flag.String("ethnode", "http://home.stefan-benten.de:7701/", "Ethereum Node Endpoint")
	listen := flag.String("listen", "0.0.0.0:7710", "Listen Address for Raiden Endpoint")
	raiden := flag.String("listen-raiden", "0.0.0.0:7709", "Listen Address for Raiden Endpoint")
	ver := flag.String("version", "v0.100.2", "Raiden Binary Version")
	keystore := flag.String("keystore", "./keystore", "Keystore Path")
	pw := flag.String("password", "superStr0ng", "Password used for Keystore encryption")
	flag.Parse()

	//Set global variables
	raidenEndpoint = *raiden
	keystorePath = *keystore
	password = *pw
	version = *ver

	// if Override is requested, it deletes all existing keystore files
	if *override {
		err := os.RemoveAll(keystorePath)
		if err != nil {
			log.Fatalln("Could not delete keystore files, due to:", err)
		}
	}

	if *skip {
		//set temporary RaidenPID for now
		//ToDo: Fetch PID from system
		raidenPID = -1
	}

	//Load Ethereum Address or generate a new one
	prepareETHAddress()

	//When using the direct flag start Raiden directly and request payments, else open an browser interface for interaction
	if *direct {
		//Start Raiden Binary
		log.Println("Direct Flag set, starting Raiden Binary...")
		if raidenPID == 0 {
			raidenPID = raidenlib.StartRaidenBinary("./raiden-binary", version, *keystore, *pw, ethAddress, *ethnode, *raiden)
			if raidenPID == 0 {
				log.Fatalln("error: unable to start Raiden Binary")
				return
			}
		}
		_, _, err := raidenlib.SendRequest("GET", *endpoint+path.Join("start", ethAddress), "", "application/json")
		if err != nil {
			log.Fatalln(err)
		}
		active = true
	} else {
		log.Println("Opening Website for User Interaction")
		err := browser.OpenURL("http://" + *listen)
		if err != nil {
			log.Println("error opening webbrowser window")
		}
	}
	log.Printf("Starting Webserver on address: %v", *listen)
	setupWebserver(*listen)
}
