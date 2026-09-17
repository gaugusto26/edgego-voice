package tts

import (
	"context"
	"testing"

	"edgego-voice/internal/capcuttts"
	"edgego-voice/internal/edgetts"
)

func TestMultiEngineRouting(t *testing.T) {
	edgeClient := edgetts.NewClient("")
	capcutClient := capcuttts.NewClient("")
	multi := NewMultiEngineClient(edgeClient, capcutClient)

	if !multi.IsCapCutVoice("wenrou") {
		t.Errorf("Esperava 'wenrou' ser identificada como voz CapCut")
	}
	if !multi.IsCapCutVoice("DiT_pt_male_wenrou") {
		t.Errorf("Esperava 'DiT_pt_male_wenrou' ser identificada como voz CapCut")
	}
	if !multi.IsCapCutVoice("empresario") {
		t.Errorf("Esperava 'empresario' ser identificada como voz CapCut")
	}
	if !multi.IsCapCutVoice("ICL_br_male_212M_LuizS") {
		t.Errorf("Esperava 'ICL_br_male_212M_LuizS' ser identificada como voz CapCut")
	}
	if multi.IsCapCutVoice("pt-BR-FranciscaNeural") {
		t.Errorf("'pt-BR-FranciscaNeural' não deve ser identificada como voz CapCut")
	}
}

func TestMultiEngineCapCutLive(t *testing.T) {
	if testing.Short() {
		t.Skip("Pulando teste de rede em modo -short")
	}

	edgeClient := edgetts.NewClient("")
	capcutClient := capcuttts.NewClient("")
	multi := NewMultiEngineClient(edgeClient, capcutClient)

	ctx := context.Background()
	audio, err := multi.Synthesize(ctx, edgetts.SynthesizeOptions{
		Text:         "Teste de roteamento unificado no EdgeGo Voice.",
		Voice:        "wenrou",
		Rate:         "1.0",
		Emotion:      "calm",
		EmotionScale: "1.0",
		Format:       "mp3",
	})

	if err != nil {
		t.Skipf("Síntese ao vivo do CapCut indisponível no ambiente de rede: %v", err)
		return
	}

	if len(audio) < 1000 {
		t.Fatalf("Áudio retornado muito pequeno: %d bytes", len(audio))
	}
}
