package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"github.com/stefanbenten/raiden-on-storj/raidenlib"
)

const raidenEndpoint = "http://0.0.0.0:7709/api/v1/"
const tokenAddress = "0xd762baF19084256262b3f9164a9183009A9001da"
const keystorePath = "./keystore"
const password = "superStr0ng"
const passwordFile = "password.txt"

var channels = map[string]int{}
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
				statuscode, body, err := raidenlib.SendRequest("POST", raidenEndpoint+path.Join("payments", tokenAddress, receiver), fmt.Sprintf(`{"amount": %v}`, amount), "application/json")
				if err != nil {
					return
				}
				if statuscode == http.StatusPaymentRequired {
					var jsonr map[string]string
					err = json.Unmarshal([]byte(body), jsonr)
					if err != nil {
						return
					}
					log.Println(body)
					//log.Printf("Channel Balance of ID %v insufficient (is: %v, need: %v), refunding..", channels[receiver], jsonr["balance"], amount)
					err = raiseChannelFunds(receiver, 5000000000)
				}
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

	status, body, err := raidenlib.SendRequest("PUT", raidenEndpoint+"channels", message, "application/json")
	if status == http.StatusCreated {
		err = json.Unmarshal([]byte(body), jsonr)
		if jsonr["partner_address"] == receiver && err == nil {
			log.Printf("Channel setup successfully for %v with balance of %v", receiver, deposit)
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
		log.Printf("No Channel with %v found, creating...", address)
		id, err := setupChannel(address, 5000000000)
		if err != nil {
			fmt.Println(err)
			return
		}
		log.Printf("Channel with %v created, ID is %v", address, id)
		channels[address] = id

		err = sendPayments(address, 1337)
		if err != nil {
			fmt.Println(err)
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"status":"Opened Channel successfully"`))
		return
	}
	log.Printf("Found Channel with ID %v, closing...", channels[address])
	err := closeChannel(address)
	if err != nil {
		fmt.Println(err)
	}
}

func raiseChannelFunds(receiver string, total_deposit int64) (err error) {
	var jsonr map[string]string
	message := fmt.Sprintf(`{"total_deposit": "%v"}`, total_deposit)
	status, body, err := raidenlib.SendRequest("PATCH", raidenEndpoint+"channels", message, "application/json")
	if status == http.StatusOK {
		err = json.Unmarshal([]byte(body), jsonr)
		if jsonr["partner_address"] != receiver && err == nil {

		}
	}
	return
}

func closeChannel(receiver string) (err error) {
	var jsonr map[string]string
	message := `{"state": "closed"}`
	status, body, err := raidenlib.SendRequest("PATCH", raidenEndpoint+"channels", message, "application/json")
	if status == http.StatusOK {
		err = json.Unmarshal([]byte(body), jsonr)
		if err != nil && jsonr["state"] != "closed" && jsonr["partner_address"] == receiver {
			return errors.New("unable to close channel! Please check the Raiden log files")
		}
	}
	return
}

func stopPayments(w http.ResponseWriter, r *http.Request) {
	close(quit)
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("Successfully stopped payments"))
}

func createRaidenEndpoint(ethNode string) {
	ethAddress, err := raidenlib.LoadEthereumAddress(keystorePath, password, passwordFile)
	if err != nil {
		log.Println(err)
		ethAddress = raidenlib.CreateEthereumAddress(keystorePath, password, passwordFile)
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
