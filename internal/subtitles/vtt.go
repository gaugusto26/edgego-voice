package subtitles

import (
	"fmt"
	"strings"
)

// FormatTimeVTT converte milissegundos para o formato de tempo WebVTT (HH:MM:SS.mmm).
func FormatTimeVTT(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	minutes := (ms % 3600000) / 60000
	seconds := (ms % 60000) / 1000
	millis := ms % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}

// FormatVTT converte uma lista de SentenceCue para uma string no formato WebVTT (.vtt).
func FormatVTT(cues []SentenceCue) string {
	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	if len(cues) == 0 {
		return sb.String()
	}

	sb.Grow(len(cues)*64 + 16)

	for i, cue := range cues {
		cueIndex := cue.Index
		if cueIndex <= 0 {
			cueIndex = i + 1
		}

		startStr := FormatTimeVTT(cue.StartMs)
		endStr := FormatTimeVTT(cue.EndMs)

		fmt.Fprintf(&sb, "%d\n%s --> %s\n%s\n\n", cueIndex, startStr, endStr, strings.TrimSpace(cue.Text))
	}

	return sb.String()
}
