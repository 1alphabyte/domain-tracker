package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v5/pgtype"
)

func getExpiringByClient() map[int][]Domain {
	// Get all domains
	domains, err := getDomains()
	if err != nil {
		log.Println("Failed to get domains", err)
		return nil
	}

	expiringDomains := make(map[int][]Domain)
	config := getConfig()

	for _, domain := range domains {
		currTime := time.Now().AddDate(0, 0, config.DaysDomainExp)

		if currTime.After(domain.Expiration) {
			expiringDomains[domain.ClientID] = append(expiringDomains[domain.ClientID], domain)
		}
	}

	return expiringDomains
}

func sendEmailToClient(domains map[int][]Domain, progress chan<- string) {
	send := func(msg string) {
		log.Println(msg)
		if progress != nil {
			select {
			case progress <- msg:
			default:
			}
		}
	}

	config := getConfig()
	for clientID, d := range domains {
		var client, techEmail, purchaseEmail string
		err := db.QueryRow(context.TODO(), "SELECT name, techEmail, purchaseEmail FROM clients WHERE id = $1", clientID).Scan(&client, &techEmail, &purchaseEmail)
		if err != nil {
			send("failed to get client: " + client + " - " + err.Error())
			continue
		}
		if techEmail == "" && purchaseEmail == "" {
			send("Client has no email: " + client)
			continue
		}
		var domainList strings.Builder
		var pbWarn string

		toEmailStr := techEmail + "," + purchaseEmail
		if techEmail == "" || purchaseEmail == "" {
			if techEmail == "" {
				toEmailStr = purchaseEmail
			} else if purchaseEmail == "" {
				toEmailStr = techEmail
			}
		}

		for _, domain := range d {
			dbRes, err := db.Exec(context.TODO(), `
				INSERT INTO reminder_deliveries
					(domain_id,
					reminder_type,
					expiration_date,
					recipient,
					status)
				VALUES
					($1,
					'expiration',
					$2,
					$3,
					'pending')
				ON CONFLICT (domain_id, reminder_type, expiration_date) DO NOTHING`,
				domain.ID, domain.Expiration, toEmailStr)
			if err != nil {
				send("Failed to insert reminder deliveries: " + err.Error())
				continue
			}
			if dbRes.RowsAffected() == 0 {
				var status string
				var sentAt pgtype.Timestamptz
				err = db.QueryRow(context.TODO(), "SELECT status, sent_at FROM reminder_deliveries WHERE domain_id = $1", domain.ID).Scan(&status, &sentAt)
				if err != nil {
					send("Failed to get reminder deliveries: " + err.Error())
					continue
				}
				if status == "sent" && !sentAt.Time.IsZero() && sentAt.Valid {
					send("Reminder already sent for this domain: " + domain.Domain)
					domain.ID = -1
					continue
				}
			}
			domainList.WriteString(domainCard(
				"#e3b341",
				"https://"+domain.Domain+"/",
				domain.Domain,
				fmt.Sprintf("Expires %s &middot; Client: %s &middot; Registrar: %s",
					domain.Expiration.Format("01/02/2006"), client, domain.Registrar),
				humanize.Time(domain.Expiration)))
			if domain.Registrar != "Porkbun LLC" {
				pbWarn = `<p style="margin:0 0 20px;font-size:14px;color:#57606a;">
					Additionally, it looks like some or all of your domain(s) aren't with our partner registrar, <a href="https://porkbun.com">Porkbun</a>. We recommend transferring your domains to Porkbun to ensure that you can take full advantage of our domain services. Please indicate in your reply if you would like to transfer your domains to Porkbun.
				</p>`
			}
		}
		var introI string
		if len(d) == 1 {
			introI = "The following domain is"
		} else {
			introI = fmt.Sprintf("The following %d domains are", len(d))
		}

		intro := fmt.Sprintf(`
			<p style="margin:0 0 20px;font-size:14px;color:#57606a;">
				Dear %s,
				%s expiring within the next <strong>%d days</strong>.
				Please reply back to this email to either approve the renewal of these domains or to let us know if you no longer wish to keep them.
			</p>
			%s`,
			client,
			introI,
			config.DaysDomainExp,
			pbWarn)

		if domainList.Len() == 0 {
			send("No domains to send email for client: " + client)
			continue
		}

		send(fmt.Sprintf("Sending email to client: %s, domains: %d. To: %s", client, len(d), toEmailStr))

		err = sendEmail("Your domains are expiring soon", emailHTML("Your domains are expiring soon", intro+domainList.String()), toEmailStr, config.ReplyEmail)
		if err != nil {
			send(fmt.Sprintf("Failed to send email to client: %s", err.Error()))
			for _, domain := range d {
				_, err = db.Exec(context.TODO(), "UPDATE reminder_deliveries SET sent_at = null, status = 'error' WHERE domain_id = $1", domain.ID)
				if err != nil {
					log.Println("Failed to update reminder_deliveries", err)
					continue
				}
			}
		} else {
			for _, domain := range d {
				if domain.ID == -1 {
					continue
				}
				_, err = db.Exec(context.TODO(), "UPDATE reminder_deliveries SET sent_at = now(), status = 'sent' WHERE domain_id = $1", domain.ID)
				if err != nil {
					log.Println("Failed to update reminder_deliveries", err)
					continue
				}
			}
		}
	}
}
