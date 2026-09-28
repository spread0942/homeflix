package subtitles

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	srtIndexLine = regexp.MustCompile(`^\d+$`)
	srtTiming    = regexp.MustCompile(`^(\d{2}:\d{2}:\d{2}),(\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}),(\d{3})(.*)$`)
)

// ToVTT converts SubRip (.srt) text to WebVTT. If the input already looks like
// WebVTT, it is returned with a WEBVTT header ensured.
func ToVTT(raw []byte) []byte {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	text := string(raw)
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "WEBVTT") {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return []byte(text)
	}

	var b strings.Builder
	b.WriteString("WEBVTT\n\n")

	blocks := splitCueBlocks(text)
	for _, block := range blocks {
		lines := splitLines(block)
		if len(lines) == 0 {
			continue
		}
		i := 0
		if srtIndexLine.MatchString(strings.TrimSpace(lines[0])) {
			i = 1
		}
		if i >= len(lines) {
			continue
		}
		timing := strings.TrimSpace(lines[i])
		m := srtTiming.FindStringSubmatch(timing)
		if m == nil {
			continue
		}
		b.WriteString(m[1])
		b.WriteByte('.')
		b.WriteString(m[2])
		b.WriteString(" --> ")
		b.WriteString(m[3])
		b.WriteByte('.')
		b.WriteString(m[4])
		if m[5] != "" {
			b.WriteString(m[5])
		}
		b.WriteByte('\n')
		for _, line := range lines[i+1:] {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func splitCueBlocks(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitLines(block string) []string {
	raw := strings.Split(block, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		out = append(out, strings.TrimRight(line, "\r"))
	}
	return out
}
