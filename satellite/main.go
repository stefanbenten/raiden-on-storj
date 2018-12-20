package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"raiden-on-storj/lib"
)

const raidenEndpoint = "http://127.0.0.1:7709/api/v1/"
const tokenAddress = "0x396764f15ed1467883A9a5B7D42AcFb788CD1826"
const keystorePath = "./keystore"
const password = "superStr0ng"
const passwordFile = "password.txt"

var channels map[string]int
var ticker *time.Ticker
var quit chan struct{}

func fetchRaidenBinary() {
	command := exec.Command("sh", "../install.sh")
	var out bytes.Buffer
	command.Stdout = &out
	//Start command and wait for the result
	err := command.Run()
	if err != nil {
		log.Println(err)
	}

	log.Println("Successfully got Raiden Binary")
}

func startRaidenBinary(binarypath string, address string, ethEndpoint string) {
	log.Printf("Starting Raiden Binary for Address: %v and endpoint: %v", address, ethEndpoint)

	exists, err := os.Stat(binarypath)
	if err != nil || exists.Name() != "raiden-binary" {
		log.Println("Binary not found, fetching from Repo")
		fetchRaidenBinary()
	}

	u, _ := url.Parse(raidenEndpoint)

	command := exec.Command(binarypath,
		"--accept-disclaimer",
		"--keystore-path", keystorePath,
		"--password-file", passwordFile,
		"--address", address,
		"--eth-rpc-endpoint", ethEndpoint,
		"--network-id", "kovan",
		"--environment-type", "development",
		"--gas-price", "20000000000",
		"--api-address", u.Host,
		"--rpccorsdomain", "all",
	)
	// set var to get the output
	var out bytes.Buffer

	// set the output to our variable
	command.Stdout = &out
	err = command.Start()
	if err != nil {
		log.Println(err)
	}
}

func sendPayments(receiver string, amount int64) (err error) {
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Printf("Sending Payment to %v", receiver)
				_, _, err = lib.SendRequest("POST", raidenEndpoint+path.Join("payments", tokenAddress, receiver), fmt.Sprintf(`{"amount": %v}`, amount), "application/json")
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	return
}

func setupChannel(receiver string, deposit int64) (channelID int, err error) {
	var jsonr map[string]string

	log.Printf("Setting up Channel for %v with balance of %v", receiver, deposit)

	message := fmt.Sprintf(`{
			"partner_address": "%v", 
			"token_address": "%v", 
			"total_deposit": %v, 
			"settle_timeout": 500}`,
		receiver,
		tokenAddress,
		deposit,
	)

	status, body, err := lib.SendRequest("PUT", raidenEndpoint+"channels", message, "application/json")
	if status == http.StatusCreated {
		json.Unmarshal([]byte(body), jsonr)
		if jsonr["partner_address"] == receiver {
			log.Println("Channel setup successfully for %v with balance of %v", receiver, deposit)
			channelID, err = strconv.Atoi(jsonr["channel_identifier"])
			return
		}
	}
	return 0, err
}

func handleChannelRequest(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	address := params["paymentAddress"]
	if channels[address] == 0 {
		id, err := setupChannel(address, 5000000000)
		if err != nil {
			fmt.Println(err)
			return
		}
		channels[address] = id

		err = sendPayments(address, 1337)
		if err != nil {
			fmt.Println(err)
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`"status":"Opened Channel successfully"`))
		return
	}
	err := closeChanel(address)
	if err != nil {
		fmt.Println(err)
	}
}

func closeChanel(receiver string) (err error) {
	return
}

func stopPayments(w http.ResponseWriter, r *http.Request) {
	close(quit)
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("Successfully stopped payments"))
}

func createRaidenEndpoint(ethNode string) {
	ethAddress, err := lib.LoadEthereumAddress(keystorePath, password, passwordFile)
	if err != nil {
		log.Println(err)
		ethAddress = lib.CreateEthereumAddress(keystorePath, password, passwordFile)
	}
	log.Printf("Loaded Account: %v successfully", ethAddress)

	startRaidenBinary("./raiden-binary", ethAddress, ethNode)
	//Wait for Binary to start up
	time.Sleep(20 * time.Second)
}

func setupWebserver(addr string) {
	router := mux.NewRouter()
	router.HandleFunc("/stop", stopPayments).Methods("GET")
	router.HandleFunc("/{paymentAddress}", handleChannelRequest).Methods("GET")
	err := http.ListenAndServe(addr, router)
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	fmt.Println("Starting Webserver")
	createRaidenEndpoint("http://home.stefan-benten.de:7701")
	setupWebserver("0.0.0.0:7700")

	ticker = time.NewTicker(5 * time.Second)
	quit = make(chan struct{})
}
