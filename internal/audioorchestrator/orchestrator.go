package audioorchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"edgego-voice/internal/edgetts"
	"edgego-voice/internal/sfx"
	"edgego-voice/internal/subtitles"
	"edgego-voice/internal/tts"
)

// SynthesisMetrics sumariza o consumo de caracteres e efeitos nesta execução.
type SynthesisMetrics struct {
	EdgeChars  int
	SFXCount   int
	DurationMs int64
}

// EstimateAudioDurationMs calcula ou estima a duração do áudio em milissegundos.
func EstimateAudioDurationMs(audio []byte, format string) int64 {
	if len(audio) == 0 {
		return 0
	}
	var dur int64
	switch strings.ToLower(format) {
	case "wav":
		// WAV PCM 24kHz 16-bit mono = 48.000 bytes/s = 48 bytes/ms
		dataLen := len(audio)
		if dataLen > 44 {
			dataLen -= 44
		}
		dur = int64(dataLen) / 48
	case "opus", "webm":
		// Opus ~24-32kbps (~3.5 bytes/ms)
		dur = int64(len(audio)) * 1000 / 3500
	default: // mp3
		// MP3 24kHz 48kbps = 6.000 bytes/s = 6 bytes/ms
		dur = int64(len(audio)) * 1000 / 6000
	}
	if dur <= 0 && len(audio) > 0 {
		dur = 100
	}
	return dur
}

// SynthesizeOrchestrated executa a síntese orquestrada delegando para SynthesizeOrchestratedWithSubtitles.
func SynthesizeOrchestrated(
	ctx context.Context,
	synthesizer tts.Synthesizer,
	text string,
	defaultOpts edgetts.SynthesizeOptions,
	voiceMap map[string]string,
) ([]byte, SynthesisMetrics, error) {
	audio, _, metrics, err := SynthesizeOrchestratedWithSubtitles(ctx, synthesizer, text, defaultOpts, voiceMap)
	return audio, metrics, err
}

// SynthesizeOrchestratedWithSubtitles executa a síntese de múltiplos blocos e gera legendas perfeitamente sincronizadas.
func SynthesizeOrchestratedWithSubtitles(
	ctx context.Context,
	synthesizer tts.Synthesizer,
	text string,
	defaultOpts edgetts.SynthesizeOptions,
	voiceMap map[string]string,
) ([]byte, subtitles.SubtitleResult, SynthesisMetrics, error) {
	rawSegments := ParseSegments(text, defaultOpts.Voice, defaultOpts.Rate, defaultOpts.Pitch, voiceMap)
	if len(rawSegments) == 0 {
		return nil, subtitles.SubtitleResult{}, SynthesisMetrics{}, fmt.Errorf("nenhum segmento de áudio para sintetizar")
	}

	segments := optimizeAndMergeSegments(rawSegments)
	metrics := SynthesisMetrics{}

	// Caso otimizado: texto simples sem pausas extras, sem SFX e sem som ambiente
	if len(segments) == 1 && segments[0].Type == SegmentSpeech && segments[0].AmbientTrack == "" {
		opts := defaultOpts
		opts.Text = segments[0].Text
		opts.Voice = segments[0].Voice
		opts.Rate = segments[0].Rate
		opts.Pitch = segments[0].Pitch
		if segments[0].Emotion != "" {
			opts.Emotion = segments[0].Emotion
			opts.EmotionScale = segments[0].EmotionScale
		}
		if segments[0].Style != "" {
			opts.Style = segments[0].Style
		}

		var audio []byte
		var words []subtitles.WordTimestamp
		var sents []subtitles.SentenceCue
		var err error

		if subSynth, ok := synthesizer.(tts.SubtitleSynthesizer); ok {
			audio, words, sents, err = subSynth.SynthesizeWithSubtitles(ctx, opts)
		} else {
			audio, err = synthesizer.Synthesize(ctx, opts)
		}

		if err == nil {
			audio = StripID3(audio)
			metrics.EdgeChars = utf8.RuneCountInString(segments[0].Text)
			if segments[0].TelephonyFilter || defaultOpts.TelephonyFilter {
				if filtered, fErr := sfx.ApplyTelephonyFilter(audio, defaultOpts.Format); fErr == nil && len(filtered) > 0 {
					audio = filtered
				}
			}

			// Calcula a duração física real dos frames de áudio MP3/WAV
			exactDur := ExactAudioDurationMs(audio, defaultOpts.Format)
			var durMs int64
			if len(sents) > 0 {
				// Timestamps do Edge TTS são precisos
				durMs = sents[len(sents)-1].EndMs
				if exactDur > durMs {
					durMs = exactDur
				} else {
					durMs += 80
				}
			} else {
				durMs = exactDur
				if durMs < 200 {
					wordsCount := len(strings.Fields(segments[0].Text))
					estimatedByWords := int64(wordsCount * 350)
					if estimatedByWords < 500 {
						estimatedByWords = 500
					}
					if durMs < estimatedByWords {
						durMs = estimatedByWords
					}
				}
				sents = subtitles.GenerateFallbackCues(segments[0].Text, durMs)
			}
			metrics.DurationMs = durMs

			subResult := subtitles.SubtitleResult{
				Text:       segments[0].Text,
				DurationMs: durMs,
				Sentences:  sents,
				Words:      words,
				SRT:        subtitles.FormatSRT(sents),
				VTT:        subtitles.FormatVTT(sents),
			}
			return audio, subResult, metrics, nil
		}
		return audio, subtitles.SubtitleResult{}, metrics, err
	}

	type segmentResult struct {
		index     int
		data      []byte
		edgeChars int
		words     []subtitles.WordTimestamp
		sentences []subtitles.SentenceCue
		err       error
	}

	results := make([][]byte, len(segments))
	segmentWords := make([][]subtitles.WordTimestamp, len(segments))
	segmentSents := make([][]subtitles.SentenceCue, len(segments))

	var wg sync.WaitGroup
	resultChan := make(chan segmentResult, len(segments))

	// Semáforo: limita a 2 conexões WebSocket simultâneas ao Edge TTS.
	// Sem limite, múltiplos segmentos paralelos saturam o endpoint e causam
	// "context deadline exceeded" por throttling do servidor remoto.
	const maxConcurrentSynth = 2
	synthSem := make(chan struct{}, maxConcurrentSynth)

	hasAnyExplicitEmotion := false
	hasAnyExplicitStyle := false
	for _, s := range segments {
		if s.Emotion != "" {
			hasAnyExplicitEmotion = true
		}
		if s.Style != "" {
			hasAnyExplicitStyle = true
		}
	}

	for i, seg := range segments {
		// 1. Silêncio real local (ou continuidade do som ambiente durante a pausa)
		if seg.Type == SegmentSilence {
			var silenceBytes []byte
			var err error
			if seg.AmbientTrack != "" {
				silenceBytes, err = sfx.GenerateDurationSFXWithVolume(seg.AmbientTrack, defaultOpts.Voice, defaultOpts.Format, seg.Duration, 0.45)
			} else {
				silVoice := seg.Voice
				if silVoice == "" {
					silVoice = defaultOpts.Voice
				}
				silenceBytes, err = GenerateSilenceForVoice(defaultOpts.Format, silVoice, seg.Duration)
			}
			if err != nil {
				return nil, subtitles.SubtitleResult{}, metrics, fmt.Errorf("falha ao gerar silêncio para segmento %d: %w", i, err)
			}
			results[i] = silenceBytes
			continue
		}

		// 2. Efeito Sonoro (SFX) - Carrega arquivo de áudio customizado de ./sfx ou sintetiza acústico puro
		if seg.Type == SegmentSFX {
			var sfxBytes []byte
			var err error
			if seg.Duration > 0 {
				sfxBytes, err = sfx.GenerateDurationSFX(seg.SFXType, defaultOpts.Voice, defaultOpts.Format, seg.Duration)
			} else {
				sfxBytes, err = sfx.GetAudioForSFX(seg.SFXType, defaultOpts.Voice, defaultOpts.Format)
			}
			if err != nil {
				return nil, subtitles.SubtitleResult{}, metrics, fmt.Errorf("falha ao carregar sfx %s: %w", seg.SFXType, err)
			}

			// Se houver uma trilha de ambiente ativa e diferente do SFX atual, mixa o som de ambiente sob o SFX!
			if seg.AmbientTrack != "" && !strings.EqualFold(seg.AmbientTrack, seg.SFXType) {
				if mixed, mixErr := sfx.MixAmbientTrack(sfxBytes, seg.AmbientTrack, defaultOpts.Voice, defaultOpts.Format, 0.45); mixErr == nil && len(mixed) > 0 {
					sfxBytes = mixed
				}
			}

			// Aplicar filtro de linha telefônica se ativo para o segmento ou globalmente
			if seg.TelephonyFilter || defaultOpts.TelephonyFilter {
				if filtered, fErr := sfx.ApplyTelephonyFilter(sfxBytes, defaultOpts.Format); fErr == nil && len(filtered) > 0 {
					sfxBytes = filtered
				}
			}

			results[i] = sfxBytes
			metrics.SFXCount++
			continue
		}

		// 3. Síntese de Fala Paralela via Edge TTS ou CapCut
		wg.Add(1)
		go func(idx int, s Segment) {
			defer wg.Done()

			// Adquire o slot do semáforo antes de abrir o WebSocket
			synthSem <- struct{}{}
			defer func() { <-synthSem }()

			charCount := utf8.RuneCountInString(s.Text)

			segOpts := defaultOpts
			segOpts.Text = s.Text
			segOpts.Voice = s.Voice
			segOpts.Rate = s.Rate
			segOpts.Pitch = s.Pitch
			segOpts.Volume = s.Volume
			segOpts.BreakComma = defaultOpts.BreakComma
			segOpts.BreakPeriod = defaultOpts.BreakPeriod

			if s.Emotion != "" {
				segOpts.Emotion = s.Emotion
				segOpts.EmotionScale = s.EmotionScale
			} else if hasAnyExplicitEmotion {
				segOpts.Emotion = ""
				segOpts.EmotionScale = "0"
			}

			if s.Style != "" {
				segOpts.Style = s.Style
			} else if hasAnyExplicitStyle {
				segOpts.Style = ""
			}

			var audioData []byte
			var words []subtitles.WordTimestamp
			var sents []subtitles.SentenceCue
			var err error

			if subSynth, ok := synthesizer.(tts.SubtitleSynthesizer); ok {
				audioData, words, sents, err = subSynth.SynthesizeWithSubtitles(ctx, segOpts)
			} else {
				audioData, err = synthesizer.Synthesize(ctx, segOpts)
			}

			if err == nil && len(audioData) > 0 {
				audioData = StripID3(audioData)

				// Se houver trilha de fundo, mixar som ambiente
				if s.AmbientTrack != "" {
					if mixed, mixErr := sfx.MixAmbientTrack(audioData, s.AmbientTrack, s.Voice, defaultOpts.Format, 0.45); mixErr == nil && len(mixed) > 0 {
						audioData = mixed
					}
				}
				// Se o segmento ou a chamada ativar linha telefônica, aplica o filtro DSP
				if s.TelephonyFilter || defaultOpts.TelephonyFilter {
					if filtered, fErr := sfx.ApplyTelephonyFilter(audioData, defaultOpts.Format); fErr == nil && len(filtered) > 0 {
						audioData = filtered
					}
				}
			}

			resultChan <- segmentResult{
				index:     idx,
				data:      audioData,
				edgeChars: charCount,
				words:     words,
				sentences: sents,
				err:       err,
			}
		}(i, seg)
	}

	wg.Wait()
	close(resultChan)

	// Coletar resultados preservando a ordem cronológica
	for res := range resultChan {
		if res.err != nil {
			return nil, subtitles.SubtitleResult{}, metrics, fmt.Errorf("falha na síntese do segmento %d: %w", res.index, res.err)
		}
		results[res.index] = res.data
		segmentWords[res.index] = res.words
		segmentSents[res.index] = res.sentences
		metrics.EdgeChars += res.edgeChars
	}

	// Calcular offsets acumulados de legendas e concatenar áudio
	var accumulatedOffsetMs int64 = 0
	var allSentences []subtitles.SentenceCue
	var allWords []subtitles.WordTimestamp

	for i, seg := range segments {
		segData := results[i]
		if seg.Type == SegmentSilence {
			durMs := seg.Duration.Milliseconds()
			accumulatedOffsetMs += durMs
			continue
		}
		if seg.Type == SegmentSFX {
			durMs := seg.Duration.Milliseconds()
			if durMs <= 0 {
				durMs = EstimateAudioDurationMs(segData, defaultOpts.Format)
			}
			accumulatedOffsetMs += durMs
			continue
		}

		// SegmentSpeech
		resWords := segmentWords[i]
		resSents := segmentSents[i]

		// Duração física real do chunk de áudio gerado
		realChunkDurMs := ExactAudioDurationMs(segData, defaultOpts.Format)

		var segDurMs int64
		if len(resSents) > 0 {
			// Edge TTS: timestamps nativos
			segDurMs = realChunkDurMs
			if segDurMs <= 0 {
				segDurMs = resSents[len(resSents)-1].EndMs + 80
			}
		} else {
			// CapCut TTS / Fallback: calibrado pela duração física real dos frames
			segDurMs = realChunkDurMs
			if segDurMs < 200 {
				wordsCount := len(strings.Fields(seg.Text))
				estimatedByWords := int64(wordsCount * 350)
				if estimatedByWords < 500 {
					estimatedByWords = 500
				}
				if segDurMs < estimatedByWords {
					segDurMs = estimatedByWords
				}
			}
			resSents = subtitles.GenerateFallbackCues(seg.Text, segDurMs)
		}

		for _, cue := range resSents {
			shifted := cue
			shifted.Index = len(allSentences) + 1
			shifted.StartMs += accumulatedOffsetMs
			shifted.EndMs += accumulatedOffsetMs
			allSentences = append(allSentences, shifted)
		}

		for _, w := range resWords {
			shifted := w
			shifted.StartMs += accumulatedOffsetMs
			shifted.EndMs += accumulatedOffsetMs
			allWords = append(allWords, shifted)
		}

		accumulatedOffsetMs += segDurMs
	}

	// Estimar tamanho total para pré-alocação de memória (golang-performance)
	totalLen := 0
	for _, chunk := range results {
		totalLen += len(chunk)
	}

	finalAudio := make([]byte, 0, totalLen)
	for _, chunk := range results {
		finalAudio = append(finalAudio, chunk...)
	}

	metrics.DurationMs = accumulatedOffsetMs

	subResult := subtitles.SubtitleResult{
		Text:       text,
		DurationMs: accumulatedOffsetMs,
		Sentences:  allSentences,
		Words:      allWords,
		SRT:        subtitles.FormatSRT(allSentences),
		VTT:        subtitles.FormatVTT(allSentences),
	}

	return finalAudio, subResult, metrics, nil
}

// optimizeAndMergeSegments mescla segmentos de fala adjacentes consecutivos de mesma voz para prosódia fluida.
func optimizeAndMergeSegments(segments []Segment) []Segment {
	if len(segments) == 0 {
		return segments
	}

	var merged []Segment
	for _, seg := range segments {
		if len(merged) == 0 {
			merged = append(merged, seg)
			continue
		}

		last := &merged[len(merged)-1]
		if last.Type == SegmentSpeech && seg.Type == SegmentSpeech &&
			last.Voice == seg.Voice &&
			last.Rate == seg.Rate &&
			last.Pitch == seg.Pitch &&
			last.Style == seg.Style &&
			last.StyleDegree == seg.StyleDegree &&
			last.Emotion == seg.Emotion &&
			last.EmotionScale == seg.EmotionScale &&
			last.AmbientTrack == seg.AmbientTrack &&
			last.TelephonyFilter == seg.TelephonyFilter {
			last.Text = strings.TrimSpace(last.Text) + " " + strings.TrimSpace(seg.Text)
		} else {
			merged = append(merged, seg)
		}
	}

	return merged
}
