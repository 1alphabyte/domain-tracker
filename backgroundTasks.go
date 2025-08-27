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

	err = sendEmail("Domains expiring soon", fmt.Sprintf(`
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
	if err != nil {
		log.Printf("Failed to send expiration reminder email: %v\n", err)
	}
}

func detectNameserverChanges() {
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

	// Array to store changes
	var NSChanges []NSChange
	for _, d := range domains {
		ns := d.Nameservers
		if len(ns) == 0 {
			continue
		}
		// Fetch new nameserver
		newNs := ResolveDNS(d.Domain, "NS")

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
				<h3>The following domains' nameserver have changed:</h3>
				<ul>
					%s
				</ul>
				<br />
				<footer style='font-size: smaller;'>
					Powered by <img src='https://assets.cdn.utsav2.dev:453/bucket/domaintrk/favicon.webp' style='width: 20px; border-radius: 50%%;' />Domain Tracker for <img src='https://cbt.io/wp-content/uploads/2023/07/favicon.png' style='width: 20px;' />California Business Technology<sup>®</sup> Inc.
				</footer>
			`, listChanges))
	if err != nil {
		log.Printf("Failed to send nameserver change alert email: %v\n", err)
	}
}
