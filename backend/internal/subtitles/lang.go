package subtitles

import (
	"path/filepath"
	"strings"
)

// common filename language tags → BCP-47-ish short codes + display labels.
var langFromTag = map[string]struct {
	Code  string
	Label string
}{
	"en": {Code: "en", Label: "English"},
	"eng": {Code: "en", Label: "English"},
	"english": {Code: "en", Label: "English"},
	"it": {Code: "it", Label: "Italian"},
	"ita": {Code: "it", Label: "Italian"},
	"italian": {Code: "it", Label: "Italian"},
	"es": {Code: "es", Label: "Spanish"},
	"spa": {Code: "es", Label: "Spanish"},
	"spanish": {Code: "es", Label: "Spanish"},
	"fr": {Code: "fr", Label: "French"},
	"fre": {Code: "fr", Label: "French"},
	"fra": {Code: "fr", Label: "French"},
	"french": {Code: "fr", Label: "French"},
	"de": {Code: "de", Label: "German"},
	"ger": {Code: "de", Label: "German"},
	"deu": {Code: "de", Label: "German"},
	"german": {Code: "de", Label: "German"},
	"pt": {Code: "pt", Label: "Portuguese"},
	"por": {Code: "pt", Label: "Portuguese"},
	"ja": {Code: "ja", Label: "Japanese"},
	"jpn": {Code: "ja", Label: "Japanese"},
	"japanese": {Code: "ja", Label: "Japanese"},
	"zh": {Code: "zh", Label: "Chinese"},
	"chi": {Code: "zh", Label: "Chinese"},
	"zho": {Code: "zh", Label: "Chinese"},
	"chinese": {Code: "zh", Label: "Chinese"},
	"ko": {Code: "ko", Label: "Korean"},
	"kor": {Code: "ko", Label: "Korean"},
	"korean": {Code: "ko", Label: "Korean"},
	"ru": {Code: "ru", Label: "Russian"},
	"rus": {Code: "ru", Label: "Russian"},
	"russian": {Code: "ru", Label: "Russian"},
	"und": {Code: "und", Label: "Subtitles"},
}

// LabelFor returns a display label for a language code.
func LabelFor(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "Subtitles"
	}
	if info, ok := langFromTag[code]; ok {
		return info.Label
	}
	return strings.ToUpper(code)
}

// NormalizeLang maps aliases to a short code.
func NormalizeLang(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "und"
	}
	if info, ok := langFromTag[raw]; ok {
		return info.Code
	}
	// keep short free-form codes (2–3 letters)
	if len(raw) <= 3 {
		return raw
	}
	return "und"
}

// ParseFilename extracts a video basename and language from a subtitle filename.
// Examples: Movie.it.srt → ("Movie", "it", "Italian"); Movie.srt → ("Movie", "und", "Subtitles")
func ParseFilename(filename string) (base, lang, label string) {
	name := filepath.Base(filename)
	ext := strings.ToLower(filepath.Ext(name))
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if ext != ".srt" && ext != ".vtt" {
		return stem, "und", "Subtitles"
	}

	dot := strings.LastIndex(stem, ".")
	if dot > 0 {
		tag := strings.ToLower(stem[dot+1:])
		if info, ok := langFromTag[tag]; ok {
			return stem[:dot], info.Code, info.Label
		}
	}
	under := strings.LastIndex(stem, "_")
	if under > 0 {
		tag := strings.ToLower(stem[under+1:])
		if info, ok := langFromTag[tag]; ok {
			return stem[:under], info.Code, info.Label
		}
	}
	return stem, "und", "Subtitles"
}
