package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/stefanbenten/raiden-on-storj/raidenlib"
)

var (
	accepting             = true
	channels              = map[string]int64{}
	closingchannels       = map[string]*chan struct{}{}
	interval              = 2 * time.Second
	raidenEndpoint        = "http://0.0.0.0:7709/api/v1/"
	tokenAddress          = "0xd762baF19084256262b3f9164a9183009A9001da"
	keystorePath          = "./keystore"
	password              = "superStr0ng"
	passwordFile          = "password.txt"
	deposit         int64 = 5000000000
	payAmount       int64 = 1337
	version               = "v0.100.2"
	lock            *sync.Mutex
)

//checkChannelID does a lookup for the ChannelID to receiver and returns ID or error when not existing
func checkChannelID(receiver string) (id int64, err error) {
	var jsonr map[string]interface{}

	lock.Lock()
	id = channels[receiver]
	lock.Unlock()

	if id == 0 {
		log.Printf("No Channel with %s found in local list, checking API...", receiver)
		//Fetch Channel Information from API
		info, err := getChannelInfo(receiver)
		if err != nil {
			return 0, err
		}
		//Map Information
		err = json.Unmarshal([]byte(info), &jsonr)
		if err != nil {
			log.Println(err)
			return 0, err
		}
		if jsonr["partner_address"] == receiver {
			id = int64(jsonr["channel_identifier"].(float64))
		}
	}
	return
}

//sendPayment sends amount to receiver
func sendPayment(receiver string, amount int64) (err error) {
	statuscode, body, err := raidenlib.SendRequest("POST", raidenEndpoint+path.Join("payments", tokenAddress, receiver), fmt.Sprintf(`{"amount": %v}`, amount), "application/json")
	if err != nil {
		return err
	}
	if statuscode == http.StatusPaymentRequired {
		var jsonr map[string]interface{}
		err = json.Unmarshal([]byte(body), &jsonr)
		if err != nil {
			return err
		}
		//TODO: Test Missing Channel Funds
		log.Println(body)
		//log.Printf("Channel Balance of ID %v insufficient (is: %v, need: %v), refunding..", channels[receiver], jsonr["balance"], amount)
		err = raiseChannelFunds(receiver, deposit)
	}
	return
}

//startPayments starts a go routine which sends every interval the amount to receiver
func startPayments(receiver string, amount int64) (err error) {
	go func() (err error) {
		lock.Lock()
		if closingchannels[receiver] != nil {
			log.Printf("Payments to %s are already going out", receiver)
			return
		}
		quit := make(chan struct{})
		closingchannels[receiver] = &quit
		lock.Unlock()
		ticker := time.NewTicker(interval)
		active := true

		for active {
			select {
			case t := <-ticker.C:
				log.Printf("Sending Payment to %s at: %s", receiver, t.Format("2006-01-02 15:04:05 +0800"))
				err = sendPayment(receiver, amount)
			case <-quit:
				log.Printf("Stopped Payments to %s", receiver)
				ticker.Stop()
				active = false
			}
		}
		return nil
	}()
	return
}

//stopPayments destroys payment go routine
func stopPayments(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if r.FormValue("password") == "stop" {
			accepting = false
			lock.Lock()
			for address, c := range closingchannels {
				if c != nil {
					close(*c)
					closingchannels[address] = nil
					log.Printf("Stopping Payments for: %s", address)
				}
			}
			lock.Unlock()
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":"stopped all payments"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"not authenticated"}`))
	}
}

//getChannelInfo returns the Raiden Channel Information (body) as json encoded string
func getChannelInfo(receiver string) (body string, err error) {
	status, body, err := raidenlib.SendRequest("GET", raidenEndpoint+path.Join("channels", tokenAddress, receiver), "", "application/json")
	if status == http.StatusOK {
		return body, nil
	}
	if err == nil {
		err = errors.New(fmt.Sprintf("Channel Info Query failed, error: %v", err))
	}
	return "", err
}

//setupChannel creates a new channel with receiver and funds it with deposit
func setupChannel(receiver string, deposit int64) (channelID int64, err error) {
	var jsonr map[string]interface{}

	log.Printf("Setting up Channel for %s with balance of %v", receiver, deposit)

	message := fmt.Sprintf(`{
			"partner_address": "%s",
			"token_address": "%s",
			"total_deposit": %v,
			"settle_timeout": 500}`,
		receiver,
		tokenAddress,
		deposit,
	)

	status, body, err := raidenlib.SendRequest("PUT", raidenEndpoint+"channels", message, "application/json")
	if status == http.StatusCreated {
		err = json.Unmarshal([]byte(body), &jsonr)
		if jsonr["partner_address"] == receiver && err == nil {
			log.Printf("Channel setup successfully for %s with balance of %v", receiver, deposit)
			channelID = int64(jsonr["channel_identifier"].(float64))
			return
		}
	}
	if err == nil {
		err = errors.New(fmt.Sprintf("Error with Status %v : %s", status, body))
	}
	return 0, err
}

//raiseChannelFunds increases the current channel deposit with receiver to totalDeposit
func raiseChannelFunds(receiver string, deposit int64) (err error) {
	var jsonr map[string]interface{}

	channelinfo, err := getChannelInfo(receiver)
	err = json.Unmarshal([]byte(channelinfo), &jsonr)
	if err != nil {
		return
	}
	totalDeposit := int64(jsonr["total_deposit"].(float64)) + deposit

	message := fmt.Sprintf(`{"total_deposit": "%v"}`, totalDeposit)
	status, body, err := raidenlib.SendRequest("PATCH", raidenEndpoint+path.Join("channels", tokenAddress, receiver), message, "application/json")
	if status == http.StatusOK {
		err = json.Unmarshal([]byte(body), &jsonr)
		if err != nil {
			return
		}
		if int64(jsonr["total_deposit"].(float64)) == totalDeposit {
			log.Printf("Successfully raised balance by %v in channel with: %s", deposit, receiver)
			log.Printf("- Channel Balance is now: %v", int64(jsonr["balance"].(float64)))
		}
	}
	return
}

//closeChannel is closing the channel with receiver
func closeChannel(receiver string) (err error) {
	var jsonr map[string]interface{}
	message := `{"state": "closed"}`
	status, body, err := raidenlib.SendRequest("PATCH", raidenEndpoint+path.Join("channels", tokenAddress, receiver), message, "application/json")
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		err = json.Unmarshal([]byte(body), &jsonr)
		if err != nil && jsonr["state"] != "closed" && jsonr["partner_address"] == receiver {
			return errors.New("unable to close channel! Please check the Raiden log files")
		}
		return nil
	} else {
		return errors.New("unable to close channel! Please check the Raiden log files - " + strconv.Itoa(status) + body)
	}
}

//handleChannelRequest handles all WebRequests from Clients
func handleChannelRequest(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	address := params["paymentAddress"]
	if address == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"no address provided"}`))
		return
	}
	if !accepting {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"not accepting function calls, please try again later"}`))
		return
	}
	switch r.RequestURI {
	case path.Join("/payments/start", address):

		//Check For Channel existance, else create new one
		id, err := checkChannelID(address)
		if err != nil {
			log.Printf("Creating Channel with %s and deposit %v", address, deposit)
			id, err = setupChannel(address, deposit)
			if err != nil || id == 0 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"Channel ID: %v, error: %v"}`, id, err)))
				return
			}
			log.Printf("Channel with %s created, ID is %v", address, id)
		}

		//Get Lock to prevent Concurrency Issues
		lock.Lock()
		channels[address] = id
		lock.Unlock()

		//Fire up Payments to the new channel
		err = startPayments(address, payAmount)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"Channel ID: %v, error: %v"}`, id, err)))
			return
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"status":"Started Payments successfully"`))

	case path.Join("/payments/stop", address):
		lock.Lock()
		c := closingchannels[address]
		close(*c)
		closingchannels[address] = nil
		lock.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Stopped Payments successfully"}`))

	case path.Join("/payments/close", address):
		err := closeChannel(address)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Closed Channel successfully"}`))
	}
}

//handleDebug gives operator access to Raiden API for direct Calls
func handleDebug(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		{
			t, err := template.New("").Parse(`
			<html>
				<head>
					<title>Debugging</title>
				</head>
				<body>
					<form action="/debug" method="POST">
						Method:<br>
						<input name="method" type="text" size=10 value="GET"><br>
						Endpoint:<br>
						<input name="endpoint" type="text" size=100 value=""><br>
						Message:<br>
						<input name="message" type="text" size=100 value=""><br>
						<hr>
						<input type="submit" value="Send Request" />
					</form>
				</body>
			</html>`,
			)
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
			_, _ = w.Write([]byte(fmt.Sprintf("Status: %v, Body: %s", status, body)))
		}
	}
}

//createRaidenEndpoint starts the Raiden Binary after loading/generating the ETH Address
func createRaidenEndpoint(ethNode string) {
	ethAddress, err := raidenlib.LoadEthereumAddress(keystorePath, passwordFile)
	if err != nil {
		log.Println(err)
		ethAddress = raidenlib.CreateEthereumAddress(keystorePath, password, passwordFile)
	}
	log.Printf("Loaded Account: %s successfully", ethAddress)

	u, _ := url.Parse(raidenEndpoint)
	raidenlib.StartRaidenBinary("./raiden-binary", version, keystorePath, passwordFile, ethAddress, ethNode, u.Host)
}

//setupWebserver configures the webserver with all necessary Handlers and Endpoints
func setupWebserver(addr string) {
	router := mux.NewRouter()
	router.HandleFunc("/stop", stopPayments).Methods("GET")
	router.HandleFunc("/payments/start/{paymentAddress}", handleChannelRequest).Methods("GET")
	router.HandleFunc("/payments/stop/{paymentAddress}", handleChannelRequest).Methods("GET")
	router.HandleFunc("/payments/close/{paymentAddress}", handleChannelRequest).Methods("GET")
	router.HandleFunc("/debug", handleDebug).Methods("GET", "POST")
	err := http.ListenAndServe(addr, router)
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	tm := flag.Int("interval", 2000, "Interval for sending payments (microseconds)")
	pm := flag.Int64("paymentvalue", 1337, "Amount to be sent per each payment")
	ver := flag.String("version", "v0.100.2", "Raiden Binary Version")
	flag.Parse()
	payAmount = *pm
	version = *ver

	interval = time.Duration(*tm) * time.Millisecond
	log.Printf("Setting Payment Interval to %v and payment amount to %v", interval, payAmount)

	//Create lock for the channel map
	lock = &sync.Mutex{}

	createRaidenEndpoint("http://home.stefan-benten.de:7701")
	log.Println("Starting Webserver")
	setupWebserver("0.0.0.0:7700")
}
