package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/openrdap/rdap"
	"github.com/wneessen/go-mail"
)

// emailHTML wraps email content in a consistent branded template
func emailHTML(title, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f0f4f8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f0f4f8;padding:32px 16px;">
<tr><td align="center">
<table width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%%;">
  <tr>
    <td style="background-color:#0d1117;padding:24px 32px;border-radius:12px 12px 0 0;">
      <h1 style="margin:6px 0 0;font-size:20px;font-weight:700;color:#e6edf3;">%s</h1>
    </td>
  </tr>
  <tr>
    <td style="background-color:#ffffff;padding:28px 32px;">
      %s
    </td>
  </tr>
  <tr>
    <td style="background-color:#f6f8fa;padding:14px 32px;border-radius:0 0 12px 12px;border-top:1px solid #e1e4e8;">
      <p style="margin:0;font-size:12px;color:#6e7681;">Powered by Domain Tracker &middot; California Business Technology&reg; Inc.</p>
    </td>
  </tr>
</table>
</td></tr>
</table>
</body>
</html>`, title, content)
}

// domainCard returns an HTML card for a single domain/cert row in an email
func domainCard(borderColor, linkURL, name, subtitle, badge string) string {
	return fmt.Sprintf(`
<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:12px;border:1px solid #e1e4e8;border-radius:8px;overflow:hidden;">
  <tr>
    <td style="padding:14px 18px;border-left:4px solid %s;background-color:#ffffff;">
      <p style="margin:0;font-size:16px;font-weight:600;"><a href="%s" style="color:#29a8e1;text-decoration:none;">%s</a></p>
      <p style="margin:5px 0 0;font-size:13px;color:#57606a;">%s</p>
      <p style="margin:5px 0 0;font-size:13px;font-weight:600;color:%s;">%s</p>
    </td>
  </tr>
</table>`, borderColor, linkURL, name, subtitle, borderColor, badge)
}

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

func writeConfig(conf Config) {
	jsonData, err := json.MarshalIndent(conf, "", "		")
	if err != nil {
		log.Fatal("Failed to marshal config", err)
	}
	if err = os.WriteFile("./config.json", jsonData, 0644); err != nil {
		log.Fatal("Failed to write config", err)
	}
}

func generateSessionToken() string {
	// A 32-byte token provides 256 bits of randomness, which is secure.
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	return base64.URLEncoding.EncodeToString(b)
}

func checkSessionToken(r *http.Request) (int, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return -1, err
	}

	var session Session
	err = db.QueryRow(context.TODO(), "SELECT userId, expires FROM sessions WHERE token = $1", cookie.Value).Scan(&session.UserID, &session.Expiry)
	if err != nil {
		return -1, err
	}

	// check if the session is expired
	if time.Now().After(session.Expiry) {
		return -1, errors.New("session expired")
	}
	return session.UserID, nil
}

func ResolveDNS(domain string, class string) []string {
	dnsRes, err := http.Get("https://dns.google/resolve?name=" + domain + "&type=" + class)
	if err != nil {
		log.Print(err)
		return []string{}
	}
	defer dnsRes.Body.Close()
	var DNSResponse GoogDNSResponse
	if err := json.NewDecoder(dnsRes.Body).Decode(&DNSResponse); err != nil {
		log.Print(err)
		return []string{}
	}
	if DNSResponse.Status == 0 {
		Records := make([]string, 0, len(DNSResponse.Answer))
		for _, ans := range DNSResponse.Answer {
			Records = append(Records, strings.ToLower(ans.Data))
		}
		return Records
	}
	return []string{}
}

func fetchDomainData(domain string) (exp time.Time, ns []string, reg string, rawData string, dns DNS, err error) {
	client := &rdap.Client{}

	query, err := client.QueryDomain(domain)
	if err != nil {
		log.Print(err)
		// Try to fall back to whois
		result, err := whois.Whois(domain)
		if err != nil {
			log.Println(err)
			return time.Time{}, []string{}, "", "", DNS{}, err
		}
		res, err := whoisparser.Parse(result)
		if err != nil {
			log.Println(err)
			return time.Time{}, []string{}, "", "", DNS{}, err
		}

		exp = *res.Domain.ExpirationDateInTime
		// Get nameservers
		nameservers := make([]string, 0, len(res.Domain.NameServers))
		for _, ns := range res.Domain.NameServers {
			nameservers = append(nameservers, strings.ToLower(ns))
		}

		ns = nameservers
		reg = res.Registrar.Name
		mRawData, e := json.Marshal(res)
		if e != nil {
			log.Print(e)
			rawData = "<error>"
		} else {
			rawData = string(mRawData)
		}
	} else {
		// Extract expiration date from RDAP events
		for i := range query.Events {
			if query.Events[i].Action == "expiration" {
				exp, err = time.Parse(time.RFC3339, query.Events[i].Date)
				if err != nil {
					log.Print(err)
					return time.Time{}, []string{}, "", "", DNS{}, err
				}
				break
			}
		}

		// Get nameservers
		nameservers := make([]string, 0, len(query.Nameservers))
		for _, ns := range query.Nameservers {
			nameservers = append(nameservers, strings.ToLower(ns.LDHName))
		}
		ns = nameservers

		// Get registrar
		for _, entity := range query.Entities {
			if len(entity.Roles) > 0 && entity.Roles[0] == "registrar" {
				reg = entity.VCard.Name()
			}
		}

		// Get the raw RDAP JSON response
		jsonBytes, err := json.Marshal(query)
		if err != nil {
			// soft fail
			log.Printf("Error marshaling RDAP response to JSON: %v", err)
			rawData = "<error>"
		} else {
			rawData = string(jsonBytes)
		}
	}

	// Get DNS
	dns = DNS{
		A:    ResolveDNS(domain, "A"),
		AAAA: ResolveDNS(domain, "AAAA"),
		MX:   ResolveDNS(domain, "MX"),
		NS:   ResolveDNS(domain, "NS"),
	}

	return exp, ns, reg, rawData, dns, nil
}

func sendEmail(subj string, body string) error {
	message := mail.NewMsg(mail.WithNoDefaultUserAgent())
	if err := message.From(getConfig().FromEmail); err != nil {
		log.Print("failed to set From address:", err)
	}
	if err := message.ToFromString(getConfig().EmailForExp); err != nil {
		log.Print("failed to set To address:", err)
	}
	message.SetMessageIDWithValue(generateSessionToken() + "@domain-tracker")
	message.SetGenHeader("X-Mailer", "utsav2.dev/domain-tracker/v3 (https://github.com/1alphabyte/domain-tracker)")
	message.Subject(subj)

	message.SetBodyString(mail.TypeTextHTML, body)

	// --- Create the client ---
	c, err := mail.NewClient(
		getConfig().SMTPHost,
		mail.WithPort(getConfig().SMTPPort),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(getConfig().SMTP_USER),
		mail.WithPassword(getConfig().SMTPPass),
		mail.WithSSL(),
	)
	if err != nil {
		return err
	}
	// Send the email
	if err := c.DialAndSend(message); err != nil {
		return err
	}
	return nil
}

func TrimDot(ns []string) []string {
	normalized := make([]string, len(ns))
	for i, s := range ns {
		normalized[i] = strings.TrimSuffix(s, ".")
	}
	return normalized
}

func getTLSCert(domain string) (CommonName string, validTo time.Time, issuer string, rawData []byte, err error) {
	// Connect to the server of the domain on port 443 and get the TLS certificate
	conn, err := tls.Dial("tcp", domain+":443", nil)
	if err != nil {
		return "", time.Time{}, "", nil, err
	}
	defer conn.Close()

	// Get the certificate
	cert := conn.ConnectionState().PeerCertificates[0]
	certJSON, err := json.Marshal(cert)
	if err != nil {
		return "", time.Time{}, "", nil, err
	}
	return cert.Subject.CommonName, cert.NotAfter, cert.Issuer.CommonName, certJSON, nil
}
