import { Database } from "bun:sqlite";

const db = new Database(process.env.DB_PATH || "./db.sqlite", { create: true });

db.query("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, password TEXT NOT NULL, lastLogin INTEGER)").run();
db.query("CREATE TABLE IF NOT EXISTS sessions (token TEXT NOT NULL, userId INTEGER NOT NULL, expires INTEGER NOT NULL, FOREIGN KEY(userId) REFERENCES users(id))").run();
db.query("CREATE TABLE IF NOT EXISTS domains (id INTEGER PRIMARY KEY, domain TEXT NOT NULL UNIQUE, expiration INTEGER NOT NULL, nameservers TEXT, registrar TEXT NOT NULL, clientId INTEGER NOT NULL, rawWhoisData TEXT NOT NULL, notes TEXT, FOREIGN KEY(clientId) REFERENCES clients(id))").run();
db.query("CREATE TABLE IF NOT EXISTS clients (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE)").run();

// load init user
const password = await Bun.password.hash(process.env.INIT_PASSWORD || "admin123");
db.query("INSERT INTO users (username, password) VALUES (?1, ?2)").run(process.env.INIT_USER || "admin", password);