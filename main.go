package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/portto/solana-go-sdk/client"
	"github.com/portto/solana-go-sdk/common"
	"github.com/portto/solana-go-sdk/program/system"
	"github.com/portto/solana-go-sdk/rpc"
	"github.com/portto/solana-go-sdk/types"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
	"html"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
)

const username = "admin"

var USDMinimum float64 = 5
var MediaMin float64 = 0.025 // Currently unused
var MessageMaxChar int = 250
var NameMaxChar int = 25
var rpcURL string = "http://127.0.0.1:28088/json_rpc"
var solToUsd = 0.00
var xmrToUsd = 0.00
var addressSliceSolana []AddressSolana

var checked string = ""
var killDono = 30.00 * time.Hour // hours it takes for a dono to be unfulfilled before it is no longer checked.
var indexTemplate *template.Template
var payTemplate *template.Template

var alertTemplate *template.Template
var progressbarTemplate *template.Template
var userOBSTemplate *template.Template
var viewTemplate *template.Template

var loginTemplate *template.Template
var incorrectLoginTemplate *template.Template
var userTemplate *template.Template
var logoutTemplate *template.Template
var incorrectPasswordTemplate *template.Template
var baseCheckingRate = 15

var minSolana, minMonero float64 // Global variables to hold minimum SOL and XMR required to equal the global value
var minDonoValue float64 = 5.0   // The global value to equal in USD terms
var lamportFee = 1000000

// Mainnet
//var c = client.NewClient(rpc.MainnetRPCEndpoint)

// Devnet
var adminSolanaAddress = "9mP1PQXaXWQA44Fgt9PKtPKVvzXUFvrLD2WDLKcj9FVa"
var adminEthereumAddress = "9mP1PQXaXWQA44Fgt9PKtPKVvzXUFvrLD2WDLKcj9FVa"
var adminHexcoinAddress = "9mP1PQXaXWQA44Fgt9PKtPKVvzXUFvrLD2WDLKcj9FVa"
var c = client.NewClient(rpc.DevnetRPCEndpoint)

type priceData struct {
	Monero struct {
		Usd float64 `json:"usd"`
	} `json:"monero"`
	Solana struct {
		Usd float64 `json:"usd"`
	} `json:"solana"`
}

type User struct {
	UserID               int
	Username             string
	HashedPassword       []byte
	EthAddress           string
	SolAddress           string
	HexcoinAddress       string
	XMRWalletPassword    string
	MinDono              int
	MinMediaDono         int
	MediaEnabled         bool
	CreationDatetime     string
	ModificationDatetime string
}

type UserPageData struct {
	ErrorMessage string
}

var db *sql.DB
var userSessions = make(map[string]int)
var amountNeeded = 1000.00
var amountSent = 200.00

func MakeRequest(URL string) string {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", URL, nil)
	req.Header.Set("Header_Key", "Header_Value")
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("Err is", err)
	}
	defer res.Body.Close()

	resBody, _ := ioutil.ReadAll(res.Body)
	response := string(resBody)

	return response
}

type getBalanceResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Context struct {
			Slot uint64 `json:"slot"`
		} `json:"context"`
		Value uint64 `json:"value"`
	} `json:"result"`
	ID int `json:"id"`
}

type superChat struct {
	Name     string
	Message  string
	Media    string
	Amount   string
	Address  string
	QRB64    string
	PayID    string
	CheckURL string
	IsSolana bool
}

type indexDisplay struct {
	MaxChar   int
	MinSolana float64
	MinMonero float64
	SolPrice  float64
	XMRPrice  float64
	MinAmnt   float64
	Checked   string
}

type alertPageData struct {
	Name          string
	Message       string
	Amount        float64
	Currency      string
	Refresh       int
	DisplayToggle string
}

type progressbarData struct {
	Message string
	Needed  float64
	Sent    float64
	Refresh int
}

type obsDataStruct struct {
	FilenameGIF string
	FilenameMP3 string
	URLdisplay  string
	URLdonobar  string
}

type rpcResponse struct {
	ID      string `json:"id"`
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		IntegratedAddress string `json:"integrated_address"`
		PaymentID         string `json:"payment_id"`
	} `json:"result"`
}

type AddressSolana struct {
	KeyPublic  string
	KeyPrivate ed25519.PrivateKey
	DonoName   string
	DonoString string
	DonoAmount float64
	DonoAnon   bool
}

type MoneroPrice struct {
	Monero struct {
		Usd float64 `json:"usd"`
	} `json:"monero"`
}

var a alertPageData
var pb progressbarData
var obsData obsDataStruct
var pbMessage = "Stream Tomorrow"

func setMinDonos() {

	// Calculate how much Monero is needed to equal the min usd donation var.
	minMonero = minDonoValue / xmrToUsd
	// Calculate how much Solana is needed to equal the min usd donation var.
	minSolana = minDonoValue / solToUsd
	minMonero, _ = strconv.ParseFloat(fmt.Sprintf("%.5f", minMonero), 64)
	minSolana, _ = strconv.ParseFloat(fmt.Sprintf("%.5f", minSolana), 64)

	log.Println("Minimum XMR Dono:", minMonero)
	log.Println("Minimum SOL Dono:", minSolana)
}

func fetchExchangeRates() {
	for {
		// Fetch the exchange rate data from the API
		resp, err := http.Get("https://api.coingecko.com/api/v3/simple/price?ids=monero,solana&vs_currencies=usd")
		if err != nil {
			fmt.Println("Error fetching price data:", err)
			// Wait five minutes before trying again
			time.Sleep(300 * time.Second)
			continue
		}
		defer resp.Body.Close()

		// Parse the JSON response
		var data priceData
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			fmt.Println("Error decoding price data:", err)
			// Wait five minutes before trying again
			time.Sleep(300 * time.Second)
			continue
		}

		// Update the exchange rate values
		xmrToUsd = data.Monero.Usd
		solToUsd = data.Solana.Usd

		fmt.Println("Updated exchange rates:", "1 XMR = ", xmrToUsd, " USD,", "1 SOL = ", solToUsd, " USD")

		// Calculate how much Monero is needed to equal the min usd donation var.
		minMonero = minDonoValue / data.Monero.Usd
		// Calculate how much Solana is needed to equal the min usd donation var.
		minSolana = minDonoValue / data.Solana.Usd

		minMonero, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", minMonero), 64)
		minSolana, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", minSolana), 64)

		// Save the minimum Monero and Solana variables
		fmt.Println("Min monero:", minMonero)

		fmt.Println("Min solana:", minSolana)
		// Wait three minutes before fetching again
		if xmrToUsd == 0 || solToUsd == 0 {
			time.Sleep(180 * time.Second)
		} else {
			time.Sleep(30 * time.Second)
		}

	}
}

func startMoneroWallet() {
	cmd := exec.Command("monero/monero-wallet-rpc.exe", "--rpc-bind-port", "28088", "--daemon-address", "https://xmr-node.cakewallet.com:18081", "--wallet-file", "monero/wallet", "--disable-rpc-login", "--password", "")

	// Capture the output of the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running command: %v\n", err)
		return
	}

	// Print the output of the command
	fmt.Println(string(output))
}

func main() {

	go startMoneroWallet()

	log.Println("Starting server")

	var err error

	// Open a new database connection
	db, err = sql.Open("sqlite3", "users.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check if the database and tables exist, and create them if they don't
	err = createDatabaseIfNotExists(db)
	if err != nil {
		panic(err)
	}

	// create a RPC client for Solana
	fmt.Println(reflect.TypeOf(c))

	// get the current running Solana version
	response, err := c.GetVersion(context.TODO())
	if err != nil {
		panic(err)
	}

	fmt.Println("version", response.SolanaCore)

	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/style.css")
	})
	http.HandleFunc("/xmr.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/xmr.svg")
	})

	http.HandleFunc("/sol.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/sol.svg")
	})

	// Schedule a function to run fetchExchangeRates every three minutes
	go fetchExchangeRates()
	go checkDonos()

	a.Refresh = 10
	pb.Refresh = 1

	obsData.URLdonobar = "/progressbar"
	obsData.URLdisplay = "/alert"

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/pay", paymentHandler)
	http.HandleFunc("/alert", alertOBSHandler)

	http.HandleFunc("/progressbar", progressbarOBSHandler)

	// serve login and user interface pages
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/incorrect_login", incorrectLoginHandler)
	http.HandleFunc("/user", userHandler)
	http.HandleFunc("/userobs", userOBSHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/changepassword", changePasswordHandler)
	http.HandleFunc("/changeuser", changeUserHandler)
	getObsData(db, 1)

	indexTemplate, _ = template.ParseFiles("web/index.html")
	payTemplate, _ = template.ParseFiles("web/pay.html")
	alertTemplate, _ = template.ParseFiles("web/alert.html")

	userOBSTemplate, _ = template.ParseFiles("web/obs/settings.html")
	progressbarTemplate, _ = template.ParseFiles("web/obs/progressbar.html")

	loginTemplate, _ = template.ParseFiles("web/login.html")
	incorrectLoginTemplate, _ = template.ParseFiles("web/incorrect_login.html")
	userTemplate, _ = template.ParseFiles("web/user.html")
	logoutTemplate, _ = template.ParseFiles("web/logout.html")
	incorrectPasswordTemplate, _ = template.ParseFiles("web/password_change_failed.html")

	user, err := getUserByUsername(username)
	if err != nil {
		panic(err)
	}

	minDonoValue = float64(user.MinDono)
	adminSolanaAddress = user.SolAddress
	setMinDonos()

	err = http.ListenAndServe(":8900", nil)
	if err != nil {
		panic(err)
	}

}

func checkDonos() {
	for {
		fulfilledDonos := checkUnfulfilledDonos()
		fmt.Println("Fulfilled Donos:")
		for _, dono := range fulfilledDonos {
			fmt.Println(dono)
			err := createNewQueueEntry(db, dono.Address, dono.Name, dono.Message, dono.AmountSent, dono.CurrencyType)
			if err != nil {
				panic(err)
			}
			addDonoToDonoBar(dono.AmountSent, dono.CurrencyType)
		}
		time.Sleep(time.Duration(baseCheckingRate) * time.Second)
	}
}

func addDonoToDonoBar(as float64, c string) {
	usdVal := 0.00

	if c == "XMR" {
		usdVal = as * xmrToUsd
	} else if c == "SOL" {
		usdVal = as * solToUsd
	}
	pb.Sent += usdVal
	amountSent = pb.Sent

	err := updateObsData(db, 1, 1, obsData.FilenameGIF, obsData.FilenameMP3, "alice", pb)

	if err != nil {
		log.Println("Error: ", err)
		return
	}
}

func createNewQueueEntry(db *sql.DB, address string, name string, message string, amount float64, currency string) error {
	_, err := db.Exec(`
		INSERT INTO queue (name, message, amount, currency) VALUES (?, ?, ?, ?)
	`, name, message, amount, currency)
	if err != nil {
		return err
	}
	return nil
}

func createNewDono(user_id int, dono_address string, dono_name string, dono_message string, amount_to_send float64, currencyType string, encrypted_ip string, anon_dono bool) {
	// Open a new database connection
	db, err := sql.Open("sqlite3", "users.db")
	if err != nil {

		panic(err)
	}
	defer db.Close()

	// Get current time
	createdAt := time.Now().UTC()

	// Execute the SQL INSERT statement
	_, err = db.Exec(`
		INSERT INTO donos (
			user_id,
			dono_address,
			dono_name,
			dono_message,
			amount_to_send,
			amount_sent,
			currency_type,
			anon_dono,
			fulfilled,
			encrypted_ip,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user_id, dono_address, dono_name, dono_message, amount_to_send, 0.0, currencyType, anon_dono, false, encrypted_ip, createdAt, createdAt)
	if err != nil {
		log.Println(err)
		panic(err)
	}
}

type Dono struct {
	ID           int
	UserID       int
	Address      string
	Name         string
	Message      string
	AmountToSend float64
	AmountSent   float64
	CurrencyType string
	AnonDono     bool
	Fulfilled    bool
	EncryptedIP  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func clearEncryptedIP(dono *Dono) {
	dono.EncryptedIP = ""
}

func encryptIP(ip string) string {
	h := sha256.New()
	h.Write([]byte("IPFingerprint" + ip))
	hash := h.Sum(nil)
	return hex.EncodeToString(hash)
}

func getUnfulfilledDonoIPs() ([]string, error) {
	ips := []string{}

	db, err := sql.Open("sqlite3", "users.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT ip FROM donos WHERE fulfilled = false`)
	if err != nil {
		return ips, err
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		err := rows.Scan(&ip)
		if err != nil {
			return ips, err
		}
		ips = append(ips, ip)
	}

	err = rows.Err()
	if err != nil {
		return ips, err
	}

	return ips, nil
}

func checkUnfulfilledDonos() []Dono {
	ips, _ := getUnfulfilledDonoIPs() // get ips

	// Open a new database connection
	db, err := sql.Open("sqlite3", "users.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Retrieve all unfulfilled donos from the database
	rows, err := db.Query(`SELECT * FROM donos WHERE fulfilled = false`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var fulfilledSlice []bool
	var amountSlice []float64
	var fulfilledDonos []Dono
	var rowsToUpdate []int // slice to store row ids to be updated
	var dono Dono
	for rows.Next() { // Loop through the unfulfilled donos and check their status
		err := rows.Scan(&dono.ID, &dono.UserID, &dono.Address, &dono.Name, &dono.Message, &dono.AmountToSend, &dono.AmountSent, &dono.CurrencyType, &dono.AnonDono, &dono.Fulfilled, &dono.EncryptedIP, &dono.CreatedAt, &dono.UpdatedAt)
		if err != nil {
			panic(err)
		}

		log.Println(dono.ID)

		log.Println("Dono ID: ", dono.ID, "Address: ", dono.Address)
		log.Println("Name: ", dono.Name)
		log.Println("Message: ", dono.Message)
		log.Println("AmountToSend: ", dono.AmountToSend)
		log.Println("AmountSent(old): ", dono.AmountSent)
		log.Println("CurrencyType: ", dono.CurrencyType)
		// Check if the dono has exceeded the killDono time
		timeElapsedFromDonoCreation := time.Since(dono.CreatedAt)
		if timeElapsedFromDonoCreation > (killDono) {
			dono.Fulfilled = true
			rowsToUpdate = append(rowsToUpdate, dono.ID)
			// add true to fulfilledSlice
			fulfilledSlice = append(fulfilledSlice, true)

			amountSlice = append(amountSlice, dono.AmountSent)
			log.Println("Dono too old, killed (marked as fulfilled) and won't be checked again. \n")
			continue
		}

		// Check if the dono needs to be skipped based on exponential backoff
		secondsElapsedSinceLastCheck := time.Since(dono.UpdatedAt).Seconds()
		log.Println("Seconds since last check: ", secondsElapsedSinceLastCheck)
		expoAdder := returnIPPenalty(ips, dono.EncryptedIP) + time.Since(dono.CreatedAt).Seconds()/60/60/19
		secondsNeededToCheck := math.Pow(float64(baseCheckingRate)-0.02, expoAdder)
		log.Println("Seconds needed to check: ", secondsNeededToCheck)

		if secondsElapsedSinceLastCheck < secondsNeededToCheck {
			log.Println("Not enough time has passed, skipping. \n")
			continue // If not enough time has passed then ignore
		}
		log.Println("Enough time has passed, checking.")

		if dono.CurrencyType == "XMR" {
			dono.AmountSent, _ = getXMRBalance(dono.Address)
		} else if dono.CurrencyType == "SOL" {
			dono.AmountSent, _ = getSOLBalance(dono.Address)
		}

		log.Println("AmountSent(new): ", dono.AmountSent, "\n")

		if dono.AmountSent >= dono.AmountToSend-float64(lamportFee)/1e9 {
			wallet, _ := ReadAddress(dono.Address)

			SendSolana(wallet.KeyPublic, wallet.KeyPrivate, adminSolanaAddress, dono.AmountSent)

			dono.Fulfilled = true
			// add true to fulfilledSlice
			fulfilledDonos = append(fulfilledDonos, dono)
			rowsToUpdate = append(rowsToUpdate, dono.ID)
			fulfilledSlice = append(fulfilledSlice, true)
			amountSlice = append(amountSlice, dono.AmountSent)
			log.Println("Dono FULFILLED and sent to home sol address and won't be checked again. \n")
			continue
		}

		// add to slices
		fulfilledSlice = append(fulfilledSlice, false)
		rowsToUpdate = append(rowsToUpdate, dono.ID)
		amountSlice = append(amountSlice, dono.AmountSent)
	}

	i := 0
	// Update rows to be update in a way that never throws a database locked error
	for _, rowID := range rowsToUpdate {
		_, err = db.Exec(`UPDATE donos SET updated_at = ?, fulfilled = ?, amount_sent = ? WHERE dono_id = ?`, time.Now(), fulfilledSlice[i], amountSlice[i], rowID)
		if err != nil {
			panic(err)
		}
		i += 1
	}

	return fulfilledDonos
}

func getSOLBalance(address string) (float64, error) {
	balance, err := c.GetBalance(
		context.TODO(), // request context
		address,        // wallet to fetch balance for
	)
	if err != nil {
		return 0, err
	}
	log.Println("Address: ", address, " Balance: ", float64(balance)/1e9, "\n")
	return float64(balance) / 1e9, nil
}

func getXMRBalance(address string) (float64, error) {
	url := "http://127.0.0.1:18081/json_rpc"

	// Create the JSON request payload for RPC call and convert to JSON
	rpcRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "0",
		"method":  "get_balance",
		"params": map[string]interface{}{
			"address": address,
		},
	}
	payload, err := json.Marshal(rpcRequest)
	if err != nil {
		return 0, err
	}

	// POST request XMR RPC endpoint
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Parse JSON response
	var rpcResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&rpcResponse)
	if err != nil {
		return 0, err
	}

	// Check RPC call was successful
	if rpcResponse["error"] != nil {
		return 0, fmt.Errorf("RPC call failed: %s", rpcResponse["error"].(map[string]interface{})["message"])
	}

	// Extract balance from response and convert to float64
	balance, err := strconv.ParseFloat(rpcResponse["result"].(map[string]interface{})["balance"].(string), 64)
	if err != nil {
		return 0, err
	}
	return balance / 1000000000000, nil // convert from atomic units to XMR
}

func processQueue(db *sql.DB) error {
	// Retrieve oldest entry from queue table
	row := db.QueryRow(`
		SELECT id, name, amount, currency FROM queue
		ORDER BY created_at ASC LIMIT 1
	`)

	var id int
	var name string
	var amount float64
	var currency string
	err := row.Scan(&id, &name, &amount, &currency)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	// Check if we can display a new dono
	if displayNewDono(name, amount, currency) {
		_, err = db.Exec(`
			DELETE FROM queue WHERE id = ?
		`, id)
		if err != nil {
			return err
		}
	}

	return nil
}

// CreateAddress inserts a new address into the database.
func CreateAddress(addr AddressSolana) error {
	// Convert the private key to a byte slice.
	privateKeyBytes := []byte(addr.KeyPrivate)

	// Insert the address into the database.
	_, err := db.Exec("INSERT INTO addresses (key_public, key_private) VALUES (?, ?)",
		addr.KeyPublic, privateKeyBytes)
	return err
}

// ReadAddress reads an address from the database by public key.
func ReadAddress(publicKey string) (*AddressSolana, error) {
	// Query the database for the address.
	row := db.QueryRow("SELECT key_public, key_private FROM addresses WHERE key_public = ?", publicKey)

	var keyPublic string
	var privateKeyBytes []byte
	err := row.Scan(&keyPublic, &privateKeyBytes)
	if err != nil {
		return nil, err
	}

	// Convert the private key byte slice to an ed25519.PrivateKey.
	privateKey := ed25519.PrivateKey(privateKeyBytes)

	// Create a new AddressSolana object.
	addr := AddressSolana{
		KeyPublic:  keyPublic,
		KeyPrivate: privateKey,
	}

	return &addr, nil
}

// UpdateAddress updates an existing address in the database.
func UpdateAddress(addr AddressSolana) error {
	// Convert the private key to a byte slice.
	privateKeyBytes := []byte(addr.KeyPrivate)

	// Update the address in the database.
	_, err := db.Exec("UPDATE addresses SET key_private = ? WHERE key_public = ?",
		privateKeyBytes, addr.KeyPublic)
	return err
}

// DeleteAddress deletes an address from the database by public key.
func DeleteAddress(publicKey string) error {
	_, err := db.Exec("DELETE FROM addresses WHERE key_public = ?", publicKey)
	return err
}

func displayNewDono(name string, amount float64, currency string) bool {
	return false
}

func createDatabaseIfNotExists(db *sql.DB) error {
	// create the tables if they don't exist
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS donos (
            dono_id INTEGER PRIMARY KEY,
            user_id INTEGER,
            dono_address TEXT,
            dono_name TEXT,
            dono_message TEXT,
            amount_to_send FLOAT,            
            amount_sent FLOAT,
            currency_type TEXT,
            anon_dono BOOL,
            fulfilled BOOL,
            encrypted_ip TEXT,
            created_at DATETIME,
            updated_at DATETIME,
            FOREIGN KEY(user_id) REFERENCES users(id)
        )
    `)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS addresses (
            key_public TEXT NOT NULL,
            key_private BLOB NOT NULL
        )
    `)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS queue (
            name TEXT,
            message TEXT,
            amount FLOAT,
            currency TEXT
        )
    `)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT UNIQUE,
            HashedPassword BLOB,
            eth_address TEXT,
            sol_address TEXT,
            hex_address TEXT,
            xmr_wallet_password TEXT,
            min_donation_threshold FLOAT,
            min_media_threshold FLOAT,
            media_enabled BOOL,
            created_at DATETIME,
            modified_at DATETIME
        )
    `)

	if err != nil {
		return err
	}

	err = createObsTable(db)
	if err != nil {
		log.Fatal(err)
	}

	emptyTable, err := checkObsData(db)
	if err != nil {
		log.Fatal(err)
	}

	if emptyTable {
		pbData := progressbarData{
			Message: "test message",
			Needed:  100.0,
			Sent:    50.0,
			Refresh: 5,
		}
		err = insertObsData(db, 1, "test.gif", "test.mp3", "test_voice", pbData)
		if err != nil {
			log.Fatal(err)
		}
	}

	// create admin user if not exists
	adminUser := User{
		Username:          "admin",
		EthAddress:        "asl12312qse123we1232323lol",
		SolAddress:        "solololololololololsbfjeew",
		HexcoinAddress:    "realmoneyrealmoney123BMIhi",
		XMRWalletPassword: "",
		MinDono:           3,
		MinMediaDono:      5,
		MediaEnabled:      true,
	}

	adminHashedPassword, err := bcrypt.GenerateFromPassword([]byte("hunter123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	adminUser.HashedPassword = adminHashedPassword

	err = createUser(adminUser)
	if err != nil {
		log.Println(err)
	}

	return nil
}

func createObsTable(db *sql.DB) error {
	obsTable := `
        CREATE TABLE IF NOT EXISTS obs (
            id INTEGER PRIMARY KEY,
            user_id INTEGER,
            gif_name TEXT,
            mp3_name TEXT,
            tts_voice TEXT,
            message TEXT,
            needed FLOAT,
            sent FLOAT
        );`
	_, err := db.Exec(obsTable)
	return err
}

func insertObsData(db *sql.DB, userId int, gifName, mp3Name, ttsVoice string, pbData progressbarData) error {
	obsData := `
        INSERT INTO obs (
            user_id,
            gif_name,
            mp3_name,
            tts_voice,
            message,
            needed,
            sent
        ) VALUES (?, ?, ?, ?, ?, ?, ?);`
	_, err := db.Exec(obsData, userId, gifName, mp3Name, ttsVoice, pbData.Message, pbData.Needed, pbData.Sent)
	return err
}

func checkObsData(db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM obs").Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func updateObsData(db *sql.DB, obsId int, userId int, gifName string, mp3Name string, ttsVoice string, pbData progressbarData) error {
	updateObsData := `
        UPDATE obs
        SET user_id = ?,
            gif_name = ?,
            mp3_name = ?,
            tts_voice = ?,
            message = ?,
            needed = ?,
            sent = ?
        WHERE id = ?;`
	_, err := db.Exec(updateObsData, userId, gifName, mp3Name, ttsVoice, pbData.Message, pbData.Needed, pbData.Sent, obsId)
	return err
}

func getObsData(db *sql.DB, userId int) {
	err := db.QueryRow("SELECT gif_name, mp3_name, `message`, needed, sent FROM obs WHERE user_id = ?", userId).
		Scan(&obsData.FilenameGIF, &obsData.FilenameMP3, &pbMessage, &amountNeeded, &amountSent)
	if err != nil {
		log.Println("Error:", err)
	}

	log.Println(pbMessage)
	log.Println(amountNeeded)
	log.Println(amountSent)
}

func createUser(user User) error {

	// Insert the user's data into the database
	_, err := db.Exec(`
        INSERT INTO users (
            username,
            HashedPassword,
            eth_address,
            sol_address,
            hex_address,
            xmr_wallet_password,
            min_donation_threshold,
            min_media_threshold,
            media_enabled,
            created_at,
            modified_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, user.Username, user.HashedPassword, user.EthAddress, user.SolAddress, user.HexcoinAddress, "", user.MinDono, user.MinMediaDono, user.MediaEnabled, time.Now(), time.Now())

	adminEthereumAddress = user.EthAddress
	adminSolanaAddress = user.SolAddress
	adminHexcoinAddress = user.HexcoinAddress
	minDonoValue = float64(user.MinDono)

	return err
}

// update an existing user
func updateUser(user User) error {
	statement := `
		UPDATE users
		SET Username=?, HashedPassword=?, eth_address=?, sol_address=?, hex_address=?,
			xmr_wallet_password=?, min_donation_threshold=?, min_media_threshold=?, media_enabled=?, modified_at=datetime('now')
		WHERE id=?
	`
	_, err := db.Exec(statement, user.Username, user.HashedPassword, user.EthAddress,
		user.SolAddress, user.HexcoinAddress, user.XMRWalletPassword, user.MinDono, user.MinMediaDono,
		user.MediaEnabled, user.UserID)
	if err != nil {
		log.Fatalf("failed, err: %v", err)
	}
	return err
}

// get a user by their username
func getUserByUsername(username string) (User, error) {
	var user User
	row := db.QueryRow("SELECT * FROM users WHERE Username=?", username)
	err := row.Scan(&user.UserID, &user.Username, &user.HashedPassword, &user.EthAddress,
		&user.SolAddress, &user.HexcoinAddress, &user.XMRWalletPassword, &user.MinDono, &user.MinMediaDono,
		&user.MediaEnabled, &user.CreationDatetime, &user.ModificationDatetime)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// get a user by their session token
func getUserBySession(sessionToken string) (User, error) {
	userID, ok := userSessions[sessionToken]
	if !ok {
		return User{}, fmt.Errorf("session token not found")
	}
	var user User
	row := db.QueryRow("SELECT * FROM users WHERE id=?", userID)
	err := row.Scan(&user.UserID, &user.Username, &user.HashedPassword, &user.EthAddress,
		&user.SolAddress, &user.HexcoinAddress, &user.XMRWalletPassword, &user.MinDono, &user.MinMediaDono,
		&user.MediaEnabled, &user.CreationDatetime, &user.ModificationDatetime)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// verify that the entered password matches the stored hashed password for a user
func verifyPassword(user User, password string) bool {
	err := bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password))
	return err == nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := getUserByUsername(username)

		if err != nil {
			if err.Error() == "sql: no rows in result set" { // can't find username in DB
				http.Redirect(w, r, "/incorrect_login", http.StatusFound)
				return
			}

			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if user.UserID == 0 || !verifyPassword(user, password) {
			http.Redirect(w, r, "/incorrect_login", http.StatusFound)
			return
		}

		sessionToken, err := createSession(user.UserID)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
			Secure:   true,
		})
		http.Redirect(w, r, "/user", http.StatusFound)
		return
	}
	tmpl := template.Must(template.ParseFiles("web/login.html"))
	err := tmpl.Execute(w, nil)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func userOBSHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := getUserBySession(cookie.Value)
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	log.Println(user)
	log.Println(cookie)

	host := r.Host // get host url
	obsData.URLdonobar = host + "/progressbar"
	obsData.URLdisplay = host + "/alert"

	if r.Method == http.MethodPost {
		r.ParseMultipartForm(10 << 20) // max file size of 10 MB

		// Get the files from the request
		fileGIF, handlerGIF, err := r.FormFile("dono_animation")
		if err == nil {
			defer fileGIF.Close()

			// Save the file to the server
			fileNameGIF := handlerGIF.Filename
			fileBytesGIF, err := ioutil.ReadAll(fileGIF)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err = os.WriteFile("web/obs/media/"+fileNameGIF, fileBytesGIF, 0644); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			obsData.FilenameGIF = fileNameGIF
		}

		fileMP3, handlerMP3, err := r.FormFile("dono_sound")
		if err == nil {
			defer fileMP3.Close()

			fileNameMP3 := handlerMP3.Filename
			fileBytesMP3, err := ioutil.ReadAll(fileMP3)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err = os.WriteFile("web/obs/media/"+fileNameMP3, fileBytesMP3, 0644); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			obsData.FilenameMP3 = fileNameMP3
		}

		pbMessage = r.FormValue("message")
		amountNeededStr := r.FormValue("needed")
		amountSentStr := r.FormValue("sent")

		amountNeeded, err = strconv.ParseFloat(amountNeededStr, 64)
		if err != nil {
			// handle the error
			log.Println(err)
		}

		amountSent, err = strconv.ParseFloat(amountSentStr, 64)
		if err != nil {
			// handle the error
			log.Println(err)
		}
		pb.Message = pbMessage
		pb.Needed = amountNeeded
		pb.Sent = amountSent

		err = updateObsData(db, 1, 1, obsData.FilenameGIF, obsData.FilenameMP3, "alice", pb)

		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {

		getObsData(db, 1)
	}

	tmpl, err := template.ParseFiles("web/obs/settings.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type combinedData struct {
		obsDataStruct
		progressbarData
	}

	tnd := combinedData{obsData, pb}

	tmpl.Execute(w, tnd)

}

// handle requests to modify user data
func userHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := getUserBySession(cookie.Value)
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		user.Username = r.FormValue("username")
		user.EthAddress = r.FormValue("ethaddress")
		user.SolAddress = r.FormValue("soladdress")
		user.HexcoinAddress = r.FormValue("hexcoinaddress")
		user.XMRWalletPassword = r.FormValue("xmrwalletpassword")
		minDono, _ := strconv.Atoi(r.FormValue("mindono"))
		user.MinDono = minDono
		minMediaDono, _ := strconv.Atoi(r.FormValue("minmediadono"))
		user.MinMediaDono = minMediaDono
		mediaEnabled := r.FormValue("mediaenabled") == "on"
		user.MediaEnabled = mediaEnabled

		err := updateUser(user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/user", http.StatusSeeOther)
		return
	}

	tmpl, err := template.ParseFiles("web/user.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setMinDonos()
	tmpl.Execute(w, user)

}

func generateSessionToken(user User) string {
	// generate a random session token
	b := make([]byte, 32)
	rand.Read(b)
	sessionToken := base64.URLEncoding.EncodeToString(b)
	// save the session token in a map
	userSessions[sessionToken] = user.UserID
	return sessionToken
}

func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	// retrieve user from session
	sessionToken, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	user, err := getUserBySession(sessionToken.Value)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	// initialize user page data struct
	data := UserPageData{}

	// process form submission
	if r.Method == "POST" {
		// check current password
		if !verifyPassword(user, r.FormValue("current_password")) {
			// set user page data values
			data.ErrorMessage = "Current password entered was incorrect"
			// render password change failed form
			tmpl, err := template.ParseFiles("web/password_change_failed.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, data)
			return
		} else {
			// hash new password
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(r.FormValue("new_password")), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// update user password in database
			user.HashedPassword = hashedPassword
			err = updateUser(user)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// redirect to user page
			http.Redirect(w, r, "/user", http.StatusSeeOther)
			return
		}
	}

	// render change password form
	tmpl, err := template.ParseFiles("web/user.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func changeUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting change user handler function")
	// retrieve user from session
	sessionToken, err := r.Cookie("session_token")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	user, err := getUserBySession(sessionToken.Value)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// initialize user page data struct
	data := UserPageData{}

	// process form submission
	if r.Method == "POST" {
		user.EthAddress = r.FormValue("ethereumAddress")
		adminEthereumAddress = user.EthAddress
		user.SolAddress = r.FormValue("solanaAddress")
		adminSolanaAddress = user.SolAddress
		user.HexcoinAddress = r.FormValue("hexcoinAddress")
		adminHexcoinAddress = user.HexcoinAddress
		minDono, _ := strconv.Atoi(r.FormValue("minUsdAmount"))
		user.MinDono = minDono
		minDonoValue = float64(minDono)
		log.Println("Begin write to user")
		err = updateUser(user)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("wrote to user")
		// redirect to user page
		http.Redirect(w, r, "/user", http.StatusSeeOther)
		log.Println("redirect to user")
		return
	}

	// render change password form
	tmpl, err := template.ParseFiles("web/user.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func renderChangePasswordForm(w http.ResponseWriter, data UserPageData) {
	tmpl, err := template.ParseFiles("web/user.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// invalidate session token and redirect user to home page
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func incorrectLoginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("web/incorrect_login.html"))
	err := tmpl.Execute(w, nil)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func createSession(userID int) (string, error) {
	sessionToken := uuid.New().String()
	userSessions[sessionToken] = userID
	return sessionToken, nil
}

func validateSession(r *http.Request) (int, error) {
	sessionToken, err := r.Cookie("session_token")
	if err != nil {
		return 0, fmt.Errorf("no session token found")
	}
	userID, ok := userSessions[sessionToken.Value]
	if !ok {
		return 0, fmt.Errorf("invalid session token")
	}
	return userID, nil
}

func createWalletSolana(dName string, dString string, dAmount float64, dAnon bool) AddressSolana {
	wallet := types.NewAccount()

	address := AddressSolana{}
	address.KeyPublic = wallet.PublicKey.ToBase58()
	address.KeyPrivate = wallet.PrivateKey
	address.DonoName = dName
	address.DonoAmount = dAmount
	address.DonoString = dString
	address.DonoAnon = dAnon
	addToAddressSliceSolana(address)
	CreateAddress(address)

	return address
}

func addToAddressSliceSolana(a AddressSolana) {
	addressSliceSolana = append(addressSliceSolana, a)
	fmt.Println(len(addressSliceSolana))
}

func condenseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateStrings(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for !utf8.ValidString(s[:n]) {
		n--
	}
	return s[:n]
}

func reverse(ss []string) {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
}

func alertOBSHandler(w http.ResponseWriter, r *http.Request) {
	newDono, err := checkDonoQueue(db)
	if err != nil {
		log.Printf("Error checking donation queue: %v\n", err)
	}

	if newDono {
		fmt.Println("Showing NEW DONO!")
	} else {
		a.DisplayToggle = "display: none;"
	}
	err = alertTemplate.Execute(w, a)
	if err != nil {
		fmt.Println(err)
	}
}

func progressbarOBSHandler(w http.ResponseWriter, r *http.Request) {

	pb.Message = pbMessage
	pb.Needed = amountNeeded
	pb.Sent = amountSent

	err := progressbarTemplate.Execute(w, pb)
	if err != nil {
		fmt.Println(err)
	}

}

func getCurrentDateTime() string {
	now := time.Now()
	return now.Format("2006-01-02 15:04:05")
}

func indexHandler(w http.ResponseWriter, _ *http.Request) {
	var i indexDisplay
	i.MaxChar = MessageMaxChar
	i.MinSolana = minSolana
	i.MinMonero = minMonero
	i.SolPrice = solToUsd
	i.XMRPrice = xmrToUsd
	i.Checked = checked
	err := indexTemplate.Execute(w, i)
	if err != nil {
		fmt.Println(err)
	}
}

func checkDonoQueue(db *sql.DB) (bool, error) {

	// Fetch oldest entry from queue table
	row := db.QueryRow("SELECT name, message, amount, currency FROM queue ORDER BY rowid LIMIT 1")

	var name string
	var message string
	var amount float64
	var currency string
	err := row.Scan(&name, &message, &amount, &currency)
	if err == sql.ErrNoRows {
		// Queue is empty, do nothing
		return false, nil
	} else if err != nil {
		// Error occurred while fetching row
		return false, err
	}

	fmt.Println("Showing notif:", name, ":", message)
	// update the form in memory
	a.Name = name
	a.Message = message
	a.Amount = amount
	a.Currency = currency
	a.DisplayToggle = "display: block;"

	// Remove fetched entry from queue table
	_, err = db.Exec("DELETE FROM queue WHERE name = ? AND message = ? AND amount = ? AND currency = ?", name, message, amount, currency)
	if err != nil {
		return false, err
	}

	return true, nil
}

func returnIPPenalty(ips []string, currentDonoIP string) float64 {
	// Check if the encrypted IP matches any of the encrypted IPs in the slice of donos
	sameIPCount := 0
	for _, donoIP := range ips {
		if donoIP == currentDonoIP {
			sameIPCount++
		}
	}
	// Calculate the exponential delay factor based on the number of matching IPs
	expoAdder := 1.00
	if sameIPCount > 2 {
		expoAdder = math.Pow(1.3, float64(sameIPCount)) / 1.3
	}
	return expoAdder
}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user's IP address
	ip := r.RemoteAddr

	// Get form values
	fMon := r.FormValue("mon")
	fAmount := r.FormValue("amount")
	fName := r.FormValue("name")
	fMessage := r.FormValue("message")
	fMedia := r.FormValue("media")
	fShowAmount := r.FormValue("showAmount")
	encrypted_ip := encryptIP(ip)

	// Parse and handle errors for each form value
	mon, _ := strconv.ParseBool(fMon)
	amount, err := strconv.ParseFloat(fAmount, 64)
	if err != nil {
		if mon {
			amount = minMonero
		} else {
			amount = minSolana
		}
	}

	name := fName
	if name == "" {
		name = "Anonymous"
	}

	message := fMessage
	if message == "" {
		message = " "
	}

	media := html.EscapeString(fMedia)

	showAmount, _ := strconv.ParseBool(fShowAmount)

	var s superChat
	params := url.Values{}

	params.Add("name", name)
	params.Add("msg", message)
	params.Add("media", condenseSpaces(media))
	params.Add("amount", strconv.FormatFloat(amount, 'f', 4, 64))
	params.Add("show", strconv.FormatBool(showAmount))

	s.Amount = strconv.FormatFloat(amount, 'f', 4, 64)
	s.Name = html.EscapeString(truncateStrings(condenseSpaces(name), NameMaxChar))
	s.Message = html.EscapeString(truncateStrings(condenseSpaces(message), MessageMaxChar))
	s.Media = html.EscapeString(media)

	if mon {
		handleMoneroPayment(w, &s, params)
		createNewDono(1, s.Address, s.Name, s.Message, amount, "XMR", encrypted_ip, showAmount)
	} else {
		walletAddress := handleSolanaPayment(w, &s, params, name, message, amount, showAmount, media, mon)
		createNewDono(1, walletAddress, s.Name, s.Message, amount, "SOL", encrypted_ip, showAmount)
	}
}

func handleSolanaPayment(w http.ResponseWriter, s *superChat, params url.Values, name_ string, message_ string, amount_ float64, showAmount_ bool, media_ string, mon_ bool) string {
	var wallet_ = createWalletSolana(name_, message_, amount_, showAmount_)
	// Get Solana address and desired balance from request
	address := wallet_.KeyPublic
	donoStr := fmt.Sprintf("%.4f", wallet_.DonoAmount)

	s.Amount = donoStr

	if wallet_.DonoName == "" {
		s.Name = "Anonymous"
		wallet_.DonoName = s.Name
	} else {
		s.Name = html.EscapeString(truncateStrings(condenseSpaces(wallet_.DonoName), NameMaxChar))
	}

	s.Media = html.EscapeString(media_)
	s.PayID = wallet_.KeyPublic
	s.Address = wallet_.KeyPublic
	s.IsSolana = !mon_

	params.Add("id", s.Address)
	s.CheckURL = params.Encode()

	tmp, _ := qrcode.Encode("solana:"+address+"?amount="+donoStr, qrcode.Low, 320)
	s.QRB64 = base64.StdEncoding.EncodeToString(tmp)

	err := payTemplate.Execute(w, s)
	if err != nil {
		fmt.Println(err)
	}

	return address
}

func handleMoneroPayment(w http.ResponseWriter, s *superChat, params url.Values) {
	payload := strings.NewReader(`{"jsonrpc":"2.0","id":"0","method":"make_integrated_address"}`)
	req, err := http.NewRequest("POST", rpcURL, payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &rpcResponse{}
	if err := json.NewDecoder(res.Body).Decode(resp); err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.PayID = html.EscapeString(resp.Result.PaymentID)
	params.Add("id", resp.Result.PaymentID)
	s.Address = resp.Result.IntegratedAddress
	s.CheckURL = params.Encode()

	tmp, _ := qrcode.Encode(fmt.Sprintf("monero:%s?tx_amount=%s", resp.Result.IntegratedAddress, s.Amount), qrcode.Low, 320)
	s.QRB64 = base64.StdEncoding.EncodeToString(tmp)

	err = payTemplate.Execute(w, s)
	if err != nil {
		fmt.Println(err)
	}
}

func SendSolana(senderPublicKey string, senderPrivateKey ed25519.PrivateKey, recipientAddress string, amount float64) {

	var feePayer, _ = types.AccountFromBytes(senderPrivateKey) // fill your private key here (u8 array)

	resp, err := c.GetLatestBlockhash(context.Background())
	if err != nil {
		log.Fatalf("failed to get recent blockhash, err: %v", err)
	}

	toPubkey := common.PublicKeyFromString(recipientAddress)
	log.Println(toPubkey)
	if err != nil {
		log.Fatalf("failed to parse recipient public key, err: %v", err)
	}

	log.Println("Public Key Payer:", feePayer.PublicKey)
	amountLamports := uint64(math.Round(amount * math.Pow10(9)))
	tx, err := types.NewTransaction(types.NewTransactionParam{
		Message: types.NewMessage(types.NewMessageParam{
			FeePayer:        feePayer.PublicKey,
			RecentBlockhash: resp.Blockhash,
			Instructions: []types.Instruction{
				system.Transfer(system.TransferParam{
					From:   feePayer.PublicKey,
					To:     toPubkey,
					Amount: amountLamports - uint64(lamportFee),
				}),
			},
		}),
		Signers: []types.Account{feePayer},
	})
	if err != nil {
		log.Fatalf("failed to build raw tx, err: %v", err)
	}
	sig, err := c.SendTransaction(context.Background(), tx)
	if err != nil {
		log.Fatalf("failed to send tx, err: %v", err)
	}
	fmt.Println(sig)

}
