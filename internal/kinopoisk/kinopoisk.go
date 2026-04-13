package kinopoisk

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/eduard256/vast/internal/api"
	kp "github.com/eduard256/vast/pkg/kinopoisk"
)

var client *kp.Client

func Init() {
	token := os.Getenv("KINOPOISK_TOKEN")
	if token == "" {
		log.Printf("[kinopoisk] KINOPOISK_TOKEN not set, module disabled")
		return
	}

	client = kp.NewClient(token)

	api.HandleFunc("api/kinopoisk/search", apiSearch)
	api.HandleFunc("api/kinopoisk/top250", apiTop250)
	api.HandleFunc("api/kinopoisk/popular", apiPopular)
	api.HandleFunc("api/kinopoisk/new", apiNew)
	api.HandleFunc("api/kinopoisk/movie/", apiMovie)
}

func apiSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		api.Error(w, errors.New("query required"), http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	resp, err := client.SearchMovies(query, page, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	api.Response(w, resp)
}

func apiMovie(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/kinopoisk/movie/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		api.Error(w, errors.New("invalid id"), http.StatusBadRequest)
		return
	}

	m, err := client.GetMovie(id)
	if err != nil {
		writeError(w, err)
		return
	}
	api.Response(w, m)
}

func apiTop250(w http.ResponseWriter, r *http.Request) {
	params := url.Values{
		"lists":         {"top250"},
		"sortField":     {"top250"},
		"sortType":      {"1"},
		"limit":         {"30"},
		"selectFields":  defaultSelectFields(),
		"notNullFields": {"poster.url", "backdrop.url"},
	}
	resp, err := client.QueryMovies(params)
	if err != nil {
		writeError(w, err)
		return
	}
	api.Response(w, resp)
}

func apiPopular(w http.ResponseWriter, r *http.Request) {
	params := url.Values{
		"sortField":     {"votes.kp"},
		"sortType":      {"-1"},
		"limit":         {"20"},
		"votes.kp":      {"50000-6666666"},
		"selectFields":  defaultSelectFields(),
		"notNullFields": {"poster.url"},
	}
	resp, err := client.QueryMovies(params)
	if err != nil {
		writeError(w, err)
		return
	}
	api.Response(w, resp)
}

func apiNew(w http.ResponseWriter, r *http.Request) {
	params := url.Values{
		"year":          {"2025-2026"},
		"sortField":     {"year"},
		"sortType":      {"-1"},
		"limit":         {"20"},
		"votes.kp":      {"1000-6666666"},
		"selectFields":  defaultSelectFields(),
		"notNullFields": {"poster.url", "name"},
	}
	resp, err := client.QueryMovies(params)
	if err != nil {
		writeError(w, err)
		return
	}
	api.Response(w, resp)
}

// internals

func defaultSelectFields() []string {
	return []string{
		"id", "name", "alternativeName", "year", "rating",
		"poster", "backdrop", "genres", "shortDescription",
		"movieLength", "type", "top250",
	}
}

func writeError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "403") {
		api.Error(w, err, http.StatusForbidden)
	} else if strings.Contains(msg, "404") {
		api.Error(w, err, http.StatusNotFound)
	} else {
		api.Error(w, err, http.StatusBadGateway)
	}
}
