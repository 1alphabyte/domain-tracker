package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/openrdap/rdap"
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

func checkSessionToken(r *http.Request) (int, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return -1, err
	}

	// Get the database connection
	db := setupDatabase()

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

func ResolveDNS(domain string, class string) string {
	dnsRes, err := http.Get("https://dns.google/resolve?name=" + domain + "&type=" + class)
	if err != nil {
		log.Print(err)
		return "<error>"
	}
	defer dnsRes.Body.Close()
	var DNSResponse GoogDNSResponse
	if err := json.NewDecoder(dnsRes.Body).Decode(&DNSResponse); err != nil {
		log.Print(err)
		return "<error>"
	}
	if DNSResponse.Status == 0 {
		Records := make([]string, 0, len(DNSResponse.Answer))
		for _, ans := range DNSResponse.Answer {
			Records = append(Records, ans.Data)
		}
		return strings.Join(Records, ",")
	}
	return ""
}

func fetchDomainData(domain string) (exp time.Time, ns string, reg string, rawData string, dns DNS, err error) {
	client := &rdap.Client{}

	query, err := client.QueryDomain(domain)
	if err != nil {
		log.Print(err)
		// Try to fall back to whois
		result, err := whois.Whois(domain)
		if err != nil {
			log.Println(err)
			return time.Time{}, "", "", "", DNS{}, err
		}
		res, err := whoisparser.Parse(result)
		if err != nil {
			log.Println(err)
			return time.Time{}, "", "", "", DNS{}, err
		}

		exp = *res.Domain.ExpirationDateInTime
		// Get nameservers
		nameservers := make([]string, 0, len(res.Domain.NameServers))
		nameservers = append(nameservers, res.Domain.NameServers...)

		jsonNs, _ := json.Marshal(nameservers)
		ns = string(jsonNs)
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
					return time.Time{}, "", "", "", DNS{}, err
				}
				break
			}
		}

		// Get nameservers
		nameservers := make([]string, 0, len(query.Nameservers))
		for _, ns := range query.Nameservers {
			nameservers = append(nameservers, ns.LDHName)
		}
		jsonNs, _ := json.Marshal(nameservers)
		ns = string(jsonNs)

		// Get registrar
		for _, entity := range query.Entities {
			if entity.Roles[0] == "registrar" {
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
	}
	return exp, ns, reg, rawData, dns, nil
}
