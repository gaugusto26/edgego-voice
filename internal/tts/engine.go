package tts

import (
	"context"
	"log/slog"

	"edgego-voice/internal/capcuttts"
	"edgego-voice/internal/edgetts"
	"edgego-voice/internal/subtitles"
)

// Synthesizer é a interface unificada de síntese aceita pelos orquestradores e handlers de áudio.
type Synthesizer interface {
	Synthesize(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, error)
}

// SubtitleSynthesizer estende Synthesizer para motores que suportam retorno de legendas alinhadas.
type SubtitleSynthesizer interface {
	Synthesizer
	SynthesizeWithSubtitles(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, []subtitles.WordTimestamp, []subtitles.SentenceCue, error)
}

// MultiEngineClient implementa o padrão Provider / Strategy para despachar requisições
// automaticamente para o Microsoft Edge TTS ou CapCut TTS conforme a voz solicitada.
type MultiEngineClient struct {
	edgeClient   *edgetts.Client
	capcutClient *capcuttts.Client
}

// NewMultiEngineClient constrói o roteador unificado de motores de voz.
func NewMultiEngineClient(edgeClient *edgetts.Client, capcutClient *capcuttts.Client) *MultiEngineClient {
	return &MultiEngineClient{
		edgeClient:   edgeClient,
		capcutClient: capcutClient,
	}
}

// Synthesize despacha dinamicamente para o motor correspondente com base no identificador da voz.
func (m *MultiEngineClient) Synthesize(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, error) {
	audio, _, _, err := m.SynthesizeWithSubtitles(ctx, opts)
	return audio, err
}

// SynthesizeWithSubtitles despacha a síntese e extrai legendas sincronizadas do motor correspondente.
func (m *MultiEngineClient) SynthesizeWithSubtitles(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, []subtitles.WordTimestamp, []subtitles.SentenceCue, error) {
	if m.capcutClient != nil && m.capcutClient.IsVoiceSupported(opts.Voice) {
		slog.Debug("Roteando síntese para o motor CapCut TTS",
			"voice", opts.Voice,
			"emotion", opts.Emotion,
			"emotion_scale", opts.EmotionScale)

		capOpts := capcuttts.SynthesizeOptions{
			Text:         opts.Text,
			Voice:        opts.Voice,
			Rate:         opts.Rate,
			Pitch:        opts.Pitch,
			Emotion:      opts.Emotion,
			EmotionScale: opts.EmotionScale,
			Style:        opts.Style,
			Role:         opts.Role,
			Format:       opts.Format,
		}

		audio, err := m.capcutClient.Synthesize(ctx, capOpts)
		return audio, nil, nil, err
	}

	slog.Debug("Roteando síntese para o motor Microsoft Edge TTS",
		"voice", opts.Voice)

	return m.edgeClient.SynthesizeWithSubtitles(ctx, opts)
}

// CapCutClient retorna a instância interna do cliente CapCut.
func (m *MultiEngineClient) CapCutClient() *capcuttts.Client {
	return m.capcutClient
}

// EdgeClient retorna a instância interna do cliente Edge TTS.
func (m *MultiEngineClient) EdgeClient() *edgetts.Client {
	return m.edgeClient
}

// IsCapCutVoice verifica se uma voz é de propriedade do catálogo CapCut.
func (m *MultiEngineClient) IsCapCutVoice(voice string) bool {
	if m.capcutClient == nil {
		return false
	}
	return m.capcutClient.IsVoiceSupported(voice)
}
