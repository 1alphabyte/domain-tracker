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

	ExpiringDomains := make(map[int][]Domain)

	for _, domain := range domains {
		currTime := time.Now().AddDate(0, 0, getConfig().DaysDomainExp)

		if currTime.After(domain.Expiration) {
			ExpiringDomains[domain.ClientID] = append(ExpiringDomains[domain.ClientID], domain)
		}
	}

	return ExpiringDomains
}

func sendEmailToClient(domains map[int][]Domain) {
	for clientID, d := range domains {
		var client, techEmail, purchaseEmail string
		err := db.QueryRow(context.TODO(), "SELECT name, techEmail, purchaseEmail FROM clients WHERE id = $1", clientID).Scan(&client, &techEmail, &purchaseEmail)
		if err != nil {
			log.Println("failed to get client", err)
			client = "Unknown"
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
					Additionally, it looks like some or all of your domain aren't with our partner registrar, Porkbun. If you are okay with it, we recommend transferring your domains to Porkbun to ensure that you can take advantage of our renewal service. Please reply back to this email if you would like to transfer your domains to Porkbun.
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
			getConfig().DaysDomainExp,
			pbWarn)

		err = sendEmail("Your domains require action", emailHTML("Domains expiring soon", intro+domainList.String()))
	}
}
