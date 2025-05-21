package main

import "time"

// Note defines the structure for a note.
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
