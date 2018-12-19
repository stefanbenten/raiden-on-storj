package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/gorilla/mux"
)

const raidenEndpoint = "http://localhost:5001/api/1/"
const tokenAddress = "faf"

var channels map[string]int

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

func setupWebserver(addr string) {
	router := mux.NewRouter()
	//router.HandleFunc("/", getStatus).Methods("GET")
	router.HandleFunc("/{paymentAddress}", handleChannelRequest).Methods("GET")
	http.ListenAndServe(addr, router)
}

func main() {
	fmt.Println("Starting Webserver")
	setupWebserver("0.0.0.0:7700")
}
