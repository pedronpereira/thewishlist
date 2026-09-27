// Package handler is Vercel's Go serverless entrypoint: any exported
// func(http.ResponseWriter, *http.Request) under api/ becomes one function.
package handler

import (
	"log"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/pedronpereira/thewishlist/internal/webapp"
)

var (
	once    sync.Once
	echoApp *echo.Echo
)

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		e, err := webapp.New()
		if err != nil {
			log.Fatalf("initializing app: %v", err)
		}
		echoApp = e
	})

	echoApp.ServeHTTP(w, r)
}
