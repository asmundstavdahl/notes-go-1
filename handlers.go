package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// listNotesHandler handles requests to the root path and displays notes (with optional keyword filters)
func listNotesHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve notes and their keywords
	rows, err := db.Query(
		`SELECT n.id, n.content, n.created_at, k.name
		 FROM notes n
		 LEFT JOIN note_keywords nk ON n.id = nk.note_id
		 LEFT JOIN keywords k ON nk.keyword_id = k.id
		 ORDER BY n.created_at DESC`,
	)
	if err != nil {
		log.Printf("Error querying notes: %v", err)
		http.Error(w, "Error fetching notes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Map notes to their keywords
	noteMap := make(map[string]*NoteWithKeywords)
	order := []string{}
	for rows.Next() {
		var id, content string
		var createdAt time.Time
		var kwName sql.NullString
		if err := rows.Scan(&id, &content, &createdAt, &kwName); err != nil {
			log.Printf("Error scanning note row: %v", err)
			continue
		}
		if _, exists := noteMap[id]; !exists {
			noteMap[id] = &NoteWithKeywords{Note: Note{ID: id, Content: content, CreatedAt: createdAt}}
			order = append(order, id)
		}
		if kwName.Valid {
			noteMap[id].Keywords = append(noteMap[id].Keywords, Keyword{Name: kwName.String})
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("Row iteration error: %v", err)
	}

	// Build slice in original order
	notes := make([]NoteWithKeywords, 0, len(order))
	for _, id := range order {
		notes = append(notes, *noteMap[id])
	}

	// Retrieve all keywords for filter list
	kwRows, err := db.Query("SELECT name FROM keywords ORDER BY name")
	if err != nil {
		log.Printf("Error querying keywords: %v", err)
	}
	defer func() {
		if kwRows != nil {
			kwRows.Close()
		}
	}()
	var allKeywords []Keyword
	for kwRows != nil && kwRows.Next() {
		var k Keyword
		if err := kwRows.Scan(&k.Name); err != nil {
			log.Printf("Error scanning keyword: %v", err)
			continue
		}
		allKeywords = append(allKeywords, k)
	}
	if kwRows != nil {
		if err := kwRows.Err(); err != nil {
			log.Printf("Keyword row iteration error: %v", err)
		}
	}

	pageData := struct {
		Notes    []NoteWithKeywords
		Keywords []Keyword
	}{
		Notes:    notes,
		Keywords: allKeywords,
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

	if kwInput := r.FormValue("keywords"); kwInput != "" {
		for _, part := range strings.Split(kwInput, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if _, err := db.Exec("INSERT OR IGNORE INTO keywords(name) VALUES(?)", name); err != nil {
				log.Printf("Error inserting keyword %q: %v", name, err)
				continue
			}
			var kid int
			if err := db.QueryRow("SELECT id FROM keywords WHERE name = ?", name).Scan(&kid); err != nil {
				log.Printf("Error retrieving keyword ID for %q: %v", name, err)
				continue
			}
			if _, err := db.Exec("INSERT OR IGNORE INTO note_keywords(note_id, keyword_id) VALUES(?, ?)", newID, kid); err != nil {
				log.Printf("Error linking note %s with keyword %q: %v", newID, name, err)
			}
		}
	} else {
		var existing []string
		kwRows, err := db.Query("SELECT name FROM keywords ORDER BY name")
		if err != nil {
			log.Printf("Error querying existing keywords: %v", err)
		} else {
			defer kwRows.Close()
			for kwRows.Next() {
				var k string
				if err := kwRows.Scan(&k); err != nil {
					log.Printf("Error scanning existing keyword: %v", err)
					continue
				}
				existing = append(existing, k)
			}
			if err := kwRows.Err(); err != nil {
				log.Printf("Existing keywords iteration error: %v", err)
			}
		}
		autoKeys, err := extractKeywords(content, existing)
		if err != nil {
			log.Printf("Error extracting keywords: %v", err)
		} else {
			for _, name := range autoKeys {
				if _, err := db.Exec("INSERT OR IGNORE INTO keywords(name) VALUES(?)", name); err != nil {
					log.Printf("Error inserting keyword %q: %v", name, err)
					continue
				}
				var kid int
				if err := db.QueryRow("SELECT id FROM keywords WHERE name = ?", name).Scan(&kid); err != nil {
					log.Printf("Error retrieving keyword ID for %q: %v", name, err)
					continue
				}
				if _, err := db.Exec("INSERT OR IGNORE INTO note_keywords(note_id, keyword_id) VALUES(?, ?)", newID, kid); err != nil {
					log.Printf("Error linking note %s with keyword %q: %v", newID, name, err)
				}
			}
		}
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

	// Prepare keyword list for this note
	var noteKeywords []Keyword
	if err == nil {
		krows, kerr := db.Query(
			"SELECT k.name FROM keywords k JOIN note_keywords nk ON k.id = nk.keyword_id WHERE nk.note_id = ?",
			noteID,
		)
		if kerr != nil {
			log.Printf("Error querying keywords for note %s: %v", noteID, kerr)
		} else {
			defer krows.Close()
			for krows.Next() {
				var k Keyword
				if err := krows.Scan(&k.Name); err != nil {
					log.Printf("Error scanning keyword for note %s: %v", noteID, err)
					continue
				}
				noteKeywords = append(noteKeywords, k)
			}
			if cerr := krows.Err(); cerr != nil {
				log.Printf("Keyword row iteration error for note %s: %v", noteID, cerr)
			}
		}
	}

	templateData := struct {
		Note     Note
		Found    bool
		Keywords []Keyword
	}{
		Note:     note,
		Found:    err == nil,
		Keywords: noteKeywords,
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

// editNoteHandler handles displaying and updating an existing note, including re-extracting keywords.
func editNoteHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] == "" {
		http.Error(w, "Note ID is missing", http.StatusBadRequest)
		return
	}
	noteID := parts[3]
	if r.Method == http.MethodGet {
		var note Note
		err := db.QueryRow("SELECT id, content, created_at FROM notes WHERE id = ?", noteID).Scan(&note.ID, &note.Content, &note.CreatedAt)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		} else if err != nil {
			log.Printf("Error querying note for edit %s: %v", noteID, err)
			http.Error(w, "Error fetching note", http.StatusInternalServerError)
			return
		}
		var noteKeywords []Keyword
		kwRows, err := db.Query("SELECT k.name FROM keywords k JOIN note_keywords nk ON k.id = nk.keyword_id WHERE nk.note_id = ?", noteID)
		if err != nil {
			log.Printf("Error querying keywords for note %s: %v", noteID, err)
		} else {
			defer kwRows.Close()
			for kwRows.Next() {
				var k string
				if err := kwRows.Scan(&k); err != nil {
					log.Printf("Error scanning keyword for note %s: %v", noteID, err)
					continue
				}
				noteKeywords = append(noteKeywords, Keyword{Name: k})
			}
			if err := kwRows.Err(); err != nil {
				log.Printf("Keyword rows iteration error for note %s: %v", noteID, err)
			}
		}
		templateData := struct {
			Note     Note
			Keywords []Keyword
		}{
			Note:     note,
			Keywords: noteKeywords,
		}
		if err := templates.ExecuteTemplate(w, "edit_note.html", templateData); err != nil {
			log.Printf("Error executing edit template: %v", err)
			http.Error(w, "Error rendering edit page", http.StatusInternalServerError)
		}
	} else if r.Method == http.MethodPost {
		content := r.FormValue("content")
		if content == "" {
			http.Error(w, "Content cannot be empty", http.StatusBadRequest)
			return
		}
		if _, err := db.Exec("UPDATE notes SET content = ? WHERE id = ?", content, noteID); err != nil {
			log.Printf("Error updating note %s: %v", noteID, err)
			http.Error(w, "Error updating note", http.StatusInternalServerError)
			return
		}
		if _, err := db.Exec("DELETE FROM note_keywords WHERE note_id = ?", noteID); err != nil {
			log.Printf("Error clearing keywords for note %s: %v", noteID, err)
		}
		if kwInput := r.FormValue("keywords"); kwInput != "" {
			for _, part := range strings.Split(kwInput, ",") {
				name := strings.TrimSpace(part)
				if name == "" {
					continue
				}
				if _, err := db.Exec("INSERT OR IGNORE INTO keywords(name) VALUES(?)", name); err != nil {
					log.Printf("Error inserting keyword %q: %v", name, err)
					continue
				}
				var kid int
				if err := db.QueryRow("SELECT id FROM keywords WHERE name = ?", name).Scan(&kid); err != nil {
					log.Printf("Error retrieving keyword ID for %q: %v", name, err)
					continue
				}
				if _, err := db.Exec("INSERT OR IGNORE INTO note_keywords(note_id, keyword_id) VALUES(?, ?)", noteID, kid); err != nil {
					log.Printf("Error linking note %s with keyword %q: %v", noteID, name, err)
				}
			}
		} else {
			var existing []string
			kwRows, err := db.Query("SELECT name FROM keywords ORDER BY name")
			if err != nil {
				log.Printf("Error querying existing keywords: %v", err)
			} else {
				defer kwRows.Close()
				for kwRows.Next() {
					var k string
					if err := kwRows.Scan(&k); err != nil {
						log.Printf("Error scanning existing keyword: %v", err)
						continue
					}
					existing = append(existing, k)
				}
				if err := kwRows.Err(); err != nil {
					log.Printf("Existing keywords iteration error: %v", err)
				}
			}
			autoKeys, err := extractKeywords(content, existing)
			if err != nil {
				log.Printf("Error extracting keywords on update: %v", err)
			} else {
				for _, name := range autoKeys {
					if _, err := db.Exec("INSERT OR IGNORE INTO keywords(name) VALUES(?)", name); err != nil {
						log.Printf("Error inserting keyword %q: %v", name, err)
						continue
					}
					var kid int
					if err := db.QueryRow("SELECT id FROM keywords WHERE name = ?", name).Scan(&kid); err != nil {
						log.Printf("Error retrieving keyword ID for %q: %v", name, err)
						continue
					}
					if _, err := db.Exec("INSERT OR IGNORE INTO note_keywords(note_id, keyword_id) VALUES(?, ?)", noteID, kid); err != nil {
						log.Printf("Error linking note %s with keyword %q: %v", noteID, name, err)
					}
				}
			}
		}
		http.Redirect(w, r, fmt.Sprintf("/notes/%s", noteID), http.StatusFound)
	} else {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
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
	// Initialize templates with custom functions (e.g., for content truncation)
	funcMap := template.FuncMap{
		"shorten": func(s string) string {
			if len(s) > 100 {
				return s[:100] + "..."
			}
			return s
		},
		"joinKeywords": func(keys []Keyword) string {
			var names []string
			for _, k := range keys {
				names = append(names, k.Name)
			}
			return strings.Join(names, ", ")
		},
	}
	templates = template.Must(
		template.New("").Funcs(funcMap).
			ParseGlob(filepath.Join(templateDir, "*.html")),
	)
}

// listKeywordsHandler displays a page with all available keywords
func listKeywordsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT name FROM keywords ORDER BY name")
	if err != nil {
		log.Printf("Error querying keywords: %v", err)
		http.Error(w, "Error fetching keywords", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var keywords []Keyword
	for rows.Next() {
		var k Keyword
		if err := rows.Scan(&k.Name); err != nil {
			log.Printf("Error scanning keyword: %v", err)
			continue
		}
		keywords = append(keywords, k)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Keyword row iteration error: %v", err)
	}

	if err := templates.ExecuteTemplate(w, "keywords.html", keywords); err != nil {
		log.Printf("Error executing keywords template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

// notesByKeywordHandler displays notes associated with a specific keyword
func notesByKeywordHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "Keyword is missing", http.StatusBadRequest)
		return
	}
	keyword := parts[2]

	// Query notes filtered by keyword
	rows, err := db.Query(
		`SELECT n.id, n.content, n.created_at
		 FROM notes n
		 JOIN note_keywords nk ON n.id = nk.note_id
		 JOIN keywords k ON nk.keyword_id = k.id
		 WHERE k.name = ?
		 ORDER BY n.created_at DESC`,
		keyword,
	)
	if err != nil {
		log.Printf("Error querying notes for keyword %q: %v", keyword, err)
		http.Error(w, "Error fetching notes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Map filtered notes to include their keywords
	noteMap := make(map[string]*NoteWithKeywords)
	order := []string{}
	for rows.Next() {
		var id, content string
		var createdAt time.Time
		if err := rows.Scan(&id, &content, &createdAt); err != nil {
			log.Printf("Error scanning note row for keyword %q: %v", keyword, err)
			continue
		}
		if _, exists := noteMap[id]; !exists {
			noteMap[id] = &NoteWithKeywords{Note: Note{ID: id, Content: content, CreatedAt: createdAt}}
			order = append(order, id)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("Row iteration error: %v", err)
	}

	// Assemble slice of notes
	notes := make([]NoteWithKeywords, 0, len(order))
	for _, id := range order {
		notes = append(notes, *noteMap[id])
	}

	// Retrieve note-level keywords for each filtered note
	for i := range notes {
		nid := notes[i].Note.ID
		krows2, kerr2 := db.Query(
			"SELECT k.name FROM keywords k JOIN note_keywords nk ON k.id = nk.keyword_id WHERE nk.note_id = ?",
			nid,
		)
		if kerr2 != nil {
			log.Printf("Error querying keywords for note %s: %v", nid, kerr2)
			continue
		}
		for krows2.Next() {
			var k Keyword
			if err := krows2.Scan(&k.Name); err != nil {
				log.Printf("Error scanning keyword for note %s: %v", nid, err)
				continue
			}
			notes[i].Keywords = append(notes[i].Keywords, k)
		}
		krows2.Close()
		if cerr2 := krows2.Err(); cerr2 != nil {
			log.Printf("Keyword row iteration error for note %s: %v", nid, cerr2)
		}
	}

	// Retrieve all keywords for filter list
	kwRows, err := db.Query("SELECT name FROM keywords ORDER BY name")
	if err != nil {
		log.Printf("Error querying keywords: %v", err)
	}
	defer func() {
		if kwRows != nil {
			kwRows.Close()
		}
	}()
	var allKeywords []Keyword
	for kwRows != nil && kwRows.Next() {
		var k Keyword
		if err := kwRows.Scan(&k.Name); err != nil {
			log.Printf("Error scanning keyword: %v", err)
			continue
		}
		allKeywords = append(allKeywords, k)
	}
	if kwRows != nil {
		if err := kwRows.Err(); err != nil {
			log.Printf("Keyword row iteration error: %v", err)
		}
	}

	pageData := struct {
		Notes    []NoteWithKeywords
		Keywords []Keyword
	}{
		Notes:    notes,
		Keywords: allKeywords,
	}

	if err := templates.ExecuteTemplate(w, "index.html", pageData); err != nil {
		log.Printf("Error executing index template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}
