package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/openrdap/rdap"
	"golang.org/x/crypto/bcrypt"
)

// Handle the /api/login route
func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is POST
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

	// Get the database connection
	db := setupDatabase()

	// find the user with the username in the DB
	var user DbUser
	err := db.QueryRow(context.TODO(), "SELECT id, password, lastLogin FROM users WHERE username = $1", loginReq.Username).Scan(&user.ID, &user.Password, &user.LastLogin)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "Invalid username or password", http.StatusForbidden)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	// Compare the provided password with the stored hashed password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusForbidden)
		return
	}

	// generate a new session token
	token := generateSessionToken()
	// store token in the database
	_, err = db.Exec(context.TODO(), "INSERT INTO sessions (token, userId, expires) VALUES ($1, $2, $3)", token, user.ID, time.Now().Add(24*time.Hour).Unix())
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	// Set cookie on client
	w.Header().Set("Set-Cookie", "session="+token+"; Max-Age=86400; Path=/; httpOnly; SameSite=Strict; Secure")
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is GET
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

	// Get all rows from the domains SQL table
	rows, err := db.Query(context.TODO(), "SELECT * FROM domains")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	defer rows.Close()

	// get all domains as a array of JSON objects
	domains, err := pgx.CollectRows(rows, pgx.RowToStructByName[Domain])
	if err != nil {
		http.Error(w, "Error reading domains", http.StatusInternalServerError)
		log.Print(err)
		return
	}

	if len(domains) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

// Handle adding a domain to the DB and fetching additional (required) metadata (using RDAP or whois)
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
	if domain.ClientID == 0 || domain.Domain == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}
	client := &rdap.Client{}

	var exp int64
	var ns, reg, rawData string
	query, err := client.QueryDomain(domain.Domain)
	if err != nil {
		log.Print(err)
		// Try to fall back to whois
		result, err := whois.Whois(domain.Domain)
		if err != nil {
			log.Println(err)
			http.Error(w, "Failed to retrieve WHOIS information", http.StatusInternalServerError)
			return
		}
		res, err := whoisparser.Parse(result)
		if err != nil {
			log.Println(err)
			http.Error(w, "Failed to parse WHOIS information", http.StatusInternalServerError)
			return
		}

		exp = res.Domain.ExpirationDateInTime.Unix()
		// Get nameservers
		nameservers := make([]string, 0, len(res.Domain.NameServers))
		for _, ns := range res.Domain.NameServers {
			nameservers = append(nameservers, ns)
		}
		jsonNs, err := json.Marshal(nameservers)
		ns = string(jsonNs)
		reg = res.Registrar.Name
		mRawData, _ := json.Marshal(res)
		rawData = string(mRawData)
	} else {
		// Extract expiration date from RDAP events
		for i := range query.Events {
			if query.Events[i].Action == "expiration" {
				tmp, err := time.Parse(time.RFC3339, query.Events[i].Date)
				if err != nil {
					log.Print(err)
					http.Error(w, "Failed to parse expiration date", http.StatusInternalServerError)
					return
				}
				exp = tmp.Unix()
				break
			}
		}

		// Get nameservers
		nameservers := make([]string, 0, len(query.Nameservers))
		for _, ns := range query.Nameservers {
			nameservers = append(nameservers, ns.LDHName)
		}
		jsonNs, err := json.Marshal(nameservers)
		ns = string(jsonNs)

		// Get registrar
		for _, entity := range query.Entities {
			if entity.Roles[0] == "registrar" {
				reg = entity.VCard.Name()
			}
		}

		// Get the raw RDAP JSON response
		jsonBytes, err := json.Marshal(query)
		if err != nil {
			// soft fail
			log.Printf("Error marshaling RDAP response to JSON: %v", err)
			rawData = "<error>"
		} else {
			rawData = string(jsonBytes)
		}
	}

	// Get DNS
	dns := DNS{
		A:    ResolveDNS(domain.Domain, "A"),
		AAAA: ResolveDNS(domain.Domain, "AAAA"),
		MX:   ResolveDNS(domain.Domain, "MX"),
	}

	// Connect to the DB
	db := setupDatabase()

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
	// Check if the request method is GET
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

	// Get all rows from the clients SQL table
	rows, err := db.Query(context.TODO(), "SELECT * FROM clients")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	defer rows.Close()

	// get all clients as a array of JSON objects
	clients, err := pgx.CollectRows(rows, pgx.RowToStructByName[Client])
	if err != nil {
		http.Error(w, "Error reading clients", http.StatusInternalServerError)
		log.Print(err)
		return
	}

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
	if client.Name == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	db := setupDatabase()

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
	id := strings.Split(r.URL.Path, "/")[3]
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	db := setupDatabase()

	c, err := db.Exec(context.TODO(), "DELETE FROM domains WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	if c.RowsAffected() == 0 {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	InitDBSetup()

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			dbCleanup()
		}
	}()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/login", loginHandler)
	mux.HandleFunc("/api/get", getHandler)
	mux.HandleFunc("/api/add", addHandler)
	mux.HandleFunc("/api/clientList", clientListHandler)
	mux.HandleFunc("/api/clientAdd", clientAddHandler)
	mux.HandleFunc("/api/delete/", deleteHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		} else if r.URL.Path == "/" {
			http.Redirect(w, r, "/login/", http.StatusFound)
		}

		http.ServeFile(w, r, "./static"+r.URL.Path)
	})

	if err := http.ListenAndServe("127.0.0.1:8746", mux); err != nil {
		log.Fatal(err)
	}
}
