package main

import (
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var templates *template.Template

// initTemplates initializes HTML templates with custom functions.
func initTemplates() {
	templateDir := "templates"
	// Check if running from project root or if templates dir is directly accessible
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		// If not found, try to locate it relative to the executable's path
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Could not get executable path: %v", err)
		}
		exeDir := filepath.Dir(exePath)
		tryPath := filepath.Join(exeDir, "templates")
		if _, err := os.Stat(tryPath); os.IsNotExist(err) {
			// Fallback to checking relative to current working directory
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
