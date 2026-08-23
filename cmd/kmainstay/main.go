package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"kmainstay/internal/attachments"
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
	attachmentPath := os.Getenv("ATTACHMENT_PATH")
	if attachmentPath == "" {
		attachmentPath = databasePath + ".uploads"
	}
	attachmentStore, err := attachments.NewFilesystem(attachmentPath)
	if err != nil {
		log.Fatal(err)
	}
	if removed, err := attachmentStore.Cleanup(time.Now().Add(-24 * time.Hour)); err != nil {
		log.Printf("attachment cleanup: %v", err)
	} else if removed > 0 {
		log.Printf("removed %d abandoned attachment staging files", removed)
	}

	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.New(httpapi.Dependencies{DB: db, SecureCookies: os.Getenv("INSECURE_COOKIES") != "1", Assets: webui.Handler(), Attachments: attachmentStore}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	fmt.Printf("listening on %s\n", address)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
