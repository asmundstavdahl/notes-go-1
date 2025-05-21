package main

import (
	"log"
	"net/http"
	"os"
)

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
