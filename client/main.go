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

//const tokenAddress = "0x396764f15ed1467883A9a5B7D42AcFb788CD1826"
const keystorePath = "./keystore"
const password = "superStr0ng"
const passwordFileName = "password.txt"

var ethAddress = ""

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

func fetchRaidenBinary() {
	command := exec.Command("sh", "../install.sh")
	var out bytes.Buffer
	command.Stdout = &out
	//Start command and wait for the result
	err := command.Run()
	if err != nil {
		log.Println(err)
	}

	log.Println(out.String())
}

func startRaidenBinary(binarypath string, address string, ethEndpoint string) {
	log.Printf("Starting Raiden Binary for Address: %v and endpoint: %v", address, ethEndpoint)

	exists, err := os.Stat(binarypath)
	if err != nil || exists.Name() != "raiden-binary" {
		log.Println("Binary not found, fetching from Repo")
		fetchRaidenBinary()
	}

	keyarg := fmt.Sprintf("--keystore-path %v", keystorePath)
	passarg := fmt.Sprintf("--password-file %v", passwordFileName)
	addarg := fmt.Sprintf("--address %v", address)
	endarg := fmt.Sprintf("--eth-rpc-endpoint %v", ethEndpoint)

	command := exec.Command(binarypath)
	command.Args = []string{
		keyarg,
		passarg,
		addarg,
		endarg,
		"--network-id kovan",
		"--environment-type development",
		"--gas-price 20000000000",
		"--api-address 0.0.0.0:7709",
		"--rpccorsdomain all",
		"--accept-disclaimer",
	}
	log.Printf("Starting Raiden Binary with arguments: %v", command.Args)

	var out bytes.Buffer
	command.Stdout = &out
	//Start command but dont wait for the result
	err = command.Start()
	if err != nil {
		log.Printf("raiden binary error: %v", err)
	}
	log.Println(out.String())
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
		return "", errors.New("no keystore files found")
	}

	pass, err := ioutil.ReadFile(passwordFileName)
	if err != nil {
		return "", errors.New("no password file found")
	}

	ks := keystore.NewKeyStore(os.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)

	file := filepath.Join(keystorePath, files[0].Name())
	jsonBytes, err := ioutil.ReadFile(file)
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

			ethAddress, err = loadEthereumAddress(password)
			if err != nil {
				log.Println(err)
				log.Println("Generating new Address..")
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

			//Send Request to Satellite for starting payments
			err := sendRequest("GET", endpoint+ethAddress, "", "application/json")
			if err != nil {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Issue started payment system, please check the log files"))
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
