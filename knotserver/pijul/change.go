package pijul

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Change represents a Pijul change (analogous to a Git commit)
type Change struct {
	// Hash is the unique identifier for this change (base32 encoded)
	Hash string `json:"hash"`

	// Authors who created this change
	Authors []Author `json:"authors"`

	// Message is the change description
	Message string `json:"message"`

	// Timestamp when the change was recorded
	Timestamp time.Time `json:"timestamp"`

	// Dependencies are hashes of changes this change depends on
	Dependencies []string `json:"dependencies,omitempty"`

	// Channel where this change exists
	Channel string `json:"channel,omitempty"`
}

// Author represents a change author
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Changes returns a list of changes in the repository
// offset and limit control pagination
func (p *PijulRepo) Changes(offset, limit int) ([]Change, error) {
	args := []string{"--offset", strconv.Itoa(offset), "--limit", strconv.Itoa(limit)}

	if p.channelName != "" {
		args = append(args, "--channel", p.channelName)
	}

	output, err := p.log(args...)
	if err != nil {
		if isNoChangesError(err) {
			return []Change{}, nil
		}
		return nil, fmt.Errorf("pijul log: %w", err)
	}

	return parseLogOutput(output)
}

// TotalChanges returns the total number of changes in the current channel
func (p *PijulRepo) TotalChanges() (int, error) {
	// pijul log doesn't have a --count option, so we need to count
	// We can use pijul log with a large limit or iterate
	args := []string{"--hash-only"}

	if p.channelName != "" {
		args = append(args, "--channel", p.channelName)
	}

	output, err := p.log(args...)
	if err != nil {
		if isNoChangesError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("pijul log: %w", err)
	}

	// Count lines (each line is a change hash)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0, nil
	}

	return len(lines), nil
}

func isNoChangesError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "no changes") || strings.Contains(lower, "no change")
}

// GetChange retrieves details for a specific change by hash
func (p *PijulRepo) GetChange(hash string) (*Change, error) {
	// Use pijul change to get change details
	output, err := p.change(hash)
	if err != nil {
		return nil, fmt.Errorf("pijul change %s: %w", hash, err)
	}

	return parseChangeOutput(hash, output)
}

// parseLogOutput parses the output of pijul log
// Expected format (default output):
//
//	Hash: XXXXX
//	Author: Name <email>
//	Date: 2024-01-01 12:00:00
//
//	    Message line 1
//	    Message line 2
func parseLogOutput(output []byte) ([]Change, error) {
	var changes []Change
	scanner := bufio.NewScanner(bytes.NewReader(output))

	var current *Change
	var messageLines []string
	inMessage := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Hash: ") || strings.HasPrefix(line, "Change ") {
			// Save previous change if exists
			if current != nil {
				current.Message = strings.TrimSpace(strings.Join(messageLines, "\n"))
				changes = append(changes, *current)
			}

			hashLine := line
			if strings.HasPrefix(hashLine, "Change ") {
				hashLine = strings.Replace(hashLine, "Change ", "Hash: ", 1)
			}
			current = &Change{
				Hash: strings.TrimPrefix(hashLine, "Hash: "),
			}
			messageLines = nil
			inMessage = false
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "Author: ") {
			authorStr := strings.TrimPrefix(line, "Author: ")
			author := parseAuthor(authorStr)
			current.Authors = append(current.Authors, author)
			continue
		}

		if strings.HasPrefix(line, "Date: ") {
			dateStr := strings.TrimPrefix(line, "Date: ")
			if t, err := parseTimestamp(dateStr); err == nil {
				current.Timestamp = t
			}
			continue
		}

		// Empty line before message
		if line == "" && !inMessage {
			inMessage = true
			continue
		}

		if inMessage {
			messageLines = append(messageLines, strings.TrimPrefix(line, "    "))
		}
	}

	// Don't forget the last change
	if current != nil {
		current.Message = strings.TrimSpace(strings.Join(messageLines, "\n"))
		changes = append(changes, *current)
	}

	return changes, scanner.Err()
}

// parseChangeOutput parses the output of pijul change <hash>
func parseChangeOutput(hash string, output []byte) (*Change, error) {
	change := &Change{Hash: hash}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var messageLines []string
	inMessage := false
	inDeps := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "# Authors") {
			inDeps = false
			continue
		}

		if strings.HasPrefix(line, "# Dependencies") {
			inDeps = true
			continue
		}

		if strings.HasPrefix(line, "# Message") {
			inDeps = false
			inMessage = true
			continue
		}

		if strings.HasPrefix(line, "# ") {
			inDeps = false
			inMessage = false
			continue
		}

		if inDeps && strings.TrimSpace(line) != "" {
			change.Dependencies = append(change.Dependencies, strings.TrimSpace(line))
			continue
		}

		if inMessage {
			messageLines = append(messageLines, line)
			continue
		}

		// Parse author line
		if strings.Contains(line, "<") && strings.Contains(line, ">") {
			author := parseAuthor(line)
			change.Authors = append(change.Authors, author)
		}
	}

	change.Message = strings.TrimSpace(strings.Join(messageLines, "\n"))

	return change, scanner.Err()
}

// parseAuthor parses an author string like "Name <email>"
func parseAuthor(s string) Author {
	s = strings.TrimSpace(s)

	// Try to extract email from angle brackets
	if start := strings.Index(s, "<"); start != -1 {
		if end := strings.Index(s, ">"); end > start {
			return Author{
				Name:  strings.TrimSpace(s[:start]),
				Email: strings.TrimSpace(s[start+1 : end]),
			}
		}
	}

	return Author{Name: s}
}

// parseTimestamp parses various timestamp formats
func parseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	// Try common formats
	formats := []string{
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}

// ChangeJSON represents the JSON output format for pijul log --json
type ChangeJSON struct {
	Hash         string   `json:"hash"`
	Authors      []string `json:"authors"`
	Message      string   `json:"message"`
	Timestamp    string   `json:"timestamp"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// ChangesJSON returns changes using JSON output format (if available in pijul version)
func (p *PijulRepo) ChangesJSON(offset, limit int) ([]Change, error) {
	args := []string{
		"--offset", strconv.Itoa(offset),
		"-n", strconv.Itoa(limit),
		"--json",
	}

	if p.channelName != "" {
		args = append(args, "--channel", p.channelName)
	}

	output, err := p.log(args...)
	if err != nil {
		// Fall back to text parsing if JSON not supported
		return p.Changes(offset, limit)
	}

	var jsonChanges []ChangeJSON
	if err := json.Unmarshal(output, &jsonChanges); err != nil {
		// Fall back to text parsing if JSON parsing fails
		return p.Changes(offset, limit)
	}

	changes := make([]Change, len(jsonChanges))
	for i, jc := range jsonChanges {
		changes[i] = Change{
			Hash:         jc.Hash,
			Message:      jc.Message,
			Dependencies: jc.Dependencies,
		}

		for _, authorStr := range jc.Authors {
			changes[i].Authors = append(changes[i].Authors, parseAuthor(authorStr))
		}

		if t, err := parseTimestamp(jc.Timestamp); err == nil {
			changes[i].Timestamp = t
		}
	}

	return changes, nil
}
