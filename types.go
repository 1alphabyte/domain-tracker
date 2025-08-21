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

type Session struct {
	Token  string
	UserID int
	Expiry int64
}

type Domain struct {
	ID           int     `db:"id"`
	Domain       string  `db:"domain"`
	Expiration   int64   `db:"expiration"`
	Nameservers  *string `db:"nameservers"`
	Registrar    string  `db:"registrar"`
	DNS          string  `db:"dns"`
	ClientID     int     `db:"clientid"`
	RawWhoisData string  `db:"rawwhoisdata"`
	Notes        *string `db:"notes"`
}
