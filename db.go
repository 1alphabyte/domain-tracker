package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func InitDBSetup() {
	// check if the database has already been initialized
	if _, err := os.Stat(getConfig().DBInitFile); err == nil {
		// Database has already been initialized
		return
	}

	db := setupDatabase()

	defer db.Close(context.Background())

	_, err := db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password BYTEA NOT NULL
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create users table: %v\n", err)
	}

	_, err = db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS sessions (
			token VARCHAR(48) NOT NULL,
			userId INTEGER NOT NULL,
			expires TIMESTAMPTZ NOT NULL,
			FOREIGN KEY(userId) REFERENCES users(id)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create sessions table: %v\n", err)
	}

	_, err = db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS clients (
			id SERIAL PRIMARY KEY,
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
			expiration TIMESTAMPTZ NOT NULL,
			nameservers JSONB,
			registrar TEXT NOT NULL,
			dns JSON NOT NULL,
			clientId INTEGER NOT NULL,
			rawWhoisData JSON NOT NULL,
			notes TEXT,
			FOREIGN KEY(clientId) REFERENCES clients(id)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create domains table: %v\n", err)
	}

	_, err = db.Exec(context.TODO(), `
		CREATE TABLE IF NOT EXISTS crts (
			id SERIAL PRIMARY KEY,
			domain TEXT NOT NULL UNIQUE,
			commonName TEXT NOT NULL UNIQUE,
			expiration TIMESTAMPTZ NOT NULL,
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

	// create an initial user
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(getConfig().InitPwd), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalln("Failed to hash password:", err)
	}

	_, err = db.Exec(context.TODO(), "INSERT INTO users (username, password) VALUES ($1, $2)", getConfig().InitUsr, hashedPassword)
	if err != nil {
		log.Fatalf("Failed to create initial user: %v\n", err)
	}

	// Create a file to indicate that the database has been initialized
	if err := os.WriteFile(getConfig().DBInitFile, []byte{}, 0644); err != nil {
		log.Fatalf("Failed to create db init file (check cfg): %v\n", err)
	}
}

func setupDatabase() *pgx.Conn {
	conn, err := pgx.Connect(context.Background(), getConfig().DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	return conn
}
