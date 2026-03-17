package main

import (
	"log"

	"share-app-host/internal/app"
	"share-app-host/internal/config"
)

func main() {
	cfg := config.Load()
	if err := app.New(cfg).Run(); err != nil {
		log.Fatal(err)
	}
}
