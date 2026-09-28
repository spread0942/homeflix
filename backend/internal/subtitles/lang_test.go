package subtitles

import "testing"

func TestParseFilename(t *testing.T) {
	cases := []struct {
		in, base, lang, label string
	}{
		{"Movie.it.srt", "Movie", "it", "Italian"},
		{"Movie.en.srt", "Movie", "en", "English"},
		{"Show_S01E01.eng.vtt", "Show_S01E01", "en", "English"},
		{"Film.srt", "Film", "und", "Subtitles"},
		{"Film_ita.srt", "Film", "it", "Italian"},
	}
	for _, c := range cases {
		base, lang, label := ParseFilename(c.in)
		if base != c.base || lang != c.lang || label != c.label {
			t.Fatalf("%s: got (%q,%q,%q) want (%q,%q,%q)", c.in, base, lang, label, c.base, c.lang, c.label)
		}
	}
}

func TestNormalizeLang(t *testing.T) {
	if got := NormalizeLang("ITA"); got != "it" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeLang(""); got != "und" {
		t.Fatalf("got %q", got)
	}
}
