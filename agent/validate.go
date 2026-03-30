package agent

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ValidationResult holds the outcome of parsing a JSONL session file.
type ValidationResult struct {
	// TotalEvents is the total number of parsed events.
	TotalEvents int
	// Counts maps each event type to how many times it appeared.
	Counts map[ParsedEventType]int
	// Errors collects any requirement failures from [Require].
	Errors []string
}

// Require asserts that at least minCount events of the given type were parsed.
// Failures are accumulated in Errors and surfaced by [Check].
func (v *ValidationResult) Require(eventType ParsedEventType, minCount int) {
	got := v.Counts[eventType]
	if got < minCount {
		v.Errors = append(v.Errors,
			fmt.Sprintf("%s: got %d, want >= %d", eventType, got, minCount))
	}
}

// Check returns an error if any [Require] calls failed, or if no events were
// parsed at all. Returns nil on success.
func (v *ValidationResult) Check() error {
	if v.TotalEvents == 0 {
		return fmt.Errorf("no events parsed from JSONL")
	}
	if len(v.Errors) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(v.Errors, "\n  "))
	}
	return nil
}

// ValidateJSONL reads a JSONL session file line-by-line, parses each line with
// the given parser, and returns a [ValidationResult] with event counts.
// This is the shared validation logic used by both unit tests (against test
// fixtures) and CI (against live CLI output).
func ValidateJSONL(parser SessionParser, r io.Reader) (*ValidationResult, error) {
	result := &ValidationResult{
		Counts: make(map[ParsedEventType]int),
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB for large tool outputs
	for scanner.Scan() {
		events := parser.ParseLine(scanner.Bytes())
		for _, e := range events {
			result.Counts[e.Type]++
			result.TotalEvents++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning JSONL: %w", err)
	}

	return result, nil
}
