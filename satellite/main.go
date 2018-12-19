package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/gorilla/mux"
)

const raidenEndpoint = "http://localhost:5001/api/1/"
const tokenAddress = "faf"
const keystorePath = "../keystore"
const password = "superStr0ng"
const passwordFileName = "password.txt"

var channels map[string]int
var ethAddress string

func sendRequest(method string, url string, message string, contenttype string) (err error) {
	var jsonStr = []byte(message)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", contenttype)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Response Status:", resp.Status)
	fmt.Println("Response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Response Body:", string(body))
	return
}

func startRaidenBinary(binarypath string, address string, ethEndpoint string) {
	log.Printf("Starting Raiden Binary for Address: %v and endpoint: %v", address, ethEndpoint)

	command := exec.Command(binarypath)
	command.Args = []string{
		"--accept-disclaimer",
		fmt.Sprintf("--keystore-path %v", keystorePath),
		fmt.Sprintf("--password-file %V", passwordFileName),
		fmt.Sprintf("--address %v", address),
		fmt.Sprintf("--eth-rpc-endpoint %v", ethEndpoint),
		"--network-id kovan",
		"--environment-type development",
		"--gas-price 20000000000",
		"--api-address 0.0.0.0:7709",
		"--rpccorsdomain all",
	}
	// set var to get the output
	var out bytes.Buffer

	// set the output to our variable
	command.Stdout = &out
	err := command.Run()
	if err != nil {
		log.Println(err)
	}

	fmt.Println(out.String())
}

func createEthereumAddress(password string) (address string) {

	ks := keystore.NewKeyStore(keystorePath, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		log.Println(err)
	}
	passwordfile, err := os.Create(passwordFileName)
	if err != nil {
		log.Println("Unable to create password-file")
	}
	//Write Password to File for Raiden Usage
	_, err = passwordfile.Write([]byte(password))
	if err != nil {
		log.Println(err)
	}
	err = passwordfile.Close()
	if err != nil {
		log.Println(err)
	}
	return account.Address.Hex()
}

func loadEthereumAddress(password string) (address string, err error) {

	files, err := ioutil.ReadDir(keystorePath)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("No Keystore Files found")
	}

	file := filepath.Join(keystorePath, files[0].Name())
	ks := keystore.NewKeyStore(os.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)
	jsonBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	pass, err := ioutil.ReadFile(passwordFileName)
	if err != nil {
		return "", err
	}
	account, err := ks.Import(jsonBytes, string(pass), password)
	if err != nil {
		return "", err
	}
	//Remove temporary keystore file
	if err := os.Remove(file); err != nil {
		log.Fatal(err)
	}
	return account.Address.Hex(), err
}

func sendPayments(receiver string, amount int64) (err error) {
	go func() {
		err = sendRequest("POST", raidenEndpoint+path.Join("payments", tokenAddress, receiver), fmt.Sprintf(`{"amount": %v}`, amount), "application/json")
	}()
	return
}

func setupChannel(receiver string, deposit int64) (channelID int, err error) {
	message := fmt.Sprintf(`{"partner_address": "%v", "token_address": "%v", "total_deposit": %v, "settle_timeout": 500}`, receiver, tokenAddress, deposit)
	err = sendRequest("PUT", raidenEndpoint+"channels", message, "application/json")
	return
}

func handleChannelRequest(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	address := params["paymentAddress"]
	if channels[address] == 0 {
		id, err := setupChannel(address, 50000)
		if err != nil {
			fmt.Println(err)
			return
		}
		channels[address] = id

		err = sendPayments(address, 1337)
		if err != nil {
			fmt.Println(err)
		}
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

func createRaidenEndpoint(ethNode string) {
	ethAddress, err := loadEthereumAddress(password)
	if err != nil {
		log.Println(err)
		ethAddress = createEthereumAddress(password)
	}
	startRaidenBinary("./raiden-binary", ethAddress, ethNode)
}

func setupWebserver(addr string) {
	router := mux.NewRouter()
	//router.HandleFunc("/", getStatus).Methods("GET")
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
}
