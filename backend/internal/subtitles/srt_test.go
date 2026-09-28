package subtitles

import (
	"strings"
	"testing"
)

func TestToVTTFromSRT(t *testing.T) {
	in := "1\n00:00:01,000 --> 00:00:04,000\nHello world\n\n2\n00:00:05,500 --> 00:00:07,000\nSecond line\n"
	out := string(ToVTT([]byte(in)))
	if !strings.HasPrefix(out, "WEBVTT\n") {
		t.Fatalf("missing WEBVTT header: %q", out)
	}
	if !strings.Contains(out, "00:00:01.000 --> 00:00:04.000") {
		t.Fatalf("timing not converted: %q", out)
	}
	if !strings.Contains(out, "Hello world") {
		t.Fatalf("cue text missing: %q", out)
	}
}

func TestToVTTPassthrough(t *testing.T) {
	in := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi\n"
	out := string(ToVTT([]byte(in)))
	if !strings.HasPrefix(out, "WEBVTT") {
		t.Fatalf("expected passthrough, got %q", out)
	}
	if !strings.Contains(out, "Hi") {
		t.Fatalf("cue missing: %q", out)
	}
}
