package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/pedronpereira/thewishlist/webapp"
)

func main() {
	// Loads .env into the real process environment for local dev, if present
	// (see .env.example). Missing is the normal case everywhere else — Docker,
	// Azure, and Vercel (api/index.go, which never calls this) all get their
	// env vars from the platform itself — so a load failure isn't fatal.
	_ = godotenv.Load()

	e, err := webapp.New()
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("WEBSITES_PORT")
	}

	if port == "" {
		port = "43067"
	}

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}
