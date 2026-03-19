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
	// Delete expired sessions
	delSess, err := db.Exec(context.TODO(), "DELETE FROM sessions WHERE expires < $1", time.Now())
	if err != nil {
		log.Printf("Failed to delete expired sessions: %v\n", err)
	}
	log.Printf("Deleted %d expired session(s)\n", delSess.RowsAffected())
}

func updateDomains(progress chan<- string) {
	send := func(msg string) {
		log.Println(msg)
		if progress != nil {
			select {
			case progress <- msg:
			default:
			}
		}
	}

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

	send(fmt.Sprintf("Checking %d domain(s) for updates...", len(domains)))
	refreshed := 0

	// Iterate over each domain and check expiration
	for _, d := range domains {
		currTime := time.Now().AddDate(0, 0, getConfig().DaysDomainExp)

		// Check if the domain expires within the configured reminder period
		// If it does, refresh its data
		if currTime.After(d.Expiration) {
			send(fmt.Sprintf("Updating %s...", d.Domain))
			exp, ns, reg, rawData, dns, err := fetchDomainData(d.Domain)
			if err != nil {
				send(fmt.Sprintf("Failed to fetch data for %s: %v", d.Domain, err))
				continue
			}

			_, err = db.Exec(context.TODO(), "UPDATE domains SET expiration = $1, nameservers = $2, registrar = $3, rawWhoisData = $4, dns = $5 WHERE id = $6",
				exp, ns, reg, rawData, dns, d.ID)
			if err != nil {
				send(fmt.Sprintf("Failed to save %s: %v", d.Domain, err))
				continue
			}

			send(fmt.Sprintf("Updated %s (expires %s)", d.Domain, exp.Format("01/02/2006")))
			refreshed++
			time.Sleep(15 * time.Second) // to avoid rate limiting
		}
	}

	if refreshed == 0 {
		send("No domains needed updating")
	} else {
		send(fmt.Sprintf("Finished updating %d domain(s)", refreshed))
	}
}

func sendExpDomReminders(progress chan<- string) {
	send := func(msg string) {
		log.Println(msg)
		if progress != nil {
			select {
			case progress <- msg:
			default:
			}
		}
	}

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

	send("Checking for expiring domains...")

	var domainList string

	if len(needReminder) == 0 {
		send("No domains expiring soon, skipping email")
		return
	} else {
		send(fmt.Sprintf("%d domain(s) expiring soon:", len(needReminder)))
		warningThreshold := (time.Duration(getConfig().DaysDomainExp) * 24 * time.Hour) / 2
		critThreshold := (time.Duration(getConfig().DaysDomainExp) * 24 * time.Hour) / 3

		for _, d := range needReminder {
			send(fmt.Sprintf("  • %s (expires %s)", d.Domain, d.Expiration.Format("01/02/2006")))

			var borderColor, badge string
			if time.Until(d.Expiration) < critThreshold {
				borderColor = "#f85149"
				badge = "⚠ Critical — " + humanize.Time(d.Expiration)
			} else if time.Until(d.Expiration) < warningThreshold {
				borderColor = "#e3b341"
				badge = "Warning — " + humanize.Time(d.Expiration)
			} else {
				borderColor = "#29a8e1"
				badge = humanize.Time(d.Expiration)
			}

			// Get client name
			var client string
			err = db.QueryRow(context.TODO(), "SELECT name FROM clients WHERE id = $1", d.ClientID).Scan(&client)
			if err != nil {
				log.Println("failed to get client", err)
				client = "Unknown"
			}

			subtitle := fmt.Sprintf("Expires %s &middot; Client: %s &middot; Registrar: %s",
				d.Expiration.Format("01/02/2006"), client, d.Registrar)
			domainList += domainCard(borderColor, getConfig().BaseURL+"/dash/?q="+d.Domain, d.Domain, subtitle, badge)
		}
	}

	send("Sending expiration reminder email...")
	intro := fmt.Sprintf(`<p style="margin:0 0 20px;font-size:14px;color:#57606a;">The following %d domain(s) are expiring within the next <strong>%d days</strong>. Click a domain to view it in Domain Tracker.</p>`, len(needReminder), getConfig().DaysDomainExp)
	err = sendEmail("Domains expiring soon", emailHTML("Domains expiring soon", intro+domainList))
	if err != nil {
		send(fmt.Sprintf("Failed to send email: %v", err))
	} else {
		send("Expiration reminder email sent")
	}
}

func detectNameserverChanges() {
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
		if !slices.Equal(ns, newNs) {
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

		time.Sleep(1 * time.Second) // avoid rate limiting Google DNS
	}

	if len(NSChanges) == 0 {
		log.Println("No nameserver changes detected.")
		return
	}

	// Send an alert of the changes
	var listChanges string
	for _, change := range NSChanges {
		subtitle := fmt.Sprintf("Detected %s", change.CheckedAt.Format("01/02/2006 @ 03:04:05PM"))
		nsDetails := fmt.Sprintf(
			`<table cellpadding="0" cellspacing="0" style="margin-top:8px;font-size:13px;color:#57606a;">
			<tr><td style="padding-right:12px;white-space:nowrap;color:#6e7681;">Old NS</td><td>%s</td></tr>
			<tr><td style="padding-right:12px;white-space:nowrap;color:#6e7681;">New NS</td><td style="color:#29a8e1;">%s</td></tr>
			</table>`,
			strings.Join(change.OldNS, ", "),
			strings.Join(change.NewNS, ", "),
		)
		listChanges += domainCard("#e3b341", getConfig().BaseURL+"/dash/?q="+change.Domain, change.Domain, subtitle, "") + nsDetails
	}

	intro := fmt.Sprintf(`<p style="margin:0 0 20px;font-size:14px;color:#57606a;">Nameserver changes were detected for <strong>%d domain(s)</strong>. The database has been updated automatically.</p>`, len(NSChanges))
	err = sendEmail("Nameserver changes detected", emailHTML("Nameserver changes detected", intro+listChanges))
	if err != nil {
		log.Printf("Failed to send nameserver change alert email: %v\n", err)
	}
}

func updateTLSCerts() {
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

	var certList string

	if len(needReminder) == 0 {
		log.Println("No certificates need reminders, skipping email.")
		return
	} else {
		for _, d := range needReminder {
			subtitle := fmt.Sprintf("Expires %s &middot; Authority: %s",
				d.Expiration.Format("01/02/2006"), d.Authority)
			certList += domainCard("#29a8e1", getConfig().BaseURL+"/dash/tls/?q="+d.CommonName, d.CommonName, subtitle, humanize.Time(d.Expiration))
		}
	}

	intro := fmt.Sprintf(`<p style="margin:0 0 20px;font-size:14px;color:#57606a;">The following %d TLS certificate(s) are expiring within the next <strong>%d days</strong>. Click a certificate to view it in the TLS tracker.</p>`, len(needReminder), getConfig().DaysCertExp)
	err = sendEmail("TLS certificates expiring soon", emailHTML("TLS certificates expiring soon", intro+certList))
	if err != nil {
		log.Printf("TLS: Failed to send expiration reminder email: %v\n", err)
	}
}
