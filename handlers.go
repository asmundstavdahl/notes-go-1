package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
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

// Keyword defines a tag or label for a note.
type Keyword struct {
	Name string `json:"name"`
}

// NoteWithKeywords combines a Note with its associated Keywords.
type NoteWithKeywords struct {
	Note     Note
	Keywords []Keyword
}

var (
	db        *sql.DB
	templates *template.Template
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// extractKeywords extracts a focused list of keywords for a note.
// Most provided existing keywords are from a broad, assorted collection and
// should only be included if they are entirely appropriate for this note.
// It also suggests any new relevant keywords via the OpenAI API.
func extractKeywords(noteContent string, existing []string) ([]string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	systemPrompt := `You are an assistant that extracts a focused list of keywords for a note. Most of the provided existing keywords are from a broad, assorted collection and are unlikely to be relevant. Include only those existing keywords that are entirely appropriate for this note, and suggest any new relevant keywords. Given the note content and a list of existing keywords, output only valid JSON with a single top-level key "keywords" containing an array of strings. Do not include any additional text or explanation.`
	existingJSON, err := json.Marshal(existing)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal existing keywords: %v", err)
	}
	userPrompt := fmt.Sprintf("Existing keywords: %s\nNote content:\n%s\nRemember: most existing keywords are not relevant unless they are completely appropriate for this note. Only include existing keywords that are entirely appropriate, and suggest any new relevant keywords.", existingJSON, noteContent)

	reqBody := chatCompletionRequest{
		Model:       "gpt-4.1-nano",
		Messages:    []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}},
		Temperature: 0.2,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat completion request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completion request returned status %s: %s", resp.Status, string(data))
	}
	respDataBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read chat completion response: %v", err)
	}
	var respData chatCompletionResponse
	if err := json.Unmarshal(respDataBytes, &respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %v", err)
	}
	if len(respData.Choices) < 1 {
		return nil, fmt.Errorf("no choices in chat completion response")
	}
	raw := respData.Choices[0].Message.Content
	clean := strings.TrimSpace(raw)
	// Remove markdown code fences if present
	if strings.HasPrefix(clean, "```") {
		parts := strings.SplitN(clean, "\n", 2)
		if len(parts) > 1 {
			clean = parts[1]
		}
		clean = strings.TrimSuffix(clean, "```")
		clean = strings.TrimSpace(clean)
	}
	// Extract JSON object between first '{' and last '}'
	if start := strings.Index(clean, "{"); start >= 0 {
		if end := strings.LastIndex(clean, "}"); end > start {
			clean = clean[start : end+1]
		}
	}
	var parsed struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse keywords JSON %q: %v", clean, err)
	}
	return parsed.Keywords, nil
}

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
		templateData := struct{ Note Note }{Note: note}
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
