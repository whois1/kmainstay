package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"kmainstay/internal/database"
	"kmainstay/internal/httpapi"
	"kmainstay/internal/webui"
)

func main() {
	databasePath := os.Getenv("DB_PATH")
	if databasePath == "" {
		databasePath = "kmainstay.db"
	}
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}
	db, err := database.Open(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.New(httpapi.Dependencies{DB: db, SecureCookies: os.Getenv("INSECURE_COOKIES") != "1", Assets: webui.Handler()}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	fmt.Printf("listening on %s\n", address)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
