package audioorchestrator

import "time"

// SegmentType representa a categoria do segmento.
type SegmentType int

const (
	// SegmentSpeech representa um trecho de fala a ser sintetizado.
	SegmentSpeech SegmentType = iota
	// SegmentSilence representa um intervalo de silêncio real a ser inserido.
	SegmentSilence
	// SegmentSFX representa um micro-efeito sonoro biológico (pigarro, tosse, riso, etc).
	SegmentSFX
)

// SegmentProvider indica qual provedor/motor deve sintetizar este segmento.
type SegmentProvider string

const (
	ProviderEdge    SegmentProvider = "edge"    // Edge TTS Gratuito ($0)
	ProviderSFX     SegmentProvider = "sfx"     // SFX Local ($0)
	ProviderSilence SegmentProvider = "silence" // Gerador de Silêncio Local ($0)
)

// Segment representa um bloco sequencial de fala, efeito ou silêncio.
type Segment struct {
	Type        SegmentType
	Provider    SegmentProvider
	Text        string
	Voice       string
	Style       string
	StyleDegree float64
	Emotion     string
	EmotionScale string
	SFXType     string
	Rate         string
	Pitch        string
	Volume          string
	AmbientTrack    string
	Duration        time.Duration
	TelephonyFilter bool
}
