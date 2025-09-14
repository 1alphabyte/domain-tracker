package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Handle the /api/login route
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode the JSON request body (should have the username and password)
	var loginReq LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Make sure the username and password are not empty
	if loginReq.Username == "" || loginReq.Password == "" {
		http.Error(w, "Missing credentials", http.StatusUnauthorized)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	// Find the user with the username in the DB
	var user DbUser
	err := db.QueryRow(context.TODO(), "SELECT id, password FROM users WHERE username = $1", loginReq.Username).Scan(&user.ID, &user.Password)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "Invalid username or password", http.StatusForbidden)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	// Compare the provided password with the stored hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusForbidden)
		return
	}

	// generate a new session token
	token := generateSessionToken()
	// store token in the database
	_, err = db.Exec(context.TODO(), "INSERT INTO sessions (token, userId, expires) VALUES ($1, $2, $3)", token, user.ID, time.Now().Add(24*time.Hour))
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	// Set cookie on client
	w.Header().Set("Set-Cookie", "session="+token+"; Max-Age=86400; Path=/; httpOnly; SameSite=Strict; Secure")
}

// Handle the /api/get route
func getHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all rows from the domains SQL table
	rows, err := db.Query(context.TODO(), "SELECT * FROM domains")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	defer rows.Close()

	// Convert all rows to a array of JSON objects
	domains, err := pgx.CollectRows(rows, pgx.RowToStructByName[Domain])
	if err != nil {
		http.Error(w, "Error reading domains", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	// Check if there are no domains
	if len(domains) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Return the domains as JSON to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

// Handle the /api/edit route
func editHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Decode the JSON request body
	var req EditReqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println(err)
		return
	}

	// Make sure the required fields are present
	if req.ID == 0 || req.ClientID == 0 {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	c, err := db.Exec(context.TODO(), "UPDATE domains SET clientid = $1, notes = $2 WHERE id = $3", req.ClientID, req.Notes, req.ID)
	if err != nil {
		http.Error(w, "Failed to update domain", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	// Make sure the domain was found (and updated)
	if c.RowsAffected() == 0 {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}
}

// Handle (/api/add) adding a domain to the DB and fetching additional (required) metadata (using RDAP [preferred] or whois)
func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Decode the JSON request body
	var domain DomainReqBody
	if err := json.NewDecoder(r.Body).Decode(&domain); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println(err)
		return
	}
	// Make sure the required fields are present
	if domain.ClientID == 0 || domain.Domain == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Fetch domain data (helpers.go)
	exp, ns, reg, rawData, dns, err := fetchDomainData(domain.Domain)
	if err != nil {
		http.Error(w, "Failed to fetch domain data", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	// Insert the new domain into the DB
	_, err = db.Exec(context.TODO(), "INSERT INTO domains (domain, expiration, nameservers, registrar, dns, clientid, rawwhoisdata, notes) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		domain.Domain,
		exp,
		ns,
		reg,
		dns,
		domain.ClientID,
		rawData,
		domain.Notes,
	)
	if err != nil {
		log.Print(err)
		http.Error(w, "Failed to add domain", http.StatusInternalServerError)
		return
	}
}

func clientListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all rows from the clients SQL table
	rows, err := db.Query(context.TODO(), "SELECT * FROM clients")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	defer rows.Close()

	// Convert all rows to a array of JSON objects
	clients, err := pgx.CollectRows(rows, pgx.RowToStructByName[Client])
	if err != nil {
		http.Error(w, "Error reading clients", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	// Check if there are no clients
	if len(clients) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func clientAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get the request body and parse it
	var client Client
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println(err)
		return
	}

	// Make sure the required fields are present
	if client.Name == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	_, err = db.Exec(context.TODO(), "INSERT INTO clients (name) VALUES ($1)", client.Name)
	if err != nil {
		http.Error(w, "Failed to add client", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(client)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract the ID from the URL path
	// Expected format: /api/delete/:id
	id := strings.Split(r.URL.Path, "/")[3]
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	c, err := db.Exec(context.TODO(), "DELETE FROM domains WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	// Make sure the domain was found (and deleted)
	if c.RowsAffected() == 0 {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}
}

func deleteClientHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract the ID from the URL path
	// Expected format: /api/deleteClient/:id
	id := strings.Split(r.URL.Path, "/")[3]
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	c, err := db.Exec(context.TODO(), "DELETE FROM clients WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete client", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	// Make sure the client was found (and deleted)
	if c.RowsAffected() == 0 {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}
}

// Allow all domains to be refreshed manually (no frontend for now must call the endpoint directly)
func manRefHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Refresh all domains
	go func() {
		updateDomains()
		sendExpDomReminders()
	}()

	w.WriteHeader(http.StatusAccepted)
}

// Allow adding domain to track TLS certs
func tlsAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Decode the JSON request body
	var domain DomainReqBody
	if err := json.NewDecoder(r.Body).Decode(&domain); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println(err)
		return
	}
	// Make sure the required fields are present
	if domain.ClientID == 0 || domain.Domain == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Connect to the server of the domain on port 443 and get the TLS certificate
	conn, err := tls.Dial("tcp", domain.Domain+":443", nil)
	if err != nil {
		http.Error(w, "Failed to connect to domain on port 443", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer conn.Close()

	// Get the certificate
	cert := conn.ConnectionState().PeerCertificates[0]
	certJSON, err := json.Marshal(cert)
	if err != nil {
		http.Error(w, "Failed to marshal certificate", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	// Insert the new domain into the DB
	_, err = db.Exec(context.TODO(), "INSERT INTO crts (domain, commonName, expiration, authority, clientId, rawData, notes) VALUES ($1, $2, $3, $4, $5, $6, $7);",
		domain.Domain,
		cert.Subject.CommonName,
		cert.NotAfter,
		cert.Issuer.Organization[0],
		domain.ClientID,
		certJSON,
		domain.Notes,
	)
	if err != nil {
		log.Print(err)
		http.Error(w, "Failed to add domain", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func tlsListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all rows from the crts SQL table
	rows, err := db.Query(context.TODO(), "SELECT * FROM crts")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	defer rows.Close()

	// Convert all rows to a array of JSON objects
	domains, err := pgx.CollectRows(rows, pgx.RowToStructByName[TLSDomain])
	if err != nil {
		http.Error(w, "Error reading domains", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	// Check if there are no domains
	if len(domains) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Return the domains as JSON to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

func deleteTLSHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check session token
	userID, err := checkSessionToken(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if userID != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract the ID from the URL path
	// Expected format: /api/delete/:id
	id := strings.Split(r.URL.Path, "/")[3]
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	db := setupDatabase()
	defer db.Close(context.Background())
	c, err := db.Exec(context.TODO(), "DELETE FROM crts WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	// Make sure the domain was found (and deleted)
	if c.RowsAffected() == 0 {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}
}

func main() {
	// Initialize the database (create tables if they don't exist)
	InitDBSetup()

	// Backgrounds tasks using a goroutine and ticker
	// Send weekly expiration reminders and update domain info
	// Runs every 7 days
	go func() {
		ticker := time.NewTicker(7 * 24 * time.Hour)
		for range ticker.C {
			dbCleanup()
			updateDomains()
			sendExpDomReminders()
		}
	}()

	// Check nameservers every 24 hours
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			detectNameserverChanges()
		}
	}()

	// Set up HTTP routes
	mux := http.NewServeMux()

	mux.HandleFunc("/api/login", loginHandler)
	mux.HandleFunc("/api/get", getHandler)
	mux.HandleFunc("/api/edit", editHandler)
	mux.HandleFunc("/api/add", addHandler)
	mux.HandleFunc("/api/clientList", clientListHandler)
	mux.HandleFunc("/api/clientAdd", clientAddHandler)
	mux.HandleFunc("/api/delete/", deleteHandler)
	mux.HandleFunc("/api/refreshAll", manRefHandler)
	mux.HandleFunc("/api/deleteClient", deleteClientHandler)
	mux.HandleFunc("/api/tlsAddDomain", tlsAddHandler)
	mux.HandleFunc("/api/tlsList", tlsListHandler)
	mux.HandleFunc("/api/tlsDelete/", deleteTLSHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		} else if r.URL.Path == "/" {
			http.Redirect(w, r, "/login/", http.StatusFound)
		}

		http.ServeFile(w, r, "./static"+r.URL.Path)
	})

	if err := http.ListenAndServe(getConfig().ListenAddr, mux); err != nil {
		log.Fatal(err)
	}
}
