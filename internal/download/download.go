package download

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/eduard256/vast/internal/api"
	"github.com/eduard256/vast/pkg/db"
	"github.com/eduard256/vast/pkg/hls"
	"github.com/eduard256/vast/pkg/torrent"
)

var dataDir string

func Init() {
	dataDir = os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}

	if err := torrent.Init(dataDir); err != nil {
		log.Printf("[download] torrent init error: %v", err)
		return
	}

	api.HandleFunc("api/download", apiDownload)
	api.HandleFunc("api/downloads", apiDownloads)
	api.HandleFunc("api/downloads/", apiDownloadAction)
}

func apiDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Title          string  `json:"title"`
		AltName        string  `json:"alt_name"`
		Description    string  `json:"description"`
		PosterURL      string  `json:"poster_url"`
		BackdropURL    string  `json:"backdrop_url"`
		Year           int     `json:"year"`
		KinopoiskID    string  `json:"kinopoisk_id"`
		TmdbID         string  `json:"tmdb_id"`
		RatingKP       float64 `json:"rating_kp"`
		RatingIMDB     float64 `json:"rating_imdb"`
		Genres         string  `json:"genres"`
		Magnet         string  `json:"magnet"`
		Type           string  `json:"type"`
		TorrentTitle   string  `json:"torrent_title"`
		TorrentSize    string  `json:"torrent_size"`
		TorrentTracker string  `json:"torrent_tracker"`
		TorrentSeeders int     `json:"torrent_seeders"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.Magnet == "" || req.Title == "" {
		api.Error(w, errors.New("title and magnet required"), http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		req.Type = "movie"
	}

	result, err := db.Conn().Exec(
		`INSERT INTO media (title, alt_name, description, poster_url, backdrop_url, year, kinopoisk_id, tmdb_id, rating_kp, rating_imdb, genres, type, status, magnet, torrent_title, torrent_size, torrent_tracker, torrent_seeders)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'downloading', ?, ?, ?, ?, ?)`,
		req.Title, req.AltName, req.Description, req.PosterURL, req.BackdropURL,
		req.Year, req.KinopoiskID, req.TmdbID, req.RatingKP, req.RatingIMDB,
		req.Genres, req.Type, req.Magnet,
		req.TorrentTitle, req.TorrentSize, req.TorrentTracker, req.TorrentSeeders,
	)
	if err != nil {
		api.Error(w, err, http.StatusInternalServerError)
		return
	}

	mediaID64, _ := result.LastInsertId()
	mediaID := int(mediaID64)

	go func() {
		t, err := torrent.Add(req.Magnet)
		if err != nil {
			log.Printf("[download] torrent add error: %v", err)
			db.Conn().Exec(`UPDATE media SET status = 'error' WHERE id = ?`, mediaID)
			return
		}

		hash := t.InfoHash().HexString()
		db.Conn().Exec(`UPDATE media SET torrent_hash = ? WHERE id = ?`, hash, mediaID)
		torrent.Track(hash, mediaID, t)

		go waitComplete(hash, mediaID)
	}()

	api.Response(w, map[string]any{"status": "ok", "id": mediaID})
}

func apiDownloads(w http.ResponseWriter, r *http.Request) {
	downloads := torrent.List()

	type DownloadStatus struct {
		torrent.Progress
		AudioPercent *float64 `json:"audio_percent"`
		VideoPercent *float64 `json:"video_percent"`
	}

	var out []DownloadStatus
	for _, dl := range downloads {
		ds := DownloadStatus{Progress: dl}
		if dl.Status == "transcoding" {
			if job := hls.GetJob(dl.ID); job != nil {
				a, v := job.AudioPercent, job.VideoPercent
				ds.AudioPercent = &a
				ds.VideoPercent = &v
			}
		}
		out = append(out, ds)
	}

	if out == nil {
		api.Response(w, []DownloadStatus{})
		return
	}
	api.Response(w, out)
}

func apiDownloadAction(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, "/api/downloads/")
	if hash == "" {
		api.Error(w, errors.New("hash required"), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "DELETE":
		dl := torrent.Get(hash)
		if dl == nil {
			api.Error(w, errors.New("not found"), http.StatusNotFound)
			return
		}
		mediaID := dl.ID

		// stop transcoding if running
		hls.Cancel(mediaID)

		// stop torrent
		torrent.Remove(hash)

		// delete downloaded files
		os.RemoveAll(filepath.Join(dataDir, "downloads", hash))

		// delete HLS files
		os.RemoveAll(hls.HLSDir(dataDir, mediaID))

		// delete from database
		db.Conn().Exec(`DELETE FROM watch_position WHERE media_id = ?`, mediaID)
		db.Conn().Exec(`DELETE FROM episodes WHERE media_id = ?`, mediaID)
		db.Conn().Exec(`DELETE FROM media WHERE id = ?`, mediaID)

		api.Response(w, map[string]string{"status": "ok"})
	default:
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
	}
}

func waitComplete(hash string, mediaID int) {
	dl := torrent.Get(hash)
	if dl == nil || dl.T == nil {
		return
	}

	<-dl.T.Complete().On()

	log.Printf("[download] completed: %s", dl.T.Name())

	// find downloaded file
	inputFile := findVideoFile(filepath.Join(dataDir, "downloads", dl.T.InfoHash().HexString()))
	if inputFile == "" {
		// single file torrent
		inputFile = filepath.Join(dataDir, "downloads", dl.T.InfoHash().HexString(), dl.T.Name())
	}

	db.Conn().Exec(`UPDATE media SET status = 'transcoding', file_path = ? WHERE id = ?`, inputFile, mediaID)
	torrent.SetStatus(hash, "transcoding")

	log.Printf("[download] transcoding: %s", inputFile)

	outDir := hls.HLSDir(dataDir, mediaID)
	hls.Start(mediaID, inputFile, outDir, func(err error) {
		if err != nil {
			log.Printf("[download] transcode error: %v", err)
			db.Conn().Exec(`UPDATE media SET status = 'error' WHERE id = ?`, mediaID)
			torrent.SetStatus(hash, "error")
			return
		}

		hlsPath := hls.PlaylistPath(dataDir, mediaID)
		log.Printf("[download] transcode done: %s", hlsPath)
		db.Conn().Exec(`UPDATE media SET status = 'ready', hls_path = ? WHERE id = ?`, hlsPath, mediaID)

		// remove torrent and source files after successful transcode
		torrent.Remove(hash)
		downloadDir := filepath.Join(dataDir, "downloads", hash)
		if err := os.RemoveAll(downloadDir); err != nil {
			log.Printf("[download] failed to remove source files %s: %v", downloadDir, err)
		} else {
			log.Printf("[download] removed source files: %s", downloadDir)
		}
	})
}

func findVideoFile(dir string) string {
	exts := []string{".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".ts"}
	var best string
	var bestSize int64

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		for _, e := range exts {
			if ext == e && info.Size() > bestSize {
				best = path
				bestSize = info.Size()
				break
			}
		}
		return nil
	})

	return best
}

// StreamURL returns the HLS stream URL for a media
func StreamURL(mediaID int) string {
	return fmt.Sprintf("/stream/%d/master.m3u8", mediaID)
}
