package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/eduard256/vast/internal/api"
	"github.com/eduard256/vast/internal/appletv"
	"github.com/eduard256/vast/internal/download"
	"github.com/eduard256/vast/internal/kinopoisk"
	"github.com/eduard256/vast/internal/media"
	"github.com/eduard256/vast/internal/torrentsearch"
	"github.com/eduard256/vast/pkg/db"
	"github.com/joho/godotenv"
)

//go:embed web/*
var webFiles embed.FS

func main() {
	_ = godotenv.Load()

	// Register embedded web files before api.Init so the file server
	// acts as a fallback after all API routes are registered.
	webSub, _ := fs.Sub(webFiles, "web")
	api.SetWebFS(webSub)

	modules := []struct {
		name string
		init func()
	}{
		{"db", db.Init},
		{"api", api.Init},
		{"download", download.Init},
		{"media", media.Init},
		{"kinopoisk", kinopoisk.Init},
		{"torrentsearch", torrentsearch.Init},
		{"appletv", appletv.Init},
	}

	for _, m := range modules {
		log.Printf("[vast] init %s", m.name)
		m.init()
	}

	log.Printf("[vast] started")

	select {}
}
