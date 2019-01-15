package raidenlib

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

func downloadFile(url string, filepath string) error {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func unzip(file string, dest string) (filenames []string, err error) {

	r, err := zip.OpenReader(file)

	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}
		defer rc.Close()

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {

			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)

		} else {

			// Make File
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return filenames, err
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return filenames, err
			}

			_, err = io.Copy(outFile, rc)

			outFile.Close()

			if err != nil {
				return filenames, err
			}

		}
	}
	return filenames, nil
}

func untar(file string, dest string) (filenames []string, err error) {

	gzipStream, err := os.Open(file)

	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(uncompressedStream)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return filenames, err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(header.Name, 0755); err != nil {
				return filenames, err
			}
		case tar.TypeReg:
			outFile, err := os.Create(header.Name)
			filenames = append(filenames, header.Name)
			if err != nil {
				return filenames, err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return filenames, err
			}
		default:
			return filenames, err
		}
	}
	return filenames, err
}

func FetchRaidenBinary(version string) (err error) {
	var filenames []string

	kernel := ""
	switch runtime.GOOS {
	case "windows":
		//Return as we dont support windows yet..
		return errors.New("unsupported OS")
	case "darwin":
		kernel = "macOS.zip"
	default:
		kernel = "linux.tar.gz"
	}
	//Construct download URI and filename
	raidenbin := fmt.Sprintf("%s-%s-%s", "raiden", version, kernel)
	raidenurl := fmt.Sprintf("https://raiden-nightlies.ams3.digitaloceanspaces.com/%s", raidenbin)
	log.Println("Fetching Binary from: ", raidenurl)
	downloadFile(raidenurl, filepath.Join(os.TempDir(), raidenbin))

	log.Println(filepath.Ext(raidenbin))
	//Extract File depending on the type
	switch filepath.Ext(kernel) {
	case "zip":
		log.Println("Unzipping")
		filenames, err = unzip(filepath.Join(os.TempDir(), raidenbin), "./")
	case "tar.gz":
		log.Println("Untaring")
		filenames, err = untar(filepath.Join(os.TempDir(), raidenbin), "./")
	}
	if err != nil {
		log.Println("Fetched Raiden Binary not successfully")
		return err
	}
	//Rename The Binary
	os.Rename(filepath.Join("./", filenames[0]), "./raiden-binary")

	log.Println("Fetched Raiden Binary successfully")
	return nil
}

func StartRaidenBinary(binarypath string, keystorePath string, passwordFile string, address string, ethEndpoint string, listenAddr string) (pid int) {

	log.Println(binarypath, keystorePath, passwordFile, address, ethEndpoint, listenAddr)
	log.Printf("Starting Raiden Binary for Address: %v and endpoint: %v on listen Address: %v", address, ethEndpoint, listenAddr)

	exists, err := os.Stat(binarypath)
	if err != nil || exists.Name() != "raiden-binary" {
		log.Println("Binary not found, fetching from Repo")
		err = FetchRaidenBinary("v0.19.0")
		if err != nil {
			log.Println(err)
			return
		}
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

/*CreateEthereumAddress is creating an Ethereum Wallet KeyStore file at keystorepath encrypted by password,
saves the password in passwordFileName and returns it's address as string
*/
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

//LoadEthereumAdress is trying to load the Ethereum Address from the KeyStore file in keystorepath
func LoadEthereumAddress(keystorePath string, passwordFileName string) (address string, err error) {

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
	account, err := ks.Import(jsonBytes, string(pass), string(pass))
	if err != nil {
		return "", err
	}
	return account.Address.Hex(), err
}
