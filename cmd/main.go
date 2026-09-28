package main

import (
	"fmt"
	"log"
	"os"

	"github.com/pedronpereira/thewishlist/webapp"
)

func main() {
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
