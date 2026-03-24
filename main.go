package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/app"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
)

func main() {
	application, err := app.New(config.Load())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := application.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
