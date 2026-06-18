// Package media holds ffmpeg-backed media processing used by the attachments
// domain (kept in infra so the domain never execs a process or touches the disk).
package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FFmpegAudioConverter converts audio with the ffmpeg binary on PATH. WebM/Opus and
// Ogg/Opus share the Opus codec, so the conversion is a container remux (stream
// copy) — fast and lossless — with an Opus re-encode fallback for the rare stream
// that won't copy cleanly.
type FFmpegAudioConverter struct {
	// binary is the ffmpeg executable name/path; defaults to "ffmpeg".
	binary string
}

// NewFFmpegAudioConverter builds the converter using the ffmpeg binary on PATH.
func NewFFmpegAudioConverter() *FFmpegAudioConverter {
	return &FFmpegAudioConverter{binary: "ffmpeg"}
}

// ToOgg converts WebM/Opus bytes to Ogg/Opus. It first tries a stream copy
// (`-c copy`, no re-encode, no quality loss); if that fails it falls back to an
// Opus re-encode (`-c:a libopus`). The returned bool is true when the re-encode
// path was used.
func (c *FFmpegAudioConverter) ToOgg(ctx context.Context, in []byte) ([]byte, bool, error) {
	if len(in) == 0 {
		return nil, false, fmt.Errorf("empty audio input")
	}
	dir, err := os.MkdirTemp("", "audio-remux-")
	if err != nil {
		return nil, false, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	inPath := filepath.Join(dir, "in.webm")
	outPath := filepath.Join(dir, "out.ogg")
	if err := os.WriteFile(inPath, in, 0o600); err != nil {
		return nil, false, fmt.Errorf("write temp input: %w", err)
	}

	// 1) Remux: copy the Opus stream into an Ogg container (no re-encode).
	if copyErr := c.run(ctx, inPath, outPath, false); copyErr == nil {
		if out, rerr := os.ReadFile(outPath); rerr == nil && len(out) > 0 {
			return out, false, nil
		}
	}

	// 2) Fallback: re-encode to Opus (handles the odd stream that won't copy).
	_ = os.Remove(outPath)
	if err := c.run(ctx, inPath, outPath, true); err != nil {
		return nil, false, err
	}
	out, err := os.ReadFile(outPath)
	if err != nil {
		return nil, false, fmt.Errorf("read remux output: %w", err)
	}
	if len(out) == 0 {
		return nil, false, fmt.Errorf("ffmpeg produced empty output")
	}
	return out, true, nil
}

// run executes one ffmpeg invocation. reencode=false copies the stream; true
// re-encodes to Opus. Output is always an Ogg container.
func (c *FFmpegAudioConverter) run(ctx context.Context, inPath, outPath string, reencode bool) error {
	args := []string{"-y", "-i", inPath}
	if reencode {
		args = append(args, "-c:a", "libopus")
	} else {
		args = append(args, "-c", "copy")
	}
	args = append(args, "-f", "ogg", outPath)

	cmd := exec.CommandContext(ctx, c.binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
