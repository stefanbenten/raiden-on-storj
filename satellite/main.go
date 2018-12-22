package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/stefanbenten/raiden-on-storj/raidenlib"
)

const raidenEndpoint = "http://0.0.0.0:7709/api/v1/"
const tokenAddress = "0xd762baF19084256262b3f9164a9183009A9001da"
const keystorePath = "./keystore"
const password = "superStr0ng"
const passwordFile = "password.txt"

var channels = map[string]int64{}
var ticker *time.Ticker
var quit chan struct{}
var lock *sync.Mutex

func sendPayments(receiver string, amount int64) (err error) {
	go func() {
		active := true
		for active {
			select {
			case t := <-ticker.C:
				log.Printf("Sending Payment to %v at: %s", receiver, t.Format("2006-01-02 15:04:05 +0800"))
				statuscode, body, err := raidenlib.SendRequest("POST", raidenEndpoint+path.Join("payments", tokenAddress, receiver), fmt.Sprintf(`{"amount": %v}`, amount), "application/json")
				if err != nil {
					return
				}
				if statuscode == http.StatusPaymentRequired {
					var jsonr map[string]interface{}
					err = json.Unmarshal([]byte(body), &jsonr)
					if err != nil {
						return
					}
					log.Println(body)
					//log.Printf("Channel Balance of ID %v insufficient (is: %v, need: %v), refunding..", channels[receiver], jsonr["balance"], amount)
					err = raiseChannelFunds(receiver, 5000000000)
				}
			case <-quit:
				log.Printf("Stopped Payments to %v", receiver)
				active = false
				return
			}
		}
	}()
	return
}

func getChannelInfo(receiver string) (info string, err error) {
	log.Println(raidenEndpoint + path.Join("channels", tokenAddress, receiver))
	status, body, err := raidenlib.SendRequest("GET", raidenEndpoint+path.Join("channels", tokenAddress, receiver), "", "application/json")
	log.Println(status, body)
	//if status == http.StatusOK {
	return body, nil
	/*}
	if err == nil {
		err = errors.New(fmt.Sprintf("Query failed with Status %v", status))
	}
	return "", err*/
}

func setupChannel(receiver string, deposit int64) (channelID int64, err error) {
	var jsonr map[string]interface{}

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
	log.Println(status, body)
	if status == http.StatusCreated {
		err = json.Unmarshal([]byte(body), &jsonr)
		if jsonr["partner_address"] == receiver && err == nil {
			log.Printf("Channel setup successfully for %v with balance of %v", receiver, deposit)
			channelID = int64(jsonr["channel_identifier"].(float64))
			return
		}
	}
	if err == nil {
		err = errors.New(body)
	}
	return 0, err
}

func raiseChannelFunds(receiver string, totalDeposit int64) (err error) {
	var jsonr map[string]interface{}
	message := fmt.Sprintf(`{"total_deposit": "%v"}`, totalDeposit)
	status, body, err := raidenlib.SendRequest("PATCH", raidenEndpoint+"channels", message, "application/json")
	if status == http.StatusOK {
		err = json.Unmarshal([]byte(body), &jsonr)
		if jsonr["partner_address"] != receiver && err == nil {

		}
	}
	return
}

func closeChannel(receiver string) (err error) {
	var jsonr map[string]interface{}
	message := `{"state": "closed"}`
	status, body, err := raidenlib.SendRequest("PATCH", raidenEndpoint+"channels", message, "application/json")
	if status == http.StatusOK {
		err = json.Unmarshal([]byte(body), &jsonr)
		if err != nil && jsonr["state"] != "closed" && jsonr["partner_address"] == receiver {
			return errors.New("unable to close channel! Please check the Raiden log files")
		}
	}
	return
}

func stopPayments(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		close(quit)
		ticker.Stop()
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("Successfully stopped payments"))
	}
}

func handleChannelRequest(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	address := params["paymentAddress"]
	if address == "" {
		return
	}
	lock.Lock()
	check := channels[address]
	lock.Unlock()
	if check == 0 {
		log.Printf("No Channel with %v found, creating...", address)
		id, err := setupChannel(address, 5000000000)
		if err != nil && err.Error() != `{"errors": "Channel with given partner address already exists"}` {
			fmt.Println(err)
			return
		}
		if id == 0 {
			info, err := getChannelInfo(address)
			if err == nil {
				var jsonr map[string]interface{}
				err = json.Unmarshal([]byte(info), &jsonr)
				log.Println(jsonr)
				if jsonr["partner_address"] == address && err == nil {
					id = int64(jsonr["channel_identifier"].(float64))
				}
			} else {
				log.Println(err)
				return
			}
		}
		log.Printf("Channel with %v created, ID is %v", address, id)
		//Get Lock to prevent Concurrency Issues
		lock.Lock()
		channels[address] = id
		lock.Unlock()

		//check for Ticker and Quit Channel
		createPaymentInterval(2 * time.Second)
		err = sendPayments(address, 1337)
		if err != nil {
			fmt.Println(err)
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"status":"Opened Channel successfully"`))
		return
	}
	err := closeChannel(address)
	if err != nil {
		fmt.Println(err)
	}
}

func handleDebug(w http.ResponseWriter, r *http.Request) {
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
				TokenAddress string
			}{
				tokenAddress,
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
			method := r.FormValue("method")
			endpoint := r.FormValue("endpoint")
			body := r.FormValue("message")

			//Send Request to Satellite for starting payments
			status, body, err := raidenlib.SendRequest(method, endpoint, body, "application/json")
			if err != nil {
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("Request failed"))
				return
			}
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf("Status: %v, Body: %v", status, body)))
		}
	}
}

func createRaidenEndpoint(ethNode string) {
	ethAddress, err := raidenlib.LoadEthereumAddress(keystorePath, password, passwordFile)
	if err != nil {
		log.Println(err)
		ethAddress = raidenlib.CreateEthereumAddress(keystorePath, password, passwordFile)
	}
	log.Printf("Loaded Account: %v successfully", ethAddress)

	u, _ := url.Parse(raidenEndpoint)
	raidenlib.StartRaidenBinary("./raiden-binary", keystorePath, passwordFile, ethAddress, ethNode, u.Host)
	//Wait for Binary to start up
	time.Sleep(20 * time.Second)
}

func createPaymentInterval(interval time.Duration) {
	log.Println("Checking Payment Interval")
	if ticker == nil {
		log.Println("Created Ticker")
		ticker = time.NewTicker(interval)
	}
	if quit == nil {
		log.Println("Created Quit Channel")
		quit = make(chan struct{})
	}
}

func setupWebserver(addr string) {
	router := mux.NewRouter()
	router.HandleFunc("/stop", stopPayments).Methods("GET")
	router.HandleFunc("/payments/{paymentAddress}", handleChannelRequest).Methods("GET")
	router.HandleFunc("/debug", handleDebug).Methods("GET", "POST")
	err := http.ListenAndServe(addr, router)
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	//Create lock for the channel map
	lock = &sync.Mutex{}

	createRaidenEndpoint("http://home.stefan-benten.de:7701")
	log.Println("Starting Webserver")
	setupWebserver("0.0.0.0:7700")
}
