package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v5"
	"github.com/wneessen/go-mail"
)

func dbCleanup() {
	db := setupDatabase()

	// Delete expired sessions
	delSess, err := db.Exec(context.TODO(), "DELETE FROM sessions WHERE expires < $1", time.Now())
	if err != nil {
		log.Printf("Failed to delete expired sessions: %v\n", err)
	}
	log.Printf("Deleted %d expired session(s)\n", delSess.RowsAffected())
}

func updateDomains() {
	db := setupDatabase()
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
		}
	}
}

func sendExpDomReminders() {
	db := setupDatabase()

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
	message := mail.NewMsg()
	if err := message.From(getConfig().FromEmail); err != nil {
		log.Print("failed to set From address:", err)
	}
	if err := message.To(getConfig().EmailForExp); err != nil {
		log.Fatal("failed to set To address:", err)
	}
	message.Subject("Domains expiring soon")
	var domainList string

	if len(needReminder) == 0 {
		log.Println("No domains need reminders, skipping email.")
		return
	} else {
		for _, d := range needReminder {
			// check if less then half the reminder period is remaining
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

	// --- Create the email message ---
	message.SetBodyString(mail.TypeTextHTML, fmt.Sprintf(`
				<h3>The following domain(s) are expiring within the next %d days:</h3>
				<p>Click a domain to view it in Domain Tracker</p>
				<ul>
					%s
				</ul>
				<br />
				<footer style='font-size: smaller;'>
					Powered by <img src='https://assets.cdn.utsav2.dev:453/bucket/domaintrk/favicon.webp' style='width: 20px; border-radius: 50%%;' />Domain Tracker for <img src='https://cbt.io/wp-content/uploads/2023/07/favicon.png' style='width: 20px;' />California Business Technology<sup>®</sup> Inc.
				</footer>
			`, getConfig().DaysDomainExp, domainList))

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
		log.Fatalf("Failed to create client: %v", err)
	}
	// Send the email
	if err := c.DialAndSend(message); err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}
}
