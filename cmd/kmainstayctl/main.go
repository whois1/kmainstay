package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"kmainstay/internal/app"
	"kmainstay/internal/database"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "bootstrap" {
		fmt.Fprintln(os.Stderr, "usage: kmainstayctl bootstrap (reads DB_PATH, BOOTSTRAP_EMAIL, BOOTSTRAP_NAME, BOOTSTRAP_ORGANISATION; password from stdin)")
		os.Exit(2)
	}
	flag.CommandLine.Parse(nil)
	db, err := database.Open(os.Getenv("DB_PATH"))
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fatal(err)
	}
	result, err := app.Bootstrap(context.Background(), db, os.Getenv("BOOTSTRAP_EMAIL"), os.Getenv("BOOTSTRAP_NAME"), strings.TrimSpace(password), os.Getenv("BOOTSTRAP_ORGANISATION"))
	if err != nil {
		fatal(err)
	}
	fmt.Printf("bootstrapped user %s organisation %s\n", result.UserID, result.OrganisationID)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
