package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/mux"
	"github.com/stefanbenten/raiden-on-storj/raidenlib"
)

const raidenEndpoint = "0.0.0.0:7709"

//const tokenAddress = "0xd762baF19084256262b3f9164a9183009A9001da"
const keystorePath = "./keystore/"
const password = "superStr0ng"
const passwordFile = "password.txt"

var ethAddress = ""

func fetchRaidenBinary() {
	command := exec.Command("sh", "../install.sh")
	var out bytes.Buffer
	command.Stdout = &out
	//Start command and wait for the result
	err := command.Run()
	if err != nil {
		log.Println(err)
	}

	log.Println("Fetched Raiden Binary successfully")
}

func startRaidenBinary(binarypath string, address string, ethEndpoint string) {
	log.Printf("Starting Raiden Binary for Address: %v and endpoint: %v", address, ethEndpoint)

	exists, err := os.Stat(binarypath)
	if err != nil || exists.Name() != "raiden-binary" {
		log.Println("Binary not found, fetching from Repo")
		fetchRaidenBinary()
	}

	command := exec.Command(binarypath,
		"--keystore-path", keystorePath,
		"--password-file", passwordFile,
		"--address", ethAddress,
		"--eth-rpc-endpoint", ethEndpoint,
		"--network-id", "kovan",
		"--environment-type", "development",
		"--gas-price", "20000000000",
		"--api-address", raidenEndpoint,
		"--rpccorsdomain", "all",
		"--accept-disclaimer",
	)
	log.Printf("Starting Raiden Binary with arguments: %v", command.Args)

	var out bytes.Buffer
	command.Stdout = &out
	//Start command but dont wait for the result
	err = command.Start()
	if err != nil {
		log.Printf("raiden binary error: %v", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
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

	switch r.Method {
	case "GET":
		{
			t, err := template.ParseFiles("./index.html")
			if err != nil {
				log.Println(err)
				return
			}

			//Create Website Data
			Data := struct {
				EthereumAddress string
				Password        string
			}{
				ethAddress,
				password,
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

			if endpoint == "" || ethnode == "" {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Not all parameters provided, please check your request"))
				return
			}

			//Start Raiden Binary
			startRaidenBinary("./raiden-binary", ethAddress, ethnode)
			//Wait for Endpoint to start up
			time.Sleep(20 * time.Second)

			//Send Request to Satellite for starting payments
			_, _, err := raidenlib.SendRequest("GET", endpoint+ethAddress, "", "application/json")
			if err != nil {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Issue starting payment system, please check the log files"))
				return
			}
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("Successfully started payment system"))
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
	fmt.Println("Starting Webserver")
	setupWebserver("0.0.0.0:7710")
}
