package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

func getExpiringByClient() map[int][]Domain {
	// Get all domains
	domains, err := getDomains()
	if err != nil {
		log.Println("Failed to get domains")
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

func sendEmailToClient(domains map[int][]Domain) {
	config := getConfig()
	for clientID, d := range domains {
		var client, techEmail, purchaseEmail string
		var reminderSent bool
		err := db.QueryRow(context.TODO(), "SELECT name, techEmail, purchaseEmail, reminderSent FROM clients WHERE id = $1", clientID).Scan(&client, &techEmail, &purchaseEmail, &reminderSent)
		if err != nil {
			log.Println("failed to get client", err)
			return
		}
		if techEmail == "" && purchaseEmail == "" {
			log.Println("Client has no email")
			return
		}
		if reminderSent {
			log.Println("Client has already been sent a reminder")
			return
		}
		var domainList strings.Builder
		var pbWarn string

		for _, domain := range d {
			domainList.WriteString(domainCard(
				"#e3b341",
				"https://cbt.io/?d="+domain.Domain,
				domain.Domain,
				fmt.Sprintf("Expires %s &middot; Client: %s &middot; Registrar: %s",
					domain.Expiration.Format("01/02/2006"), client, domain.Registrar),
				humanize.Time(domain.Expiration)))
			if domain.Registrar != "Porkbun LLC" {
				pbWarn = `<p style="margin:0 0 20px;font-size:14px;color:#57606a;">
					Additionally, it looks like some or all of your domain aren't with our partner registrar, Porkbun. If you're okay with it, we recommend transferring your domains to Porkbun to ensure that you can take full advantage of our domain services. Please indicate in your reply if you would like to transfer your domains to Porkbun.
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
				Please reply back to this email to either approve the renewal of this domain or to let us know if you no longer wish to keep it.
			</p>
			%s`,
			client,
			introI,
			config.DaysDomainExp,
			pbWarn)

		err = sendEmail("Your domains require action", emailHTML("Domains expiring soon", intro+domainList.String()), techEmail+","+purchaseEmail, config.ReplyEmail)
		if err != nil {
			log.Println("Failed to send email to client", err)
		}
		_, err = db.Exec(context.TODO(), "UPDATE clients SET reminderSent = TRUE WHERE id = $1", clientID)
		if err != nil {
			log.Println("Failed to update client reminderSent", err)
		}
	}
}
