// Package types defines the text/ranke encoding — a normalized plaintext format
// shared across ranke workers.
//
// A text/ranke document has two sections separated by a blank line:
//
//	Key: Value
//	Key (label): Value
//
//	Free text body. Can be multiple lines.
//	No structure imposed here.
//
// Headers are structured and mechanically parseable. The body is freeform —
// whatever unstructured content the source contained. Both sections are
// optional: a document may have only headers, only body, or both.
//
// This format is both human/LLM-readable and machine-parseable.
package types

import (
	"fmt"
	"strings"
)

// Document is a text/ranke document: structured headers + freeform body.
type Document struct {
	Headers []Field
	Body    string
}

// Field represents a single key-value pair with an optional label.
type Field struct {
	Key   string
	Label string
	Value string
}

// Encode renders a Document as text/ranke.
func (d Document) Encode() string {
	var b strings.Builder

	for _, f := range d.Headers {
		if f.Value == "" {
			continue
		}
		if f.Label != "" {
			fmt.Fprintf(&b, "%s (%s): %s\n", f.Key, f.Label, f.Value)
		} else {
			fmt.Fprintf(&b, "%s: %s\n", f.Key, f.Value)
		}
	}

	if d.Body != "" {
		if len(d.Headers) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(d.Body)
	}

	return strings.TrimRight(b.String(), "\n")
}

// Decode parses text/ranke into a Document.
func Decode(text string) Document {
	// Split on first blank line
	headers, body := splitHeaderBody(text)

	var doc Document
	for _, line := range strings.Split(headers, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if f, ok := parseLine(line); ok {
			doc.Headers = append(doc.Headers, f)
		}
	}
	doc.Body = body
	return doc
}

func splitHeaderBody(text string) (string, string) {
	// Look for a blank line (two consecutive newlines)
	if i := strings.Index(text, "\n\n"); i >= 0 {
		return text[:i], strings.TrimSpace(text[i+2:])
	}
	// No blank line — could be all headers or all body.
	// If every line looks like a header, treat it as headers-only.
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for _, line := range lines {
		if _, ok := parseLine(line); !ok {
			// Non-header line found — treat entire text as body
			return "", strings.TrimSpace(text)
		}
	}
	return strings.TrimSpace(text), ""
}

func parseLine(line string) (Field, bool) {
	colon := strings.Index(line, ": ")
	if colon < 0 {
		return Field{}, false
	}
	key := line[:colon]
	value := line[colon+2:]

	// Keys must not contain newlines or colons
	if strings.ContainsAny(key, ":\n") {
		return Field{}, false
	}

	// Check for label: "Key (label)"
	if i := strings.Index(key, " ("); i >= 0 && strings.HasSuffix(key, ")") {
		label := key[i+2 : len(key)-1]
		key = key[:i]
		return Field{Key: key, Label: label, Value: value}, true
	}

	return Field{Key: key, Value: value}, true
}
