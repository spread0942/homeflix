package transcode

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NeedsTranscode reports whether the container/extension is unlikely to play in browsers.
func NeedsTranscode(ext, contentType string) bool {
	ext = strings.ToLower(ext)
	ct := strings.ToLower(contentType)
	switch ext {
	case ".mkv", ".avi", ".wmv", ".flv", ".ts", ".m2ts", ".mpg", ".mpeg", ".mov":
		return true
	case ".webm", ".mp4", ".m4v", ".ogv":
		return false
	}
	switch {
	case strings.Contains(ct, "matroska"),
		strings.Contains(ct, "x-msvideo"),
		strings.Contains(ct, "quicktime"),
		strings.Contains(ct, "x-ms-wmv"),
		strings.Contains(ct, "mpeg"):
		return true
	}
	return false
}

// ToBrowserMP4 remuxes/transcodes src to H.264 + AAC MP4 at dest.
func ToBrowserMP4(src, dest string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not installed in server image")
	}
	tmp := dest + ".partial.mp4"
	_ = os.Remove(tmp)

	// Map first video + first audio; transcode for max browser compatibility (Firefox).
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", src,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "22",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-ac", "2",
		"-movflags", "+faststart",
		tmp,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ReplaceWithMP4 converts srcAbs to an .mp4 next to it and returns the new absolute path.
func ReplaceWithMP4(srcAbs string) (string, error) {
	ext := filepath.Ext(srcAbs)
	destAbs := strings.TrimSuffix(srcAbs, ext) + ".mp4"
	if err := ToBrowserMP4(srcAbs, destAbs); err != nil {
		return "", err
	}
	if srcAbs != destAbs {
		if err := os.Remove(srcAbs); err != nil {
			log.Printf("transcode: keep original after convert, remove failed: %v", err)
		}
	}
	return destAbs, nil
}
