package media

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eduard256/vast/internal/api"
	"github.com/eduard256/vast/pkg/db"
)

var dataDir string

func Init() {
	dataDir = os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}

	api.HandleFunc("api/media", apiMedia)
	api.HandleFunc("api/media/count", apiMediaCount)
	api.HandleFunc("api/media/", apiMediaAction)
	api.HandleFunc("stream/", apiStream)
}

// serves /stream/{id}/master.m3u8 and /stream/{id}/seg_0001.ts
func apiStream(w http.ResponseWriter, r *http.Request) {
	// path: stream/{id}/filename
	path := strings.TrimPrefix(r.URL.Path, "/stream/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	filename := parts[1]
	filePath := filepath.Join(dataDir, "hls", strconv.Itoa(id), filename)

	if strings.HasSuffix(filename, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	} else if strings.HasSuffix(filename, ".ts") {
		w.Header().Set("Content-Type", "video/mp2t")
	} else if strings.HasSuffix(filename, ".m4s") {
		w.Header().Set("Content-Type", "video/iso.segment")
	} else if strings.HasSuffix(filename, ".mp4") {
		w.Header().Set("Content-Type", "video/mp4")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")

	http.ServeFile(w, r, filePath)
}

func apiMediaCount(w http.ResponseWriter, r *http.Request) {
	var movies, series int
	db.Conn().QueryRow(`SELECT COUNT(*) FROM media WHERE type = 'movie'`).Scan(&movies)
	db.Conn().QueryRow(`SELECT COUNT(*) FROM media WHERE type = 'series'`).Scan(&series)
	api.Response(w, map[string]int{"movies": movies, "series": series, "total": movies + series})
}

type Media struct {
	ID             int      `json:"id"`
	Title          string   `json:"title"`
	AltName        *string  `json:"alt_name"`
	Description    *string  `json:"description"`
	PosterURL      *string  `json:"poster_url"`
	BackdropURL    *string  `json:"backdrop_url"`
	Year           *int     `json:"year"`
	KinopoiskID    *string  `json:"kinopoisk_id"`
	TmdbID         *string  `json:"tmdb_id"`
	RatingKP       *float64 `json:"rating_kp"`
	RatingIMDB     *float64 `json:"rating_imdb"`
	Genres         *string  `json:"genres"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	HLSPath        *string  `json:"hls_path"`
	TorrentTitle   *string  `json:"torrent_title"`
	TorrentSize    *string  `json:"torrent_size"`
	TorrentTracker *string  `json:"torrent_tracker"`
	TorrentSeeders *int     `json:"torrent_seeders"`
	CreatedAt      string   `json:"created_at"`
}

type Episode struct {
	ID       int     `json:"id"`
	MediaID  int     `json:"media_id"`
	Season   int     `json:"season"`
	Episode  int     `json:"episode"`
	Title    *string `json:"title"`
	HLSPath  *string `json:"hls_path"`
	Status   string  `json:"status"`
}

type WatchPosition struct {
	MediaID     int     `json:"media_id"`
	EpisodeID   int     `json:"episode_id"`
	PositionSec float64 `json:"position_sec"`
	DurationSec float64 `json:"duration_sec"`
}

func apiMedia(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		rows, err := db.Conn().Query(`SELECT id, title, alt_name, description, poster_url, backdrop_url, year, kinopoisk_id, tmdb_id, rating_kp, rating_imdb, genres, type, status, hls_path, torrent_title, torrent_size, torrent_tracker, torrent_seeders, created_at FROM media ORDER BY created_at DESC`)
		if err != nil {
			api.Error(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Media
		for rows.Next() {
			var m Media
			rows.Scan(&m.ID, &m.Title, &m.AltName, &m.Description, &m.PosterURL, &m.BackdropURL, &m.Year, &m.KinopoiskID, &m.TmdbID, &m.RatingKP, &m.RatingIMDB, &m.Genres, &m.Type, &m.Status, &m.HLSPath, &m.TorrentTitle, &m.TorrentSize, &m.TorrentTracker, &m.TorrentSeeders, &m.CreatedAt)
			list = append(list, m)
		}
		if list == nil {
			list = []Media{}
		}
		api.Response(w, list)

	case "PUT":
		// update media metadata (AI uses this)
		var req struct {
			ID          int     `json:"id"`
			Title       *string `json:"title"`
			Description *string `json:"description"`
			PosterURL   *string `json:"poster_url"`
			Year        *int    `json:"year"`
			TmdbID      *string `json:"tmdb_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.Error(w, err, http.StatusBadRequest)
			return
		}
		if req.ID == 0 {
			api.Error(w, errors.New("id required"), http.StatusBadRequest)
			return
		}
		if req.Title != nil {
			db.Conn().Exec(`UPDATE media SET title = ? WHERE id = ?`, *req.Title, req.ID)
		}
		if req.Description != nil {
			db.Conn().Exec(`UPDATE media SET description = ? WHERE id = ?`, *req.Description, req.ID)
		}
		if req.PosterURL != nil {
			db.Conn().Exec(`UPDATE media SET poster_url = ? WHERE id = ?`, *req.PosterURL, req.ID)
		}
		if req.Year != nil {
			db.Conn().Exec(`UPDATE media SET year = ? WHERE id = ?`, *req.Year, req.ID)
		}
		if req.TmdbID != nil {
			db.Conn().Exec(`UPDATE media SET tmdb_id = ? WHERE id = ?`, *req.TmdbID, req.ID)
		}
		api.Response(w, map[string]string{"status": "ok"})

	default:
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
	}
}

// routes: /api/media/{id}, /api/media/{id}/episodes, /api/media/{id}/position
func apiMediaAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.SplitN(path, "/", 2)

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		api.Error(w, errors.New("invalid id"), http.StatusBadRequest)
		return
	}

	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		apiMediaByID(w, r, id)
	case "episodes":
		apiEpisodes(w, r, id)
	case "position":
		apiPosition(w, r, id)
	default:
		api.Error(w, errors.New("not found"), http.StatusNotFound)
	}
}

func apiMediaByID(w http.ResponseWriter, r *http.Request, id int) {
	var m Media
	err := db.Conn().QueryRow(
		`SELECT id, title, alt_name, description, poster_url, backdrop_url, year, kinopoisk_id, tmdb_id, rating_kp, rating_imdb, genres, type, status, hls_path, torrent_title, torrent_size, torrent_tracker, torrent_seeders, created_at FROM media WHERE id = ?`, id,
	).Scan(&m.ID, &m.Title, &m.AltName, &m.Description, &m.PosterURL, &m.BackdropURL, &m.Year, &m.KinopoiskID, &m.TmdbID, &m.RatingKP, &m.RatingIMDB, &m.Genres, &m.Type, &m.Status, &m.HLSPath, &m.TorrentTitle, &m.TorrentSize, &m.TorrentTracker, &m.TorrentSeeders, &m.CreatedAt)
	if err == sql.ErrNoRows {
		api.Error(w, errors.New("not found"), http.StatusNotFound)
		return
	}
	api.Response(w, m)
}

func apiEpisodes(w http.ResponseWriter, r *http.Request, mediaID int) {
	switch r.Method {
	case "GET":
		rows, err := db.Conn().Query(
			`SELECT id, media_id, season, episode, title, hls_path, status FROM episodes WHERE media_id = ? ORDER BY season, episode`, mediaID,
		)
		if err != nil {
			api.Error(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Episode
		for rows.Next() {
			var e Episode
			rows.Scan(&e.ID, &e.MediaID, &e.Season, &e.Episode, &e.Title, &e.HLSPath, &e.Status)
			list = append(list, e)
		}
		if list == nil {
			list = []Episode{}
		}
		api.Response(w, list)

	case "POST":
		var req struct {
			Season  int    `json:"season"`
			Episode int    `json:"episode"`
			Title   string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.Error(w, err, http.StatusBadRequest)
			return
		}
		result, err := db.Conn().Exec(
			`INSERT INTO episodes (media_id, season, episode, title) VALUES (?, ?, ?, ?)`,
			mediaID, req.Season, req.Episode, req.Title,
		)
		if err != nil {
			api.Error(w, err, http.StatusInternalServerError)
			return
		}
		epID, _ := result.LastInsertId()
		api.Response(w, map[string]any{"status": "ok", "id": epID})

	default:
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
	}
}

func apiPosition(w http.ResponseWriter, r *http.Request, mediaID int) {
	switch r.Method {
	case "GET":
		episodeID := r.URL.Query().Get("episode_id")
		epID := 0
		if episodeID != "" {
			epID, _ = strconv.Atoi(episodeID)
		}
		var pos WatchPosition
		err := db.Conn().QueryRow(
			`SELECT media_id, episode_id, position_sec, duration_sec FROM watch_position WHERE media_id = ? AND episode_id = ?`, mediaID, epID,
		).Scan(&pos.MediaID, &pos.EpisodeID, &pos.PositionSec, &pos.DurationSec)
		if err == sql.ErrNoRows {
			api.Response(w, WatchPosition{MediaID: mediaID, PositionSec: 0, DurationSec: 0})
			return
		}
		api.Response(w, pos)

	case "POST":
		var req struct {
			EpisodeID   int     `json:"episode_id"`
			PositionSec float64 `json:"position_sec"`
			DurationSec float64 `json:"duration_sec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.Error(w, err, http.StatusBadRequest)
			return
		}
		db.Conn().Exec(
			`INSERT INTO watch_position (media_id, episode_id, position_sec, duration_sec, updated_at)
			 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(media_id, episode_id) DO UPDATE SET position_sec = ?, duration_sec = ?, updated_at = CURRENT_TIMESTAMP`,
			mediaID, req.EpisodeID, req.PositionSec, req.DurationSec, req.PositionSec, req.DurationSec,
		)
		api.Response(w, map[string]string{"status": "ok"})

	default:
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
	}
}
