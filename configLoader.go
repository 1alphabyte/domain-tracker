package main

import (
	"encoding/json"
	"log"
	"os"
)

func getConfigValue() Config {
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
