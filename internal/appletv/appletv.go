package appletv

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/eduard256/vast/internal/api"
	"github.com/eduard256/vast/pkg/appletv"
)

var (
	client    *appletv.Client
	streamTpl string
)

func Init() {
	host := os.Getenv("MQTT_HOST")
	if host == "" {
		log.Println("[appletv] MQTT_HOST not set, skipping")
		return
	}

	streamTpl = os.Getenv("STREAM_URL_TEMPLATE")
	if streamTpl == "" {
		log.Println("[appletv] STREAM_URL_TEMPLATE not set, skipping")
		return
	}

	port, _ := strconv.Atoi(os.Getenv("MQTT_PORT"))
	if port == 0 {
		port = 1883
	}

	var err error
	client, err = appletv.Dial(appletv.Config{
		Host:     host,
		Port:     port,
		User:     os.Getenv("MQTT_USER"),
		Password: os.Getenv("MQTT_PASSWORD"),
		Topic:    env("MQTT_TOPIC", "appletv"),
	})
	if err != nil {
		log.Printf("[appletv] mqtt connect failed: %v", err)
		return
	}

	api.HandleFunc("api/appletv/play/", apiPlay)
}

// POST /api/appletv/play/{id}
func apiPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	if client == nil {
		api.Error(w, fmt.Errorf("appletv not connected"), http.StatusServiceUnavailable)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/appletv/play/")
	if id == "" {
		api.Error(w, fmt.Errorf("id required"), http.StatusBadRequest)
		return
	}

	url := strings.Replace(streamTpl, "{id}", id, 1)

	if err := client.Play(url); err != nil {
		api.Error(w, err, http.StatusInternalServerError)
		return
	}

	api.Response(w, map[string]string{"status": "playing", "stream": url})
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
