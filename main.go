package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
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
	w.Header().Set("Set-Cookie", "auth="+token+"; Max-Age=86400; Path=/; httpOnly; SameSite=Strict; Secure")
}

func main() {
	InitDBSetup()

	// go func() {
	// 	ticker := time.NewTicker(24 * time.Hour)
	// 	for range ticker.C {
	// 		dbCleanup()
	// 	}
	// }()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/login", loginHandler)
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
