package main

import (
	"fmt"
	"net/http"
)

func main() {
	InitDBSetup()

	// go func() {
	// 	ticker := time.NewTicker(12 * time.Hour)
	// 	for range ticker.C {
	// 		dbCleanup()
	// 	}
	// }()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		} else if r.URL.Path == "/" {
			http.Redirect(w, r, "/login/", http.StatusFound)
		}

		http.ServeFile(w, r, "./static"+r.URL.Path)
	})

	if err := http.ListenAndServe("127.0.0.1:8746", mux); err != nil {
		fmt.Println(err)
	}
}
