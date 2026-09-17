package sfx

import (
	"testing"
)

func TestNormalizeSFXType(t *testing.T) {
	testCases := []struct {
		input    string
		expected SFXType
		valid    bool
	}{
		{"hum", SFXType("hum"), true},
		{"hmm", SFXType("hum"), true},
		{"filler:hmm", SFXType("hum"), true},
		{"entendi", SFXType("entendi"), true},
		{"certo", SFXType("certo"), true},
		{"deixa-ver", SFXType("deixa-ver"), true},
		{"callcenter", SFXCallcenter, true},
		{"teclado", SFXKeyboard, true},
		{"digitando", SFXKeyboard, true},
		{"ruido", SFXNoise, true},
		{"desconhecido", SFXType("desconhecido"), false},
	}

	for _, tc := range testCases {
		res, ok := NormalizeSFXType(tc.input)
		if ok != tc.valid {
			t.Errorf("Para input '%s', esperava valid=%v, obteve %v", tc.input, tc.valid, ok)
		}
		if res != tc.expected {
			t.Errorf("Para input '%s', esperava '%s', obteve '%s'", tc.input, tc.expected, res)
		}
	}
}

func TestBuildCandidateFileNames(t *testing.T) {
	// 1. Teclado
	keyboardCandidates := buildCandidateFileNames("teclado", "pt-BR-FranciscaNeural")
	foundKeyboard := false
	for _, c := range keyboardCandidates {
		if c == "efeito-teclado" || c == "teclado" {
			foundKeyboard = true
			break
		}
	}
	if !foundKeyboard {
		t.Errorf("Esperava encontrar candidato para teclado, obteve: %+v", keyboardCandidates)
	}
}

func TestApplyTelephonyFilter_EmptyAudio(t *testing.T) {
	out, err := ApplyTelephonyFilter(nil, "mp3")
	if err != nil {
		t.Fatalf("Erro inesperado com áudio vazio: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("Esperava áudio vazio, obteve %d bytes", len(out))
	}
}
