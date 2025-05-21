package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// chatMessage represents a message in a chat completion request or response.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletionRequest is the request body for OpenAI chat completions.
type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
}

// chatCompletionResponse represents the response from an OpenAI chat completion.
type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// extractDateKeywords scans note content for relative day mentions and explicit dates,
// returning unique ISO-formatted date keywords.
func extractDateKeywords(noteContent string) []string {
	now := time.Now()
	lower := strings.ToLower(noteContent)
	var dates []string
	if strings.Contains(lower, "i dag") {
		dates = append(dates, now.Format("2006-01-02"))
	}
	if strings.Contains(lower, "i går") {
		dates = append(dates, now.AddDate(0, 0, -1).Format("2006-01-02"))
	}
	if strings.Contains(lower, "i morgen") {
		dates = append(dates, now.AddDate(0, 0, 1).Format("2006-01-02"))
	}
	weekdays := map[string]time.Weekday{
		"mandag":  time.Monday,
		"tirsdag": time.Tuesday,
		"onsdag":  time.Wednesday,
		"torsdag": time.Thursday,
		"fredag":  time.Friday,
		"lørdag":  time.Saturday,
		"søndag":  time.Sunday,
	}
	for name, wd := range weekdays {
		if strings.Contains(lower, name) {
			diff := (int(wd) - int(now.Weekday()) + 7) % 7
			dates = append(dates, now.AddDate(0, 0, diff).Format("2006-01-02"))
		}
	}
	// explicit ISO date patterns
	isoRe := regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	for _, match := range isoRe.FindAllString(noteContent, -1) {
		dates = append(dates, match)
	}
	// explicit DMY date patterns (dd.mm.yyyy or dd/mm/yyyy)
	dmyRe := regexp.MustCompile(`\b(\d{1,2})[./](\d{1,2})[./](\d{4})\b`)
	for _, match := range dmyRe.FindAllString(noteContent, -1) {
		norm := strings.ReplaceAll(strings.ReplaceAll(match, ".", "-"), "/", "-")
		if t, err := time.Parse("2-1-2006", norm); err == nil {
			dates = append(dates, t.Format("2006-01-02"))
		} else if t2, err2 := time.Parse("02-01-2006", norm); err2 == nil {
			dates = append(dates, t2.Format("2006-01-02"))
		}
	}
	// dedupe
	uniq := make([]string, 0, len(dates))
	seen := make(map[string]struct{})
	for _, d := range dates {
		if _, ok := seen[d]; !ok {
			seen[d] = struct{}{}
			uniq = append(uniq, d)
		}
	}
	return uniq
}

// extractKeywords extracts a focused list of keywords for a note.
// It filters existing keywords and suggests new ones via the OpenAI API,
// also including date-based keywords.
func extractKeywords(noteContent string, existing []string) ([]string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	nextMonday := now.AddDate(0, 0, (int(time.Monday)-int(now.Weekday())+7)%7).Format("2006-01-02")

	examples := []struct {
		Note     string
		Keywords []string
	}{
		{Note: "Handle gaver i går", Keywords: []string{"handle", "gaver", yesterday}},
		{Note: "Teamsmøte i morgen om budsjett", Keywords: []string{"teamsmøte", "budsjett", tomorrow}},
		{Note: "Planlegg workshop på mandag", Keywords: []string{"planlegg", "workshop", nextMonday}},
		{Note: "Bestill konferanse 15.06.2025", Keywords: []string{"bestill", "konferanse", "2025-06-15"}},
	}
	var exBuf strings.Builder
	exBuf.WriteString("Examples:\n")
	for _, ex := range examples {
		exBuf.WriteString(fmt.Sprintf("Note content: \"%s\"\n", ex.Note))
		respObj := struct {
			Keywords []string `json:"keywords"`
		}{Keywords: ex.Keywords}
		data, _ := json.MarshalIndent(respObj, "", "  ")
		exBuf.WriteString("Response:\n")
		exBuf.Write(data)
		exBuf.WriteString("\n\n")
	}
	systemPrompt := fmt.Sprintf(`%sYou are an assistant that extracts a focused list of keywords for a note. Most of the provided existing keywords are from a broad, assorted collection and are unlikely to be relevant. Include only those existing keywords that are entirely appropriate for this note, and suggest any new relevant keywords. For any dates or day mentions in the note (e.g., "i dag", "i går", "i morgen", or weekday names like "mandag", "tirsdag", etc.), add corresponding date keywords in ISO format. Given the note content and a list of existing keywords, output only valid JSON with a single top-level key "keywords" containing an array of strings. Do not include any additional text or explanation. Today's date is %s.`, exBuf.String(), today)
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
	if strings.HasPrefix(clean, "```") {
		parts := strings.SplitN(clean, "\n", 2)
		if len(parts) > 1 {
			clean = parts[1]
		}
		clean = strings.TrimSuffix(clean, "```")
		clean = strings.TrimSpace(clean)
	}
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

	keywords := parsed.Keywords
	for _, d := range extractDateKeywords(noteContent) {
		found := false
		for _, k := range keywords {
			if k == d {
				found = true
				break
			}
		}
		if !found {
			keywords = append(keywords, d)
		}
	}
	return keywords, nil
}
