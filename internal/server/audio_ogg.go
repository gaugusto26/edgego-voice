package server

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// convertToOggOpus converts the Opus/WebM stream produced by the Edge TTS into
// the OGG/Opus container required by WhatsApp voice notes. The conversion is
// local: no audio leaves the host and nothing is re-encoded when the copy
// stream can be remuxed.
func convertToOggOpus(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("audio vazio")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Remux first: Opus packets are copied without re-encoding.
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", "pipe:0", "-c", "copy", "-f", "ogg", "pipe:1")
	cmd.Stdin = bytes.NewReader(data)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil && isOggOpus(out.Bytes()) {
		return out.Bytes(), nil
	}

	// Fallback: re-encode when the source cannot be remuxed directly.
	cmd = exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", "pipe:0", "-c:a", "libopus", "-ar", "16000", "-ac", "1", "-f", "ogg", "pipe:1")
	cmd.Stdin = bytes.NewReader(data)
	out.Reset()
	stderr.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg falhou: %w: %s", err, stderr.String())
	}
	if !isOggOpus(out.Bytes()) {
		return nil, fmt.Errorf("saida ffmpeg nao e OGG/Opus")
	}
	return out.Bytes(), nil
}

// isOggOpus checks the OGG container magic and the Opus stream header.
func isOggOpus(data []byte) bool {
	if len(data) < 4 || string(data[:4]) != "OggS" {
		return false
	}
	return bytes.Contains(data[:512], []byte("OpusHead"))
}
