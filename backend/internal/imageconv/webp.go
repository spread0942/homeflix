package imageconv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SaveUploadAsWebP writes src to a temp file, converts it to WebP at destAbs (.webp),
// and removes the temp original.
func SaveUploadAsWebP(srcPath, destAbs string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not installed")
	}
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return err
	}
	tmp := destAbs + ".partial.webp"
	_ = os.Remove(tmp)

	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", srcPath,
		"-frames:v", "1",
		"-c:v", "libwebp",
		"-quality", "82",
		tmp,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("webp convert: %w", err)
	}
	if err := os.Rename(tmp, destAbs); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ExtractFrameAsWebP grabs a still from videoPath and writes a WebP poster to destAbs.
// Prefers a mid-intro seek (~10s, or 10% of duration when shorter) to avoid black frames.
func ExtractFrameAsWebP(videoPath, destAbs string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not installed")
	}
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return err
	}

	seek := frameSeekSeconds(videoPath)
	tmp := destAbs + ".partial.webp"
	_ = os.Remove(tmp)

	// -ss before -i for fast seek; fall back to t=0 if that fails (very short clips).
	if err := runFrameExtract(videoPath, tmp, seek); err != nil {
		_ = os.Remove(tmp)
		if seek > 0 {
			if err2 := runFrameExtract(videoPath, tmp, 0); err2 != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("frame extract: %w", err2)
			}
		} else {
			return fmt.Errorf("frame extract: %w", err)
		}
	}

	if err := os.Rename(tmp, destAbs); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func runFrameExtract(videoPath, dest string, seekSec float64) error {
	args := []string{"-y"}
	if seekSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", seekSec))
	}
	args = append(args,
		"-i", videoPath,
		"-frames:v", "1",
		"-c:v", "libwebp",
		"-quality", "82",
		dest,
	)
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func frameSeekSeconds(videoPath string) float64 {
	const prefer = 10.0
	dur := probeDurationSeconds(videoPath)
	if dur <= 0 {
		return prefer
	}
	if dur <= 2 {
		return 0
	}
	// ~10% into the file, capped at 10s, and keep at least 1s from the end.
	seek := dur * 0.1
	if seek > prefer {
		seek = prefer
	}
	if seek >= dur-0.5 {
		seek = dur / 2
	}
	if seek < 0 {
		return 0
	}
	return seek
}

func probeDurationSeconds(videoPath string) float64 {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0
	}
	cmd := exec.Command(
		"ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return sec
}
