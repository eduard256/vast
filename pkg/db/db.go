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
		description TEXT,
		poster_url TEXT,
		year INTEGER,
		tmdb_id TEXT,
		type TEXT NOT NULL DEFAULT 'movie',
		status TEXT NOT NULL DEFAULT 'downloading',
		magnet TEXT,
		torrent_hash TEXT,
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
		episode_id INTEGER,
		position_sec REAL NOT NULL DEFAULT 0,
		duration_sec REAL NOT NULL DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(media_id, episode_id)
	)`)

	// migrate: add columns if missing (for existing databases)
	conn.Exec(`ALTER TABLE media ADD COLUMN year INTEGER`)
	conn.Exec(`ALTER TABLE media ADD COLUMN tmdb_id TEXT`)
}

func Conn() *sql.DB { return conn }
