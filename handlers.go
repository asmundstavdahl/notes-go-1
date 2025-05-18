package main

import (
    "encoding/json"
    "html/template"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "time"
)

// Note defines the structure for a note
type Note struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"createdAt"`
}

var (
    notes      = make(map[string]Note)
    notesMutex = &sync.RWMutex{}
    templates  *template.Template
    notesFile  = "notes.json"
)

// loadNotes loads notes from the JSON file into memory
func loadNotes() {
    notesMutex.Lock()
    defer notesMutex.Unlock()

    data, err := ioutil.ReadFile(notesFile)
    if err != nil {
        if os.IsNotExist(err) {
            log.Printf("notes.json not found, starting with empty notes list.")
            notes = make(map[string]Note) // Ensure notes is initialized
            // Create an empty JSON file if it doesn't exist to avoid issues on first save
            emptyNotes := []Note{}
            emptyData, _ := json.MarshalIndent(emptyNotes, "", "  ")
            _ = ioutil.WriteFile(notesFile, emptyData, 0644)
            return
        }
        log.Printf("Error reading notes file: %v", err)
        return
    }

    var notesList []Note
    if err := json.Unmarshal(data, &notesList); err != nil {
        log.Printf("Error unmarshalling notes data: %v", err)
        // If unmarshalling fails, start with an empty list to prevent app crash
        notes = make(map[string]Note)
        return
    }
    
    tempNotes := make(map[string]Note)
    for _, note := range notesList {
        tempNotes[note.ID] = note
    }
    notes = tempNotes
    log.Printf("Loaded %d notes from %s", len(notes), notesFile)
}

// saveNotes saves the current notes from memory to the JSON file
func saveNotes() {
    notesMutex.RLock()
    defer notesMutex.RUnlock()

    var notesList []Note
    for _, note := range notes {
        notesList = append(notesList, note)
    }

    data, err := json.MarshalIndent(notesList, "", "  ")
    if err != nil {
        log.Printf("Error marshalling notes: %v", err)
        return
    }

    if err := ioutil.WriteFile(notesFile, data, 0644); err != nil {
        log.Printf("Error writing notes to file: %v", err)
    }
    log.Printf("Saved %d notes to %s", len(notesList), notesFile)
}

// listNotesHandler handles requests to the root path and displays notes
func listNotesHandler(w http.ResponseWriter, r *http.Request) {
    notesMutex.RLock()
    var currentNotes []Note
    // To ensure a somewhat consistent order for display, though map iteration is not guaranteed
    // A more robust solution would sort by CreatedAt
    ids := make([]string, 0, len(notes))
    for id := range notes {
        ids = append(ids, id)
    }
    // Simple sort by ID for now (can be improved to sort by CreatedAt)
    // For PoC, this might be sufficient or can be enhanced later.
    // sort.Strings(ids) // if we want to sort by ID string

    for _, id := range ids { // Iterate in a more predictable order if ids are sorted
        currentNotes = append(currentNotes, notes[id])
    }
    if len(ids) == 0 { // if no ids, means notes map is empty or only just initialized
         for _, note := range notes {
            currentNotes = append(currentNotes, note)
        }
    }

    notesMutex.RUnlock()

    pageData := struct {
        Notes []Note
    }{
        Notes: currentNotes,
    }

    err := templates.ExecuteTemplate(w, "index.html", pageData)
    if err != nil {
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

    title := r.FormValue("title")
    content := r.FormValue("content")

    if title == "" || content == "" {
        http.Error(w, "Title and content cannot be empty", http.StatusBadRequest)
        return
    }

    notesMutex.Lock()
    newID := strconv.FormatInt(time.Now().UnixNano(), 10)
    note := Note{
        ID:        newID,
        Title:     title,
        Content:   content,
        CreatedAt: time.Now(),
    }
    notes[note.ID] = note
    notesMutex.Unlock()

    saveNotes()
    http.Redirect(w, r, "/", http.StatusFound)
}

// viewNoteHandler handles requests to view a single note
func viewNoteHandler(w http.ResponseWriter, r *http.Request) {
    pathParts := strings.Split(r.URL.Path, "/")
    if len(pathParts) < 3 || pathParts[2] == "" {
        http.Error(w, "Note ID is missing", http.StatusBadRequest)
        return
    }
    noteID := pathParts[2]

    notesMutex.RLock()
    note, exists := notes[noteID]
    notesMutex.RUnlock()
    
    // The note.html template expects a struct with fields Note and Found.
    templateData := struct {
        Note  Note
        Found bool
    }{
        Note:  note,
        Found: exists,
    }

    if !exists {
        w.WriteHeader(http.StatusNotFound)
    }

    err := templates.ExecuteTemplate(w, "note.html", templateData)
    if err != nil {
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


