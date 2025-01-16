// unable to do this in bun due to an issue https://github.com/oven-sh/bun/issues/15731
// so temporarily relaying on Go to fetch the certificate

package main

import (
    "crypto/tls"
    "fmt"
    "log"
    "os"
    "encoding/json"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatalf("Usage: %s <server>", os.Args[0])
    }

    server := os.Args[1]

    // Connect to the server
    conn, err := tls.Dial("tcp", server + ":443", nil)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    // Get the certificate
    cert := conn.ConnectionState().PeerCertificates[0]
	fmt.Println(cert.Issuer.Organization[0],"!", cert.NotAfter, "!", cert.Subject.CommonName, "!")
    certJSON, err := json.Marshal(cert)
    if err != nil {
        log.Fatalf("Failed to marshal certificate: %v", err)
    }

    // Print the JSON representation of the certificate
    fmt.Print(string(certJSON))
}
