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
		type TEXT NOT NULL DEFAULT 'movie',
		status TEXT NOT NULL DEFAULT 'downloading',
		magnet TEXT,
		torrent_hash TEXT,
		file_path TEXT,
		hls_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
}

func Conn() *sql.DB { return conn }
