package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var conn *sql.DB

func Init() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}
	os.MkdirAll(dataDir, 0755)

	var err error
	conn, err = sql.Open("sqlite3", filepath.Join(dataDir, "vast.db")+"?_journal_mode=WAL")
	if err != nil {
		panic("db: " + err.Error())
	}

	conn.Exec(`CREATE TABLE IF NOT EXISTS media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		alt_name TEXT,
		description TEXT,
		poster_url TEXT,
		backdrop_url TEXT,
		year INTEGER,
		kinopoisk_id TEXT,
		tmdb_id TEXT,
		rating_kp REAL,
		rating_imdb REAL,
		genres TEXT,
		type TEXT NOT NULL DEFAULT 'movie',
		status TEXT NOT NULL DEFAULT 'downloading',
		magnet TEXT,
		torrent_hash TEXT,
		torrent_title TEXT,
		torrent_size TEXT,
		torrent_tracker TEXT,
		torrent_seeders INTEGER,
		file_path TEXT,
		hls_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS episodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		media_id INTEGER NOT NULL,
		season INTEGER NOT NULL,
		episode INTEGER NOT NULL,
		title TEXT,
		file_path TEXT,
		hls_path TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
	)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS watch_position (
		media_id INTEGER NOT NULL,
		episode_id INTEGER NOT NULL DEFAULT 0,
		position_sec REAL NOT NULL DEFAULT 0,
		duration_sec REAL NOT NULL DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(media_id, episode_id)
	)`)

	// migrate: add columns if missing (for existing databases)
	conn.Exec(`ALTER TABLE media ADD COLUMN alt_name TEXT`)
	conn.Exec(`ALTER TABLE media ADD COLUMN backdrop_url TEXT`)
	conn.Exec(`ALTER TABLE media ADD COLUMN kinopoisk_id TEXT`)
	conn.Exec(`ALTER TABLE media ADD COLUMN rating_kp REAL`)
	conn.Exec(`ALTER TABLE media ADD COLUMN rating_imdb REAL`)
	conn.Exec(`ALTER TABLE media ADD COLUMN genres TEXT`)
	conn.Exec(`ALTER TABLE media ADD COLUMN torrent_title TEXT`)
	conn.Exec(`ALTER TABLE media ADD COLUMN torrent_size TEXT`)
	conn.Exec(`ALTER TABLE media ADD COLUMN torrent_tracker TEXT`)
	conn.Exec(`ALTER TABLE media ADD COLUMN torrent_seeders INTEGER`)
}

func Conn() *sql.DB { return conn }
