package appletv

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/eduard256/vast/internal/api"
)

var apiBase string

func Init() {
	apiBase = os.Getenv("APPLETV_API_URL")
	if apiBase == "" {
		return
	}

	api.HandleFunc("api/appletv/play/", apiPlay)
	api.HandleFunc("api/appletv/off/", apiOff)
}

// POST /api/appletv/play/{id}?tv=hall&position=1228
func apiPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/appletv/play/")
	tv := r.URL.Query().Get("tv")
	if id == "" || tv == "" {
		api.Error(w, fmt.Errorf("id and tv required"), http.StatusBadRequest)
		return
	}

	position := r.URL.Query().Get("position")

	var body string
	if position != "" && position != "0" {
		body = fmt.Sprintf(`{"tv":"%s","channel_id":%s,"position":%s}`, tv, id, position)
	} else {
		body = fmt.Sprintf(`{"tv":"%s","channel_id":%s}`, tv, id)
	}

	resp, err := http.Post(apiBase+"/tv/play", "application/json", strings.NewReader(body))
	if err != nil {
		api.Error(w, err, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// POST /api/appletv/off/{tv}
func apiOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	tv := strings.TrimPrefix(r.URL.Path, "/api/appletv/off/")
	if tv == "" {
		api.Error(w, fmt.Errorf("tv required"), http.StatusBadRequest)
		return
	}

	body := fmt.Sprintf(`{"tv":"%s"}`, tv)
	resp, err := http.Post(apiBase+"/tv/off", "application/json", strings.NewReader(body))
	if err != nil {
		api.Error(w, err, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
