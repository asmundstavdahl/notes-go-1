package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Note defines the structure for a note
type Note struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

var (
	db        *sql.DB
	templates *template.Template
)

// listNotesHandler handles requests to the root path and displays notes
func listNotesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, content, created_at FROM notes ORDER BY created_at DESC")
	if err != nil {
		log.Printf("Error querying notes: %v", err)
		http.Error(w, "Error fetching notes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var currentNotes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.Content, &note.CreatedAt); err != nil {
			log.Printf("Error scanning note: %v", err)
			continue
		}
		currentNotes = append(currentNotes, note)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Row iteration error: %v", err)
	}

	pageData := struct {
		Notes []Note
	}{
		Notes: currentNotes,
	}

	if err := templates.ExecuteTemplate(w, "index.html", pageData); err != nil {
		log.Printf("Error executing index template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

// createNoteHandler handles requests to create a new note
func createNoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")

	if content == "" {
		http.Error(w, "Content cannot be empty", http.StatusBadRequest)
		return
	}

	newID := strconv.FormatInt(time.Now().UnixNano(), 10)
	createdAt := time.Now()
	if _, err := db.Exec(
		"INSERT INTO notes(id, content, created_at) VALUES(?, ?, ?)",
		newID, content, createdAt,
	); err != nil {
		log.Printf("Error inserting new note: %v", err)
		http.Error(w, "Error saving note", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// viewNoteHandler handles requests to view a single note
func viewNoteHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "Note ID is missing", http.StatusBadRequest)
		return
	}
	noteID := parts[2]

	var note Note
	err := db.QueryRow(
		"SELECT id, content, created_at FROM notes WHERE id = ?",
		noteID,
	).Scan(&note.ID, &note.Content, &note.CreatedAt)

	templateData := struct {
		Note  Note
		Found bool
	}{
		Note:  note,
		Found: err == nil,
	}

	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
	} else if err != nil {
		log.Printf("Error querying note: %v", err)
		http.Error(w, "Error fetching note", http.StatusInternalServerError)
		return
	}

	if err := templates.ExecuteTemplate(w, "note.html", templateData); err != nil {
		log.Printf("Error executing note template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

func initTemplates() {
	templateDir := "templates"
	// Check if running from project root or if templates dir is directly accessible
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		// If not found, try to locate it relative to the executable's path
		// This is more robust for deployed binaries.
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Could not get executable path: %v", err)
		}
		exeDir := filepath.Dir(exePath)
		tryPath := filepath.Join(exeDir, "templates")
		if _, err := os.Stat(tryPath); os.IsNotExist(err) {
			// Fallback to checking relative to current working directory (common for `go run`)
			wd, _ := os.Getwd()
			tryPathWd := filepath.Join(wd, "templates")
			if _, err := os.Stat(tryPathWd); os.IsNotExist(err) {
				log.Fatalf("Templates directory not found at %s, %s, or ./templates. Ensure it exists.", tryPath, tryPathWd)
			}
			templateDir = tryPathWd
		} else {
			templateDir = tryPath
		}
	}

	log.Printf("Loading templates from: %s", templateDir)
	templates = template.Must(template.ParseGlob(filepath.Join(templateDir, "*.html")))
}
