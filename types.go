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
	ID           int      `db:"id" json:"id"`
	Domain       string   `db:"domain" json:"domain"`
	Expiration   int64    `db:"expiration" json:"expiration"`
	Nameservers  []string `db:"nameservers" json:"nameservers,omitempty"`
	Registrar    string   `db:"registrar" json:"registrar"`
	DNS          DNS      `db:"dns" json:"dns"`
	ClientID     int      `db:"clientid" json:"clientID"`
	RawWhoisData string   `db:"rawwhoisdata" json:"rawWhoisData"`
	Notes        *string  `db:"notes" json:"notes,omitempty"`
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
