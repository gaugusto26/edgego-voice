package subtitles

import (
	"fmt"
	"strings"
)

// FormatTimeSRT converte milissegundos para o formato de tempo SubRip (HH:MM:SS,mmm).
func FormatTimeSRT(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	minutes := (ms % 3600000) / 60000
	seconds := (ms % 60000) / 1000
	millis := ms % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

// FormatSRT converte uma lista de SentenceCue para uma string no formato SubRip (.srt).
func FormatSRT(cues []SentenceCue) string {
	if len(cues) == 0 {
		return ""
	}

	var sb strings.Builder
	// Pré-aloca estimativa razoável de memória (aproximadamente 64 bytes por legenda)
	sb.Grow(len(cues) * 64)

	for i, cue := range cues {
		cueIndex := cue.Index
		if cueIndex <= 0 {
			cueIndex = i + 1
		}

		startStr := FormatTimeSRT(cue.StartMs)
		endStr := FormatTimeSRT(cue.EndMs)

		fmt.Fprintf(&sb, "%d\n%s --> %s\n%s\n\n", cueIndex, startStr, endStr, strings.TrimSpace(cue.Text))
	}

	return sb.String()
}
