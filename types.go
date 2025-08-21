package main

type Config struct {
	DatabaseURL string `json:"databaseURL"`
	InitPwd     string `json:"initPassword"`
	InitUsr     string `json:"initUser"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type DbUser struct {
	ID        int
	Username  string
	Password  []byte
	LastLogin int64
}
