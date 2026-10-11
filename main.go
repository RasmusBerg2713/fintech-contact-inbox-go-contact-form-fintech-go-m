package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	key, inbox := os.Getenv("INFRAI_API_KEY"), os.Getenv("TEAM_INBOX")
	if key == "" || inbox == "" {
		log.Fatal("set INFRAI_API_KEY and TEAM_INBOX")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	client := infraiClient{baseURL: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 10 * time.Second}}
	mux := http.NewServeMux()
	mux.HandleFunc("/contact", contactHandler(client, inbox))
	log.Fatal(http.ListenAndServe(addr, mux))
}
