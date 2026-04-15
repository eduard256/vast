package lgtv

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
	apiBase = os.Getenv("LGTV_API_URL")
	if apiBase == "" {
		return
	}

	api.HandleFunc("api/lgtv/play/", apiPlay)
	api.HandleFunc("api/lgtv/off/", apiOff)
}

// POST /api/lgtv/play/{id}?tv=kitchen
func apiPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/lgtv/play/")
	tv := r.URL.Query().Get("tv")
	if id == "" || tv == "" {
		api.Error(w, fmt.Errorf("id and tv required"), http.StatusBadRequest)
		return
	}

	body := fmt.Sprintf(`{"tv":"%s","channel_id":%s}`, tv, id)
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

// POST /api/lgtv/off/{tv}
func apiOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	tv := strings.TrimPrefix(r.URL.Path, "/api/lgtv/off/")
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
