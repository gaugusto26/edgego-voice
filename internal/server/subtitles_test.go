package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"edgego-voice/internal/audiocache"
	"edgego-voice/internal/config"
	"edgego-voice/internal/edgetts"
	"edgego-voice/internal/personas"
	"edgego-voice/internal/subtitles"
)

type mockSynthesizerForSubtitles struct{}

func (m *mockSynthesizerForSubtitles) Synthesize(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, error) {
	// 4800 bytes ~ 800ms de MP3 simulado
	return append([]byte{0xFF, 0xF3, 0x84, 0xC4}, make([]byte, 4796)...), nil
}

func (m *mockSynthesizerForSubtitles) SynthesizeWithSubtitles(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, []subtitles.WordTimestamp, []subtitles.SentenceCue, error) {
	audio := append([]byte{0xFF, 0xF3, 0x84, 0xC4}, make([]byte, 4796)...)
	words := []subtitles.WordTimestamp{
		{Word: "Olá", StartMs: 0, EndMs: 400},
		{Word: "mundo", StartMs: 420, EndMs: 800},
	}
	sents := []subtitles.SentenceCue{
		{Index: 1, Text: "Olá mundo", StartMs: 0, EndMs: 800},
	}
	return audio, words, sents, nil
}

func setupTestRouterForSubtitles(t *testing.T) http.Handler {
	t.Helper()
	cfg := &config.Config{
		Port:          5050,
		APIKey:        "test-key",
		DefaultVoice:  "pt-BR-FranciscaNeural",
		DefaultFormat: "mp3",
		DefaultSpeed:  1.0,
		RequireAPIKey: false,
		CacheEnabled:  true,
		CacheMaxMB:    10,
		CacheTTL:      1 * time.Hour,
	}

	vm := edgetts.NewVoiceManager("../../voices.json")
	pm := personas.NewManager("test_personas.json")
	cache := audiocache.NewCache(true, 10, 1*time.Hour)
	synth := &mockSynthesizerForSubtitles{}

	return SetupRouter(cfg, synth, vm, pm, cache)
}

func TestSpeechWithSubtitles_VerboseJSON(t *testing.T) {
	router := setupTestRouterForSubtitles(t)

	reqBody := map[string]interface{}{
		"model":          "tts-1",
		"input":          "Olá mundo",
		"voice":          "pt-BR-FranciscaNeural",
		"with_subtitles": true,
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Esperava status 200, obteve %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Esperava Content-Type application/json, obteve: %s", contentType)
	}

	durHeader := rec.Header().Get("X-Subtitles-Duration-Ms")
	if durHeader == "" {
		t.Errorf("Header X-Subtitles-Duration-Ms ausente")
	}

	var resp SpeechVerboseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Falha ao decodificar JSON de resposta com legendas: %v", err)
	}

	if resp.Audio == "" {
		t.Errorf("Áudio em base64 não deve ser vazio")
	}
	decodedAudio, err := base64.StdEncoding.DecodeString(resp.Audio)
	if err != nil || len(decodedAudio) == 0 {
		t.Errorf("Falha ao decodificar áudio base64: %v", err)
	}

	if len(resp.Subtitles.Sentences) == 0 {
		t.Errorf("Esperava sentenças legendadas, obteve 0")
	}
	if !strings.Contains(resp.Subtitles.SRT, "-->") {
		t.Errorf("SRT inválido: %s", resp.Subtitles.SRT)
	}
	if !strings.HasPrefix(resp.Subtitles.VTT, "WEBVTT") {
		t.Errorf("VTT inválido: %s", resp.Subtitles.VTT)
	}
}

func TestAudioSubtitles_EndpointSRT(t *testing.T) {
	router := setupTestRouterForSubtitles(t)

	reqBody := map[string]interface{}{
		"input":           "Olá mundo",
		"voice":           "pt-BR-FranciscaNeural",
		"subtitle_format": "srt",
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/subtitles", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Esperava status 200, obteve %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/x-subrip") {
		t.Errorf("Esperava Content-Type application/x-subrip, obteve: %s", contentType)
	}

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "00:00:00,000 -->") || !strings.Contains(bodyStr, "Olá mundo") {
		t.Errorf("Conteúdo SRT incorreto: %s", bodyStr)
	}
}

func TestAudioSubtitles_EndpointVTT(t *testing.T) {
	router := setupTestRouterForSubtitles(t)

	reqBody := map[string]interface{}{
		"input":           "Olá mundo",
		"voice":           "pt-BR-FranciscaNeural",
		"subtitle_format": "vtt",
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/subtitles", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Esperava status 200, obteve %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/vtt") {
		t.Errorf("Esperava Content-Type text/vtt, obteve: %s", contentType)
	}

	bodyStr := rec.Body.String()
	if !strings.HasPrefix(bodyStr, "WEBVTT") || !strings.Contains(bodyStr, "00:00:00.000 -->") {
		t.Errorf("Conteúdo VTT incorreto: %s", bodyStr)
	}
}

func TestAudioSubtitles_EndpointJSON(t *testing.T) {
	router := setupTestRouterForSubtitles(t)

	reqBody := map[string]interface{}{
		"input":           "Olá mundo",
		"voice":           "pt-BR-FranciscaNeural",
		"subtitle_format": "json",
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/subtitles", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Esperava status 200, obteve %d: %s", rec.Code, rec.Body.String())
	}

	var res subtitles.SubtitleResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("Falha ao decodificar JSON de legendas: %v", err)
	}

	if len(res.Sentences) == 0 {
		t.Errorf("Esperava sentenças na resposta JSON")
	}
	if len(res.Words) == 0 {
		t.Errorf("Esperava palavras na resposta JSON")
	}
}
