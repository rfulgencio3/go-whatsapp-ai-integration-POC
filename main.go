package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/app"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
)

func main() {
	application, err := app.New(config.Load())
	if err != nil {
		log.Fatal(err)
	}

	if err := application.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
