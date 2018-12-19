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

func startRaidenBinary(path string) {
	command := exec.Command(path)
	command.Args = []string{
		"--keystore-path ../keystore",
		"--password-file password",
		"--network-id kovan",
		"--environment-type development",
		"--gas-price 20000000000",
		"--eth-rpc-endpoint http://home.stefan-benten.de:7701",
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
		log.Fatal(err)
	}
	passwordfile, err := os.Create(password)
	if err != nil {
		log.Println("Unable to create passwordfile")
	}
	passwordfile.Write([]byte(password))
	passwordfile.Close()
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
	pass, err := ioutil.ReadFile(file)
	account, err := ks.Import(jsonBytes, string(pass), password)
	if err != nil {
		return "", err
	}

	if err := os.Remove(file); err != nil {
		log.Fatal(err)
	}
	return account.Address.Hex(), err
}

func handleIndex(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		{
			t, err := template.ParseFiles("./client/index.html")
			if err != nil {
				log.Println(err)
				return
			}
			//Fetch or Generate Ethereum address

			address, err := loadEthereumAddress(password)
			if err != nil {
				log.Println(err)
				address = createEthereumAddress(password)
			}
			//Create Website Data
			Data := struct {
				EthereumAddress string
				Password        string
			}{
				address,
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

		}
	}
}

func setupWebserver(addr string) {
	router := mux.NewRouter()
	//router.HandleFunc("/", getStatus).Methods("GET")
	router.HandleFunc("/", handleIndex).Methods("GET")

	http.ListenAndServe(addr, router)
}

func main() {
	fmt.Println("Starting Webserver")
	startRaidenBinary("./raiden-binary")
	setupWebserver("0.0.0.0:7710")
}
