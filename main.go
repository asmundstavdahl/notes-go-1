package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

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

func main() {
	initTemplates()
	initDB()

	// Define HTTP routes
	http.HandleFunc("/", listNotesHandler)              // Handles listing notes and the creation form
	http.HandleFunc("/notes/create", createNoteHandler) // Handles submission of the new note form
	http.HandleFunc("/notes/edit/", editNoteHandler)    // Handles editing of an existing note
	http.HandleFunc("/notes/", viewNoteHandler)         // Handles viewing a single note (e.g., /notes/12345)
	http.HandleFunc("/keywords", listKeywordsHandler)   // List all available keywords and filter notes by keyword
	http.HandleFunc("/keyword/", notesByKeywordHandler) // Handles viewing all notes for a given keyword (/keyword/{keyword})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	log.Printf("Server starting on http://localhost:%s", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
