package main

import (
	"log"

	"github.com/eduard256/vast/internal/api"
	"github.com/eduard256/vast/internal/chat"
	"github.com/eduard256/vast/internal/download"
	"github.com/eduard256/vast/internal/media"
	"github.com/eduard256/vast/pkg/db"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	modules := []struct {
		name string
		init func()
	}{
		{"db", db.Init},
		{"api", api.Init},
		{"chat", chat.Init},
		{"download", download.Init},
		{"media", media.Init},
	}

	for _, m := range modules {
		log.Printf("[vast] init %s", m.name)
		m.init()
	}

	log.Printf("[vast] started")

	select {}
}
