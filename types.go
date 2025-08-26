package main

import "time"

type Config struct {
	DatabaseURL   string `json:"databaseURL"`
	InitPwd       string `json:"initPassword"`
	InitUsr       string `json:"initUser"`
	ListenAddr    string `json:"listenAddr"`
	DaysDomainExp int    `json:"remindDomainExpDays"`
	EmailForExp   string `json:"to_email"`
	FromEmail     string `json:"from_email"`
	SMTPHost      string `json:"smtp_host"`
	SMTP_USER     string `json:"SMTP_USER"`
	SMTPPass      string `json:"SMTP_PASSWORD"`
	SMTPPort      int    `json:"smtp_port"`
	BaseURL       string `json:"baseURL"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type DbUser struct {
	ID       int
	Username string
	Password []byte
}

type Session struct {
	Token  string
	UserID int
	Expiry time.Time
}

type Domain struct {
	ID           int       `db:"id" json:"id"`
	Domain       string    `db:"domain" json:"domain"`
	Expiration   time.Time `db:"expiration" json:"expiration"`
	Nameservers  []string  `db:"nameservers" json:"nameservers,omitempty"`
	Registrar    string    `db:"registrar" json:"registrar"`
	DNS          DNS       `db:"dns" json:"dns"`
	ClientID     int       `db:"clientid" json:"clientID"`
	RawWhoisData string    `db:"rawwhoisdata" json:"rawWhoisData"`
	Notes        *string   `db:"notes" json:"notes,omitempty"`
}

type Client struct {
	ID   int    `db:"id"`
	Name string `db:"name" json:"name"`
}

type DomainReqBody struct {
	Domain   string `json:"domain" binding:"required"`
	ClientID int    `json:"clientID" binding:"required"`
	Notes    string `json:"notes,omitempty"`
}

type EditReqBody struct {
	ID       int    `json:"id" binding:"required"`
	ClientID int    `json:"clientID" binding:"required"`
	Notes    string `json:"notes,omitempty"`
}

type DNS struct {
	A    string `json:"a"`
	AAAA string `json:"aaaa"`
	MX   string `json:"mx"`
}

type DNSQuestion struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type DNSAnswer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
	TTL  int
	Data string `json:"data"`
}

type GoogDNSResponse struct {
	Status   int
	TC       bool
	RD       bool
	RA       bool
	AD       bool
	CD       bool
	Question []DNSQuestion
	Answer   []DNSAnswer
	Comment  string
}
