package torrentsearch

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/eduard256/vast/internal/api"
	"github.com/eduard256/vast/pkg/freedomist"
)

var client *freedomist.Client

func Init() {
	token := os.Getenv("EXFREEDOMIST_TOKEN")
	if token == "" {
		log.Printf("[torrentsearch] EXFREEDOMIST_TOKEN not set, module disabled")
		return
	}

	client = freedomist.NewClient(token)

	api.HandleFunc("api/torrentsearch", apiSearch)
	api.HandleFunc("api/torrentsearch/magnet", apiMagnet)
}

func apiSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	var q freedomist.SearchQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		api.Error(w, err, http.StatusBadRequest)
		return
	}

	resp, err := client.Search(q)
	if err != nil {
		api.Error(w, err, http.StatusBadGateway)
		return
	}

	api.Response(w, resp)
}

func apiMagnet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		api.Error(w, errors.New("key required"), http.StatusBadRequest)
		return
	}

	magnet, err := client.Magnet(key)
	if err != nil {
		api.Error(w, err, http.StatusBadGateway)
		return
	}

	api.Response(w, map[string]string{"magnet": magnet})
}
