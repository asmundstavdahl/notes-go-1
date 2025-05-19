# Notes-Go-1 Proof of Concept

This is a simple proof-of-concept note-taking web application written in Go using only the standard library.

## Project Structure

```
notes-go-1/
├── main.go           # Main application file: server setup, routing
├── handlers.go       # HTTP handler functions for different routes
├── templates/        # Directory for HTML templates
│   ├── index.html    # Template for listing notes and creating new notes
│   ├── note.html     # Template for viewing a single note
│   └── keywords.html # Template for listing and filtering keywords
├── notes.db          # SQLite database file for data persistence (PoC)
├── DESIGN_POC.md     # Design document for the PoC
└── README.md         # This file
```

## Prerequisites

*   Go (Golang) installed (version 1.18 or higher recommended, as installed via `apt-get install golang-go` on Ubuntu 22.04).

## How to Run

1.  **Navigate to the project directory**:
    ```bash
    cd path/to/notes-go-1
    ```

2.  **Run the application**:
    ```bash
    go run main.go handlers.go
    ```
    The server will start, typically on `http://localhost:8080` (the console output will confirm the address).

3.  **Open your web browser** and go to `http://localhost:8080` to use the application.

## Functionality

*   **Create Notes**: On the main page, use the form to create new notes with content.
*   **List Notes**: The main page displays a list of all existing notes.
*   **View Note**: Click on a note in the list to view its full content on a separate page.
*   **Manage Keywords**: Assign comma-separated keywords to notes, list all keywords, and filter notes by keyword.

## Data Persistence

*   Notes are stored in a `notes.db` SQLite database file in the root of the project directory.
*   On first run, the application will create the `notes.db` database and the necessary `notes` table if they do not exist.

## Collaboration

Once you have reviewed the code, please push it to the `main` branch of our shared GitHub repository: `https://github.com/asmundstavdahl/notes-go-1.git`

