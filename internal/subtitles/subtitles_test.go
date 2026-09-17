package subtitles

import (
	"strings"
	"testing"
)

func TestFormatTimeSRT(t *testing.T) {
	tests := []struct {
		name     string
		ms       int64
		expected string
	}{
		{"Zero", 0, "00:00:00,000"},
		{"SubSecond", 350, "00:00:00,350"},
		{"SecondsAndMillis", 2540, "00:00:02,540"},
		{"Minutes", 65120, "00:01:05,120"},
		{"Hours", 3661005, "01:01:01,005"},
		{"NegativeClamped", -500, "00:00:00,000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimeSRT(tt.ms)
			if result != tt.expected {
				t.Errorf("FormatTimeSRT(%d) = %q, esperado %q", tt.ms, result, tt.expected)
			}
		})
	}
}

func TestFormatTimeVTT(t *testing.T) {
	tests := []struct {
		name     string
		ms       int64
		expected string
	}{
		{"Zero", 0, "00:00:00.000"},
		{"SubSecond", 350, "00:00:00.350"},
		{"SecondsAndMillis", 2540, "00:00:02.540"},
		{"Minutes", 65120, "00:01:05.120"},
		{"Hours", 3661005, "01:01:01.005"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimeVTT(tt.ms)
			if result != tt.expected {
				t.Errorf("FormatTimeVTT(%d) = %q, esperado %q", tt.ms, result, tt.expected)
			}
		})
	}
}

func TestFormatSRT(t *testing.T) {
	cues := []SentenceCue{
		{
			Index:   1,
			Text:    "Olá, seja bem-vindo!",
			StartMs: 0,
			EndMs:   1500,
		},
		{
			Index:   2,
			Text:    "Este é o EdgeGo Voice com legendas sincronizadas.",
			StartMs: 2000,
			EndMs:   5500,
		},
	}

	srt := FormatSRT(cues)
	if !strings.Contains(srt, "1\n00:00:00,000 --> 00:00:01,500\nOlá, seja bem-vindo!\n\n") {
		t.Errorf("SRT gerado incorreto para cue 1:\n%s", srt)
	}
	if !strings.Contains(srt, "2\n00:00:02,000 --> 00:00:05,500\nEste é o EdgeGo Voice com legendas sincronizadas.\n\n") {
		t.Errorf("SRT gerado incorreto para cue 2:\n%s", srt)
	}
}

func TestFormatVTT(t *testing.T) {
	cues := []SentenceCue{
		{
			Index:   1,
			Text:    "Primeira frase do áudio.",
			StartMs: 500,
			EndMs:   2200,
		},
	}

	vtt := FormatVTT(cues)
	if !strings.HasPrefix(vtt, "WEBVTT\n\n") {
		t.Errorf("VTT deve começar com 'WEBVTT\\n\\n', obteve:\n%s", vtt)
	}
	if !strings.Contains(vtt, "00:00:00.500 --> 00:00:02.200\nPrimeira frase do áudio.") {
		t.Errorf("VTT gerado incorreto:\n%s", vtt)
	}
}

func TestGenerateFallbackCues(t *testing.T) {
	text := "Primeira sentença de teste. Segunda sentença um pouco mais longa! E a terceira?"
	cues := GenerateFallbackCues(text, 6000)

	if len(cues) != 3 {
		t.Fatalf("Esperava 3 cues de sentença, obteve %d", len(cues))
	}

	if cues[0].StartMs != 0 {
		t.Errorf("Primeira cue deve iniciar em 0ms, obteve %d", cues[0].StartMs)
	}
	if cues[2].EndMs != 6000 {
		t.Errorf("Última cue deve terminar na duração total (6000ms), obteve %d", cues[2].EndMs)
	}
	if cues[1].StartMs != cues[0].EndMs {
		t.Errorf("Cue 2 deve começar exatamente onde cue 1 termina")
	}
}

