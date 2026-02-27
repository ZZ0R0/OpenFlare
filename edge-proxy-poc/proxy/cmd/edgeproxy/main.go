package main

import (
	"log"
	"os"

	"github.com/openflare/edge-proxy/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Printf("Fatal: %v", err)
		os.Exit(1)
	}
}
