package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/gorilla/mux"
)

const raidenEndpoint = "http://localhost:5001/api/1/"
const tokenAddress = "0x396764f15ed1467883A9a5B7D42AcFb788CD1826"
const keystorePath = "../keystore"
const password = "superStr0ng"
const passwordFileName = "password.txt"

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

func handleIndex(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		{
			t, err := template.ParseFiles("./index.html")
			if err != nil {
				log.Println(err)
				return
			}
			//Fetch or Generate Ethereum address

			ethAddress, err := loadEthereumAddress(password)
			if err != nil {
				log.Println(err)
				ethAddress = createEthereumAddress(password)
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
			log.Println("got:", endpoint, ethnode)
			//Start Raiden Binary
			startRaidenBinary("./raiden-binary", ethAddress, ethnode)
			sendRequest("GET", endpoint+ethAddress, "", "application/json")

			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Successfully started Payment System"))
		}
	}
}

func setupWebserver(addr string) {
	router := mux.NewRouter()
	//router.HandleFunc("/", getStatus).Methods("GET")
	router.HandleFunc("/", handleIndex).Methods("GET", "POST")

	http.ListenAndServe(addr, router)
}

func main() {
	fmt.Println("Starting Webserver")
	setupWebserver("0.0.0.0:7710")
}
