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
}

func main() {
	initTemplates()
	initDB()

	// Define HTTP routes
	http.HandleFunc("/", listNotesHandler)              // Handles listing notes and the creation form
	http.HandleFunc("/notes/create", createNoteHandler) // Handles submission of the new note form
	http.HandleFunc("/notes/", viewNoteHandler)         // Handles viewing a single note (e.g., /notes/12345)

	// Serve static files if any (optional for PoC, but good to have a placeholder)
	// fs := http.FileServer(http.Dir("static"))
	// http.Handle("/static/", http.StripPrefix("/static/", fs))

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
