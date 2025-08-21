package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

func InitDBSetup() {
	db := setupDatabase()

	defer db.Close(context.Background())

	_, err := db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password BYTEA NOT NULL,
			lastLogin BIGINT
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create users table: %v\n", err)
	}

	_, err = db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT NOT NULL,
			userId INTEGER NOT NULL,
			expires BIGINT NOT NULL,
			FOREIGN KEY(userId) REFERENCES users(id)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create sessions table: %v\n", err)
	}

	_, err = db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS clients (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create clients table: %v\n", err)
	}

	_, err = db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS domains (
			id SERIAL PRIMARY KEY,
			domain TEXT NOT NULL UNIQUE,
			expiration BIGINT NOT NULL,
			nameservers TEXT,
			registrar TEXT NOT NULL,
			dns TEXT NOT NULL,
			clientId INTEGER NOT NULL,
			rawWhoisData TEXT NOT NULL,
			notes TEXT,
			FOREIGN KEY(clientId) REFERENCES clients(id)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create domains table: %v\n", err)
	}

	_, err = db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS crts (
			id INTEGER PRIMARY KEY,
			domain TEXT NOT NULL UNIQUE,
			commonName TEXT NOT NULL UNIQUE,
			expiration INTEGER NOT NULL,
			authority TEXT,
			clientId INTEGER NOT NULL,
			rawData TEXT NOT NULL,
			notes TEXT,
			FOREIGN KEY(clientId) REFERENCES clients(id)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create crts table: %v\n", err)
	}
}

func setupDatabase() *pgx.Conn {
	conn, err := pgx.Connect(context.Background(), getConfigValue().DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	return conn
}
