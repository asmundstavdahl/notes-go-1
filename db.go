package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// initDB initializes the SQLite database and creates necessary tables.
func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "notes.db")
	if err != nil {
		log.Fatalf("Could not open database: %v", err)
	}

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS notes(
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL
)`,
	)
	if err != nil {
		log.Fatalf("Could not create notes table: %v", err)
	}

	// Keyword tables
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS keywords (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
)`)
	if err != nil {
		log.Fatalf("Could not create keywords table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS note_keywords (
    note_id TEXT NOT NULL,
    keyword_id INTEGER NOT NULL,
    PRIMARY KEY (note_id, keyword_id),
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
    FOREIGN KEY (keyword_id) REFERENCES keywords(id) ON DELETE CASCADE
)`)
	if err != nil {
		log.Fatalf("Could not create note_keywords table: %v", err)
	}
}
