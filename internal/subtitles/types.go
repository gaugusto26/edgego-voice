package subtitles

// WordTimestamp representa o alinhamento temporal de uma palavra individual.
type WordTimestamp struct {
	Word    string `json:"word"`
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
}

// SentenceCue representa uma legenda em nível de frase/sentença.
type SentenceCue struct {
	Index   int             `json:"index"`
	Text    string          `json:"text"`
	StartMs int64           `json:"start_ms"`
	EndMs   int64           `json:"end_ms"`
	Words   []WordTimestamp `json:"words,omitempty"`
}

// SubtitleResult encapsula as legendas completas e os formatos pré-renderizados.
type SubtitleResult struct {
	Text       string          `json:"text"`
	DurationMs int64           `json:"duration_ms"`
	Sentences  []SentenceCue   `json:"sentences"`
	Words      []WordTimestamp `json:"words,omitempty"`
	SRT        string          `json:"srt"`
	VTT        string          `json:"vtt"`
}
