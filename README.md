# Notes-Go-1 Proof of Concept

This is a simple proof-of-concept note-taking web application written in Go using only the standard library.

## Project Structure

```
notes-go-1/
├── main.go           # Main application file: server setup, routing
├── handlers.go       # HTTP handler functions for different routes
├── templates/        # Directory for HTML templates
│   ├── index.html    # Template for listing notes and creating new notes
│   └── note.html     # Template for viewing a single note
├── static/           # (Optional) For any static assets like CSS (very basic for PoC)
│   └── style.css     # (Currently empty)
├── notes.json        # Simple JSON file for data persistence (for PoC)
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

*   **Create Notes**: On the main page, use the form to create new notes with a title and content.
*   **List Notes**: The main page displays a list of all existing notes.
*   **View Note**: Click on a note's title in the list to view its full content on a separate page.

## Data Persistence

*   Notes are stored in a `notes.json` file in the root of the project directory.
*   If `notes.json` is empty or does not exist on first run, the application will create it and start with an empty list of notes. You might see a one-time console message "Error unmarshalling notes data: unexpected end of JSON input" if the file is completely empty, which is normal for the first run.

## Collaboration

Please refer to the `COLLABORATION_GUIDELINES.md` (provided earlier) for details on our development workflow.

Once you have reviewed the code, please push it to the `main` branch of our shared GitHub repository: `https://github.com/asmundstavdahl/notes-go-1.git`

