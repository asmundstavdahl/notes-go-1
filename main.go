package main

import (
    "log"
    "net/http"
    "os"
)

func main() {
    // Initialize templates
    initTemplates() // This function is in handlers.go

    // Load existing notes from file
    loadNotes() // This function is in handlers.go

    // Define HTTP routes
    http.HandleFunc("/", listNotesHandler)                 // Handles listing notes and the creation form
    http.HandleFunc("/notes/create", createNoteHandler) // Handles submission of the new note form
    http.HandleFunc("/notes/", viewNoteHandler)        // Handles viewing a single note (e.g., /notes/12345)

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

