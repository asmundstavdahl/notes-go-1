# Proof of Concept Design: Note-Taking Web App (Go)

This document outlines the project structure and basic API endpoints for the proof-of-concept (PoC) of the note-taking web application, built using only Go's standard library.

## 1. Project Structure

The project will be organized as follows within the `notes-go-1` repository:

```
notes-go-1/
├── main.go           # Main application file: server setup, routing
├── handlers.go       # HTTP handler functions for different routes
├── templates/        # Directory for HTML templates
│   ├── index.html    # Template for listing notes and creating new notes
│   └── note.html     # Template for viewing a single note
├── static/           # (Optional) For any static assets like CSS (very basic for PoC)
│   └── style.css
└── notes.json        # Simple JSON file for data persistence (for PoC)
```

*   **`main.go`**: This will be the entry point of our application. It will set up the HTTP server and define the routes using the `net/http` package.
*   **`handlers.go`**: This file will contain the functions that handle incoming HTTP requests for each route (e.g., displaying notes, creating a new note).
*   **`templates/`**: This directory will hold our HTML templates. We'll use Go's `html/template` package to render these.
    *   `index.html`: Will display a list of existing notes and a form (e.g., a text area and a submit button) to create new notes.
    *   `note.html`: Will display the content of a single selected note.
*   **`static/`**: (Optional for initial PoC) If we decide to add minimal styling, CSS files will go here. These will be served using `http.FileServer`.
*   **`notes.json`**: For the PoC, we will use a simple JSON file to store and retrieve notes. This avoids external databases and keeps dependencies to the standard library.

## 2. Data Model

A single note will have the following structure (represented as a Go struct):

```go
// In a suitable Go file, e.g., main.go or a new data.go
type Note struct {
    ID      string    `json:"id"`
    Title   string    `json:"title"` // Or just content if title is not needed for PoC
    Content string    `json:"content"`
    CreatedAt time.Time `json:"createdAt"`
}
```
For the PoC, the `ID` can be a simple timestamp-based unique string or a UUID if we want to be more robust (though UUID might require a package or careful stdlib implementation).

## 3. API Endpoints & Functionality

We will implement the following HTTP endpoints using Go's standard `net/http` library:

*   **`GET /`**
    *   **Handler**: `listNotesHandler` (in `handlers.go`)
    *   **Functionality**: Reads notes from `notes.json`, renders `index.html` template, displaying all notes and a form to create a new note.
    *   **UI**: Basic list of note titles/previews. Form with a textarea for note content and a submit button.

*   **`POST /notes/create`** (or simply `POST /` if we handle form submission on the same page)
    *   **Handler**: `createNoteHandler` (in `handlers.go`)
    *   **Functionality**: Parses the form data from the request, creates a new `Note` struct, assigns a unique ID, saves it to `notes.json`, and then redirects the user back to the main page (`GET /`).

*   **`GET /notes/{id}`**
    *   **Handler**: `viewNoteHandler` (in `handlers.go`)
    *   **Functionality**: Extracts the note ID from the URL path. Reads notes from `notes.json`, finds the note with the matching ID, and renders `note.html` template with the note's content.
    *   **UI**: Simple display of the note's content.

## 4. Implementation Details (Standard Library Focus)

*   **HTTP Server & Routing**: `net/http`
*   **HTML Templating**: `html/template`
*   **JSON Handling**: `encoding/json` for reading/writing `notes.json`.
*   **File I/O**: `os` package for reading/writing `notes.json`.
*   **Form Parsing**: Handled by `http.Request.ParseForm()` and related methods.
*   **Unique IDs**: For PoC, we can use `time.Now().UnixNano()` converted to a string, or a simple incrementing integer if concurrency is not an immediate concern for the PoC storage.

This design provides a solid foundation for the proof of concept, adhering to the requirement of using only Go's standard library and focusing on the core functionalities: creating, listing, and viewing notes with a basic web UI.
