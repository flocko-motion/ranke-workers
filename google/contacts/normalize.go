package contacts

import (
	"context"
	"fmt"

	rankedb "github.com/flocko-motion/rankedb/worker"
	"github.com/flocko-motion/ranke-workers/types"
	"github.com/flocko-motion/ranke-workers/types/l0"
)

// Normalize queries for unprocessed VCF source nodes and converts each
// into a normalized text/ranke contact.
func Normalize(ctx context.Context, client *rankedb.Client, configID, runID string) error {
	vcfNodes, err := client.Queue(ctx, rankedb.QueueParams{
		ContentClass: "source",
		ContentType:  "data",
	})
	if err != nil {
		return fmt.Errorf("queue vcf nodes: %w", err)
	}

	if len(vcfNodes) == 0 {
		fmt.Println("No unprocessed VCF nodes found.")
		return nil
	}

	fmt.Printf("\nFound %d unprocessed VCF node(s).\n", len(vcfNodes))

	for i := range vcfNodes {
		node := &vcfNodes[i]

		// Only process VCF-encoded nodes
		if node.EncodingFormat != "vcf" {
			continue
		}

		contact, err := parseVCF(node.Content)
		if err != nil {
			fmt.Printf("  SKIP %s: %v\n", node.Id, err)
			continue
		}

		text := contact.ToText()

		_, err = client.CreateNode(ctx, rankedb.CreateNodeRequest{
			Level:          0,
			ContentClass:   "source",
			ContentType:    "contact",
			EncodingClass:  "text",
			EncodingFormat: "ranke",
			Content:        rankedb.Ptr(text),
			OriginalName:   rankedb.Ptr(node.OriginalName),
			Origin:         rankedb.Ptr(node.Origin),
			Edges: []rankedb.EdgeSpec{
				{Type: "provenance/input", SourceNodeID: node.Id, RunID: rankedb.Ptr(runID)},
				{Type: "provenance/worker", SourceNodeID: configID, RunID: rankedb.Ptr(runID)},
			},
		})
		if err != nil {
			fmt.Printf("  SKIP %s: %v\n", node.Id, err)
			continue
		}

		fmt.Printf("  + normalized %s\n", node.Id)
	}

	return nil
}

// parseVCF extracts a Contact from VCF content.
func parseVCF(content string) (l0.Contact, error) {
	if content == "" {
		return l0.Contact{}, fmt.Errorf("empty VCF")
	}

	var c l0.Contact
	var noteLines []string
	inNote := false

	for _, line := range splitVCFLines(content) {
		if inNote {
			if isVCFField(line) {
				inNote = false
			} else {
				noteLines = append(noteLines, line)
				continue
			}
		}

		key, params, value := parseVCFLine(line)
		label := vcfLabel(params)

		switch key {
		case "FN":
			c.Name = value
		case "N":
			if c.Name == "" {
				c.Name = formatStructuredName(value)
			}
		case "EMAIL":
			c.Emails = append(c.Emails, types.Field{Key: "Email", Label: label, Value: value})
		case "TEL":
			c.Phones = append(c.Phones, types.Field{Key: "Phone", Label: label, Value: value})
		case "ADR":
			addr := formatAddress(value)
			if addr != "" {
				c.Addresses = append(c.Addresses, types.Field{Key: "Address", Label: label, Value: addr})
			}
		case "ORG":
			c.Organization = unescapeVCF(value)
		case "TITLE":
			c.Title = value
		case "BDAY":
			c.Birthday = value
		case "NOTE":
			noteLines = append(noteLines, value)
			inNote = true
		}
	}

	if len(noteLines) > 0 {
		c.Notes = unescapeVCF(joinLines(noteLines))
	}

	if c.Name == "" {
		return c, fmt.Errorf("VCF missing name")
	}
	return c, nil
}

// splitVCFLines handles VCF line folding (continuation lines start with space/tab).
func splitVCFLines(content string) []string {
	var lines []string
	for _, raw := range splitLines(content) {
		if len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') && len(lines) > 0 {
			lines[len(lines)-1] += raw[1:]
		} else {
			lines = append(lines, raw)
		}
	}
	return lines
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func isVCFField(line string) bool {
	for i, c := range line {
		if c == ':' && i > 0 {
			return true
		}
		if c == ' ' || c == '\t' {
			return false
		}
	}
	return false
}

// parseVCFLine splits "KEY;PARAM=VAL:value" into key, params string, value.
func parseVCFLine(line string) (string, string, string) {
	colonIdx := -1
	for i, c := range line {
		if c == ':' {
			colonIdx = i
			break
		}
	}
	if colonIdx < 0 {
		return line, "", ""
	}

	keyPart := line[:colonIdx]
	value := line[colonIdx+1:]

	if semiIdx := indexByte(keyPart, ';'); semiIdx >= 0 {
		return keyPart[:semiIdx], keyPart[semiIdx+1:], value
	}
	return keyPart, "", value
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// vcfLabel extracts a human-readable label from VCF params like "TYPE=work".
func vcfLabel(params string) string {
	for _, p := range splitSemi(params) {
		if len(p) > 5 && (p[:5] == "TYPE=" || p[:5] == "type=") {
			return p[5:]
		}
		// Bare type (vCard 2.1): "WORK", "HOME", etc.
		switch p {
		case "WORK", "work":
			return "work"
		case "HOME", "home":
			return "home"
		case "CELL", "cell":
			return "cell"
		}
	}
	return ""
}

func splitSemi(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// formatStructuredName converts "Last;First;Middle;Prefix;Suffix" to "First Last".
func formatStructuredName(n string) string {
	parts := splitSemi(n)
	if len(parts) < 2 {
		return unescapeVCF(n)
	}
	first := unescapeVCF(parts[1])
	last := unescapeVCF(parts[0])
	if first != "" && last != "" {
		return first + " " + last
	}
	if first != "" {
		return first
	}
	return last
}

// formatAddress converts ";;Street;City;Region;Postal;Country" to "Street, City, Country".
func formatAddress(adr string) string {
	parts := splitSemi(adr)
	var pieces []string
	for _, p := range parts {
		p = unescapeVCF(p)
		if p != "" {
			pieces = append(pieces, p)
		}
	}
	result := ""
	for i, p := range pieces {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

func unescapeVCF(s string) string {
	r := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n', 'N':
				r += "\n"
			case ',':
				r += ","
			case ';':
				r += ";"
			case '\\':
				r += "\\"
			default:
				r += string(s[i+1])
			}
			i++
		} else {
			r += string(s[i])
		}
	}
	return r
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
