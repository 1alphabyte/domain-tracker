package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

func getConfig() Config {
	file, err := os.ReadFile("./config.json")
	if err != nil {
		log.Fatal("Failed to read config", err)
	}

	var conf Config
	if err = json.Unmarshal(file, &conf); err != nil {
		log.Fatal("Invalid config", err)
	}
	return conf
}

func generateSessionToken() string {
	// A 32-byte token provides 256 bits of randomness, which is secure.
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	return base64.URLEncoding.EncodeToString(b)
}

func checkSessionToken(r *http.Request) (int, error) {
	cookie, err := r.Cookie("auth")
	if err != nil {
		return -1, err
	}

	// Get the database connection
	db := setupDatabase()

	var session Session
	err = db.QueryRow(context.TODO(), "SELECT userId, expires FROM sessions WHERE token = $1", cookie.Value).Scan(&session.UserID, &session.Expiry)
	if err != nil {
		return -1, err
	}

	// check if the session is expired
	if session.Expiry < time.Now().Unix() {
		return -1, errors.New("session expired")
	}
	return session.UserID, nil
}
