package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
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
