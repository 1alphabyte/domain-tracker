package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v5"
)

func dbCleanup() {
	db := setupDatabase()
	defer db.Close(context.Background())

	// Delete expired sessions
	delSess, err := db.Exec(context.TODO(), "DELETE FROM sessions WHERE expires < $1", time.Now())
	if err != nil {
		log.Printf("Failed to delete expired sessions: %v\n", err)
	}
	log.Printf("Deleted %d expired session(s)\n", delSess.RowsAffected())
}

func updateDomains() {
	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all domains
	rows, err := db.Query(context.TODO(), "SELECT * FROM domains")
	if err != nil {
		log.Printf("Failed to get domains: %v\n", err)
		return
	}
	defer rows.Close()

	// Collect rows into a slice of Domain structs
	domains, err := pgx.CollectRows(rows, pgx.RowToStructByName[Domain])
	if err != nil {
		log.Printf("Failed to collect domains: %v\n", err)
		return
	}

	// Iterate over each domain and check expiration
	for _, d := range domains {
		// get stored expiration date
		currTime := time.Now().AddDate(0, 0, getConfig().DaysDomainExp)

		// Check if the domain expires within the configured reminder period
		// If it does, refresh its data
		if currTime.After(d.Expiration) {
			log.Printf("Refreshing domain: %s\n", d.Domain)
			exp, ns, reg, rawData, dns, err := fetchDomainData(d.Domain)
			if err != nil {
				log.Println(err)
				continue
			}

			_, err = db.Exec(context.TODO(), "UPDATE domains SET expiration = $1, nameservers = $2, registrar = $3, rawWhoisData = $4, dns = $5 WHERE id = $6",
				exp, ns, reg, rawData, dns, d.ID)
			if err != nil {
				log.Printf("Failed to update domain %s: %v\n", d.Domain, err)
				continue
			}

			// TSK: consider for removal
			log.Printf("Updated domain: %s\n", d.Domain)
			time.Sleep(15 * time.Second) // to avoid rate limiting
		}
	}
}

func sendExpDomReminders() {
	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all domains
	rows, err := db.Query(context.TODO(), "SELECT * FROM domains")
	if err != nil {
		log.Printf("Failed to get domains: %v\n", err)
		return
	}
	defer rows.Close()

	domains, err := pgx.CollectRows(rows, pgx.RowToStructByName[Domain])
	if err != nil {
		log.Printf("Failed to collect domains: %v\n", err)
		return
	}
	// Array to hold domains needing reminders
	var needReminder []Domain

	// Populate the array
	for _, d := range domains {
		currTime := time.Now().AddDate(0, 0, getConfig().DaysDomainExp)

		// Check if the domain expires within the configured reminder period
		if currTime.After(d.Expiration) {
			needReminder = append(needReminder, d)
		}
	}

	log.Printf("Sending expiration reminder for %d domains", len(needReminder))

	var domainList string

	if len(needReminder) == 0 {
		log.Println("No domains need reminders, skipping email.")
		return
	} else {
		for _, d := range needReminder {
			// check if less than half the reminder period is remaining
			var inDurStr string
			warningThreshold := (time.Duration(getConfig().DaysDomainExp) * 24 * time.Hour) / 2
			critThreshold := (time.Duration(getConfig().DaysDomainExp) * 24 * time.Hour) / 3
			if time.Until(d.Expiration) < critThreshold {
				inDurStr = fmt.Sprintf("<b style='color: red;'>%s</b>⚠️", humanize.Time(d.Expiration))
			} else if time.Until(d.Expiration) < warningThreshold {
				inDurStr = fmt.Sprintf("<span style='color: orange;'>%s</span>", humanize.Time(d.Expiration))
			} else {
				inDurStr = humanize.Time(d.Expiration)
			}
			// add to the list
			domainList += fmt.Sprintf("<li><a href='%s/dash/?q=%s'>%s</a> is expiring on %s (in %s)</li>", getConfig().BaseURL, d.Domain, d.Domain, d.Expiration.Format("01/02/2006 @ 03:04:05PM"), inDurStr)
		}
	}

	err = sendEmail("Domains expiring soon", fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
			<head>
  				<meta charset="UTF-8">
  				<meta name="viewport" content="width=device-width, initial-scale=1.0">
  			</head>
			<body>
				<h3>The following domain(s) are expiring within the next %d days:</h3>
				<p>Click a domain to view it in Domain Tracker</p>
				<ul>
					%s
				</ul>
				<br />
				<footer style='font-size: smaller;'>
					Powered by <img src='https://assets.cdn.utsav2.dev:453/bucket/domaintrk/favicon.webp' style='width: 20px; border-radius: 50%%;' />Domain Tracker for <img src='https://cbt.io/wp-content/uploads/2023/07/favicon.png' style='width: 20px;' />California Business Technology<sup>®</sup> Inc.
				</footer>
			</body>
		</html>`, getConfig().DaysDomainExp, domainList))
	if err != nil {
		log.Printf("Failed to send expiration reminder email: %v\n", err)
	}
}

func detectNameserverChanges() {
	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all domains
	rows, err := db.Query(context.TODO(), "SELECT * FROM domains")
	if err != nil {
		log.Printf("Failed to get domains: %v\n", err)
		return
	}
	defer rows.Close()

	domains, err := pgx.CollectRows(rows, pgx.RowToStructByName[Domain])
	if err != nil {
		log.Printf("Failed to collect domains: %v\n", err)
		return
	}

	// Array to store changes
	var NSChanges []NSChange
	for _, d := range domains {
		ns := d.Nameservers
		if len(ns) == 0 {
			continue
		}
		// Fetch new nameserver
		newNs := ResolveDNS(d.Domain, "NS")
		if len(newNs) == 0 {
			log.Printf("Failed to resolve NS for domain %s\n", d.Domain)
			continue
		}

		// Compare old and new nameservers

		slices.Sort(newNs)
		slices.Sort(ns)

		newNs = TrimDot(newNs)
		ns = TrimDot(ns)

		// Check if there is a change
		googChange := !slices.Equal(ns, newNs)

		if googChange {
			// Store the change
			NSChanges = append(NSChanges, NSChange{
				Domain:    d.Domain,
				OldNS:     ns,
				NewNS:     newNs,
				CheckedAt: time.Now(),
			})

			// Update the database with the new nameservers
			_, err = db.Exec(context.TODO(), "UPDATE domains SET nameservers = $1 WHERE id = $2", newNs, d.ID)
			if err != nil {
				log.Printf("Failed to update nameservers for domain %s: %v\n", d.Domain, err)
				continue
			}
		}
	}

	if len(NSChanges) == 0 {
		log.Println("No nameserver changes detected.")
		return
	}

	// Send a alert of the change
	var listChanges string
	for _, change := range NSChanges {
		listChanges += fmt.Sprintf("<li><b>%s</b><br />Old NS: %s<br />New NS: %s<br />Checked At: %s</li><br />",
			change.Domain,
			strings.Join(change.OldNS, ", "),
			strings.Join(change.NewNS, ", "),
			change.CheckedAt.Format("01/02/2006 @ 03:04:05PM"),
		)
	}

	err = sendEmail("Domain nameserver changes detected", fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
			<head>
  				<meta charset="UTF-8">
  				<meta name="viewport" content="width=device-width, initial-scale=1.0">
  			</head>
			<body>
				<h3>The following domains' nameserver have changed:</h3>
				<ul>
					%s
				</ul>
				<br />
				<footer style='font-size: smaller;'>
					Powered by <img src='https://assets.cdn.utsav2.dev:453/bucket/domaintrk/favicon.webp' style='width: 20px; border-radius: 50%%;' />Domain Tracker for <img src='https://cbt.io/wp-content/uploads/2023/07/favicon.png' style='width: 20px;' />California Business Technology<sup>®</sup> Inc.
				</footer>
			</body>
		</html>`, listChanges))
	if err != nil {
		log.Printf("Failed to send nameserver change alert email: %v\n", err)
	}
}

func updateTLSCerts() {
	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all certificates
	rows, err := db.Query(context.TODO(), "SELECT * FROM crts")
	if err != nil {
		log.Printf("Failed to get certificates: %v\n", err)
		return
	}
	defer rows.Close()

	// Collect rows into a slice of Domain structs
	domains, err := pgx.CollectRows(rows, pgx.RowToStructByName[TLSDomain])
	if err != nil {
		log.Printf("Failed to collect certificates: %v\n", err)
		return
	}

	// Iterate over each certificate and check expiration
	for _, d := range domains {
		log.Printf("Refreshing certificate: %s\n", d.Domain)
		commonName, validTo, issuer, rawData, err := getTLSCert(d.Domain)
		if err != nil {
			log.Println(err)
			continue
		}

		_, err = db.Exec(context.TODO(), "UPDATE crts SET expiration = $1, authority = $2, rawData = $3, commonName = $4 WHERE id = $5",
			validTo, issuer, rawData, commonName, d.ID)
		if err != nil {
			log.Printf("Failed to update certificate %s: %v\n", d.Domain, err)
			continue
		}

		// TSK: consider for removal
		log.Printf("Updated certificate: %s\n", d.Domain)
		time.Sleep(5 * time.Second) // to avoid rate limiting
	}
}

func sendTLSExpirationReminders() {
	db := setupDatabase()
	defer db.Close(context.Background())
	// Get all certificates
	rows, err := db.Query(context.TODO(), "SELECT * FROM crts")
	if err != nil {
		log.Printf("Failed to get certificates: %v\n", err)
		return
	}
	defer rows.Close()

	certs, err := pgx.CollectRows(rows, pgx.RowToStructByName[TLSDomain])
	if err != nil {
		log.Printf("Failed to collect certificates: %v\n", err)
		return
	}
	// Array to hold certificates needing reminders
	var needReminder []TLSDomain

	// Populate the array
	for _, d := range certs {
		currTime := time.Now().AddDate(0, 0, getConfig().DaysCertExp)

		// Check if the certificate expires within the configured reminder period
		if currTime.After(d.Expiration) {
			needReminder = append(needReminder, d)
		}
	}

	log.Printf("Sending expiration reminder for %d certificates", len(needReminder))

	var domainList string

	if len(needReminder) == 0 {
		log.Println("No domains need reminders, skipping email.")
		return
	} else {
		for _, d := range needReminder {
			// add to the list
			domainList += fmt.Sprintf("<li><a href='%s/dash/tls/?q=%s'>%s</a> is expiring on %s UTC (in %s)</li>", getConfig().BaseURL, d.CommonName, d.CommonName, d.Expiration.Format("01/02/2006 @ 03:04:05PM"), humanize.Time(d.Expiration))
		}
	}

	err = sendEmail("TLS certificates soon", fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
			<head>
  				<meta charset="UTF-8">
  				<meta name="viewport" content="width=device-width, initial-scale=1.0">
  			</head>
			<body>
				<h3>The following TLS certificate(s) are expiring within the next %d days:</h3>
				<p>Click a domain to view it in TLS certificate Tracker</p>
				<ul>
					%s
				</ul>
				<br />
				<footer style='font-size: smaller;'>
					Powered by <img src='https://assets.cdn.utsav2.dev:453/bucket/domaintrk/favicon.webp' style='width: 20px; border-radius: 50%%;' />Domain Tracker for <img src='https://cbt.io/wp-content/uploads/2023/07/favicon.png' style='width: 20px;' />California Business Technology<sup>®</sup> Inc.
				</footer>
			</body>
		</html>`, getConfig().DaysCertExp, domainList))
	if err != nil {
		log.Printf("TLS: Failed to send expiration reminder email: %v\n", err)
	}
}
