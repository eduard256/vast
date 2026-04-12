package torrent

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
)

var (
	client *torrent.Client
	mu     sync.Mutex
	active = map[string]*Download{}
)

type Download struct {
	T      *torrent.Torrent
	ID     int    // media ID in database
	Status string // "downloading", "done", "error"
	Error  string
}

func Init(dataDir string) error {
	dir := filepath.Join(dataDir, "downloads")
	os.MkdirAll(dir, 0755)

	cfg := torrent.NewDefaultClientConfig()
	cfg.DefaultStorage = storage.NewFileByInfoHash(dir)
	cfg.DataDir = dir
	cfg.Seed = true

	var err error
	client, err = torrent.NewClient(cfg)
	return err
}

// Add starts downloading a magnet link, returns info hash
func Add(magnet string) (*torrent.Torrent, error) {
	t, err := client.AddMagnet(magnet)
	if err != nil {
		return nil, err
	}
	<-t.GotInfo()
	t.DownloadAll()
	return t, nil
}

// Track registers a download for status tracking
func Track(hash string, mediaID int, t *torrent.Torrent) {
	mu.Lock()
	active[hash] = &Download{T: t, ID: mediaID, Status: "downloading"}
	mu.Unlock()
}

type Progress struct {
	ID          int     `json:"id"`
	Hash        string  `json:"hash"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	Error       string  `json:"error,omitempty"`
	Total       int64   `json:"total"`
	Done        int64   `json:"done"`
	Percent     float64 `json:"percent"`
	Seeds       int     `json:"seeds"`
	DownSpeed   int64   `json:"down_speed"`
}

// List returns status of all active downloads
func List() []Progress {
	mu.Lock()
	defer mu.Unlock()

	var out []Progress
	for hash, dl := range active {
		p := Progress{
			ID:     dl.ID,
			Hash:   hash,
			Status: dl.Status,
			Error:  dl.Error,
		}

		if dl.T != nil && dl.T.Info() != nil {
			p.Name = dl.T.Name()
			p.Total = dl.T.Length()
			p.Done = dl.T.BytesCompleted()
			if p.Total > 0 {
				p.Percent = float64(p.Done) / float64(p.Total) * 100
			}

			stats := dl.T.Stats()
			p.Seeds = stats.ConnectedSeeders
			p.DownSpeed = stats.BytesReadUsefulData.Int64()
		}

		out = append(out, p)
	}
	return out
}

// Get returns a single download status
func Get(hash string) *Download {
	mu.Lock()
	defer mu.Unlock()
	return active[hash]
}

// SetStatus updates download status
func SetStatus(hash, status string) {
	mu.Lock()
	if dl, ok := active[hash]; ok {
		dl.Status = status
	}
	mu.Unlock()
}

// Remove stops and removes a download
func Remove(hash string) {
	mu.Lock()
	if dl, ok := active[hash]; ok {
		dl.T.Drop()
		delete(active, hash)
	}
	mu.Unlock()
}

func Close() {
	if client != nil {
		client.Close()
	}
}
