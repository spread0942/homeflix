package imageconv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
