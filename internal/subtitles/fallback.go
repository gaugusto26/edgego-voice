package subtitles

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// reSentenceSplit divide sentenças preservando pontuações terminais e tratando reticências
	reSentenceSplit = regexp.MustCompile(`(?:\.{3}|[.!?]+|\n+)(?:["'»)]*)\s*`)
	// reInnerComma detecta vírgulas e dois-pontos internos que produzem micropausas
	reInnerComma = regexp.MustCompile(`[,:;]\s+`)
)

// GenerateFallbackCues divide o texto em sentenças e calcula timestamps calibrados
// combinando o peso fonético de caracteres e os delays acústicos de vírgulas e pontuações finais.
func GenerateFallbackCues(text string, durationMs int64) []SentenceCue {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	if durationMs <= 0 {
		wordsCount := len(strings.Fields(text))
		durationMs = int64(wordsCount * 320)
		if durationMs < 600 {
			durationMs = 600
		}
	}

	rawSentences := splitIntoSentences(text)
	if len(rawSentences) == 0 {
		return []SentenceCue{
			{
				Index:   1,
				Text:    text,
				StartMs: 0,
				EndMs:   durationMs,
			},
		}
	}

	n := len(rawSentences)
	if n == 1 {
		return []SentenceCue{
			{
				Index:   1,
				Text:    rawSentences[0],
				StartMs: 0,
				EndMs:   durationMs,
			},
		}
	}

	// 1. Calcular o peso acústico de cada sentença:
	// - Cada caractere imprimível representa ~1 unidade fonética
	// - Cada vírgula/dois-pontos adiciona ~4 unidades (equivale a ~200ms de pausa)
	// - Cada pontuação terminal (. ! ?) adiciona ~8 unidades (~400ms de pausa pós-sentença)
	// - Reticências (...) adicionam ~12 unidades (~600ms de pausa)
	// - Peso mínimo por sentença = 12 unidades (garante que palavras isoladas como "Olá!" tenham tempo natural de fala)
	weights := make([]float64, n)
	totalWeight := 0.0

	for i, s := range rawSentences {
		charCount := 0
		for _, r := range s {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				charCount++
			}
		}

		commaCount := len(reInnerComma.FindAllString(s, -1))
		isEllipsis := strings.Contains(s, "...") || strings.Contains(s, "…")
		hasTerminalPunct := strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?")

		// Cálculo do peso proporcional
		w := float64(charCount) * 1.0
		w += float64(commaCount) * 4.0

		if isEllipsis {
			w += 10.0
		} else if hasTerminalPunct {
			w += 7.0
		}

		// Piso mínimo: mesmo sentenças curtas precisam de tempo mínimo de articulação
		if w < 14.0 {
			w = 14.0
		}

		weights[i] = w
		totalWeight += w
	}

	if totalWeight <= 0 {
		totalWeight = 1.0
	}

	// 2. Distribuir a duração total garantindo transição sem gaps descompensados
	cues := make([]SentenceCue, n)
	var currentMs int64 = 0

	for i, s := range rawSentences {
		// Duração proporcional calculada
		ratio := weights[i] / totalWeight
		cueDuration := int64(float64(durationMs) * ratio)

		// Garantir duração mínima de 500ms
		if cueDuration < 500 {
			cueDuration = 500
		}

		endMs := currentMs + cueDuration
		if i == n-1 || endMs > durationMs {
			endMs = durationMs
		}

		cues[i] = SentenceCue{
			Index:   i + 1,
			Text:    s,
			StartMs: currentMs,
			EndMs:   endMs,
		}

		currentMs = endMs
	}

	// Garantir que a última sentença feche exatamente no fim do áudio
	if n > 0 && cues[n-1].EndMs < durationMs {
		cues[n-1].EndMs = durationMs
	}

	return cues
}

// splitIntoSentences divide o texto respeitando pontuações (. ! ? \n) sem quebrar números decimais.
func splitIntoSentences(text string) []string {
	var sentences []string
	var sb strings.Builder

	runes := []rune(text)
	n := len(runes)

	for i := 0; i < n; i++ {
		r := runes[i]
		sb.WriteRune(r)

		// Verifica se é ponto final, exclamação, interrogação ou quebra de linha
		if r == '.' || r == '!' || r == '?' || r == '\n' {
			// Ignorar números decimais: ex "3.14" ou "v1.0"
			if r == '.' && i > 0 && i+1 < n && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
				continue
			}

			// Se for reticências ("..."), consome os pontos seguintes para não quebrar no meio
			for i+1 < n && runes[i+1] == '.' {
				i++
				sb.WriteRune(runes[i])
			}

			// Se o próximo caractere for aspa ou fechamento de parêntese, inclui na sentença
			for i+1 < n && (runes[i+1] == '"' || runes[i+1] == '\'' || runes[i+1] == '»' || runes[i+1] == ')') {
				i++
				sb.WriteRune(runes[i])
			}

			s := strings.TrimSpace(sb.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			sb.Reset()
		}
	}

	remaining := strings.TrimSpace(sb.String())
	if remaining != "" {
		sentences = append(sentences, remaining)
	}

	return sentences
}
