package raidenlib

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

func FetchRaidenBinary() {
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

func StartRaidenBinary(binarypath string, keystorePath string, passwordFile string, address string, ethEndpoint string, listenAddr string) (pid int) {

	log.Println(binarypath, keystorePath, passwordFile, address, ethEndpoint, listenAddr)
	log.Printf("Starting Raiden Binary for Address: %v and endpoint: %v on listen Address: %v", address, ethEndpoint, listenAddr)

	exists, err := os.Stat(binarypath)
	if err != nil || exists.Name() != "raiden-binary" {
		log.Println("Binary not found, fetching from Repo")
		FetchRaidenBinary()
	}

	command := exec.Command(binarypath,
		"--keystore-path", keystorePath,
		"--password-file", passwordFile,
		"--address", address,
		"--eth-rpc-endpoint", ethEndpoint,
		"--network-id", "kovan",
		"--environment-type", "development",
		"--gas-price", "20000000000",
		"--api-address", listenAddr,
		"--rpccorsdomain", "all",
		"--accept-disclaimer",
	)

	var out, errs bytes.Buffer
	command.Stdout = &out
	command.Stderr = &errs

	//Start command but dont wait for the result
	err = command.Start()
	if err != nil {
		log.Printf("raiden binary error: %v", err)
	}
	pid = command.Process.Pid
	if pid == 0 {
		log.Println("Raiden STDOUT:", out.String())
		log.Println("Raiden ERROUT:", errs.String())
	}

	//Check if Raiden Node is online
	var down = true
	var counter = 0
	resp := fmt.Sprintf(`{"our_address": "%v"}`, address)
	for down {
		_, err := os.FindProcess(pid)
		if err != nil {
			log.Println("Raiden process died, please check Raiden logs")
			return 0
		}
		// After waiting 5 minutes error out
		if counter >= 300 {
			return 0
		}
		//give proactive response if process isnt died yet
		if counter%5 == 0 {
			log.Println("Raiden Startup is ongoing..")
		}
		counter++
		time.Sleep(time.Second)
		status, body, err := SendRequest("GET", "http://"+listenAddr+"/api/v1/address", "", "application/json")
		if status == http.StatusOK && err == nil {
			if body == resp {
				down = false
			}
		}
	}
	log.Printf("Started Raiden Binary with PID %v", pid)
	return
}

func SendRequest(method string, url string, message string, contenttype string) (statuscode int, body string, err error) {
	var jsonStr = []byte(message)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", contenttype)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	statuscode = resp.StatusCode
	bbody, _ := ioutil.ReadAll(resp.Body)
	body = string(bbody)
	return
}

func CreateEthereumAddress(keystorePath string, password string, passwordFileName string) (address string) {

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

func LoadEthereumAddress(keystorePath string, password string, passwordFileName string) (address string, err error) {

	files, err := ioutil.ReadDir(keystorePath)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("no keystore files found")
	}
	if len(files) > 1 {
		log.Println("multiple keystore files found, using first one")
	}
	//Read Password file
	pass, err := ioutil.ReadFile(passwordFileName)
	if err != nil {
		return "", errors.New("no password file found")
	}
	//Create Keystore for the account
	ks := keystore.NewKeyStore(os.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)

	//Get Account
	file := filepath.Join(keystorePath, files[0].Name())
	jsonBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	//Import keystone file into KeyStore
	account, err := ks.Import(jsonBytes, string(pass), password)
	if err != nil {
		return "", err
	}
	return account.Address.Hex(), err
}
