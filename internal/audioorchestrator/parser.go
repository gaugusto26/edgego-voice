package audioorchestrator

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// reTag detecta pausas, vozes, velocidade, tom, efeitos sfx, ambiente, telefone, estilos e tags de emoção
	reTag = regexp.MustCompile(`(?i)(?:` +
		// 1, 2, 3: Pausas
		`\[(?:pausa|pause|break):\s*([0-9.]+(?:ms|s)?)\]|\((?:pausa|pause|break):\s*([0-9.]+(?:ms|s)?)\)|<break\s+time=["']?([0-9.]+(?:ms|s)?)["']?\s*\/?>|` +
		// 4, 5: Troca de Voz
		`\[(?:voz|voice):\s*([^\]]+)\]|<voice\s+name=["']?([^"'>]+)["']?\s*>|` +
		// 6: Velocidade
		`\[(?:velocidade|speed):\s*([0-9.]+(?:x)?)\]|` +
		// 7: Tom
		`\[(?:tom|pitch):\s*([+-]?[0-9]+(?:hz|%)?)\]|` +
		// 8, 9, 10, 11: SFX com Duração explícita: [som:teclado:5s] ou [teclado:5s]
		`\[(?:som|sfx|sound):\s*([^\]:]+)(?::\s*([0-9.]+(?:ms|s)?))?\]|\[(teclado|callcenter|ruido|ruído|ambiente):\s*([0-9.]+(?:ms|s)?)\]|` +
		// 12: Fechamento de Telefone ou Ambiente: [/telefone], [/callcenter], [/teclado], [/ambiente], [/ruido]
		`\[\/(telefone|callcenter|ambiente|escritorio|escritório|ruido|ruído|teclado)\]|` +
		// 13: Abertura de Telefone ou Ambiente: [telefone], [callcenter], [ambiente], [ruido], [teclado]
		`\[(telefone|callcenter|ambiente|escritorio|escritório|ruido|ruído|teclado)\]|` +
		// 14, 15, 16: Abertura de Estilo ou Emoção [estilo:cheerful], [emocao:feliz], [emocao:firme:1.5]
		`\[(?:estilo|style|emocao|emoção|emotion):\s*([^\]]+)\]|<mstts:express-as\s+style=["']?([^"'>]+)["']?\s*(?:styledegree=["']?([0-9.]+)["']?)?\s*>|` +
		// 17: Emoções diretas: [alegre], [triste], [feliz], [calmo], etc
		`\[(alegre|cheerful|triste|sad|bravo|irritado|angry|animado|empolgado|excited|calmo|calm|gritando|shouting|amigavel|amigável|friendly|esperancoso|esperançoso|hopeful|feliz|happy|serio|sério|firme|surpreso|surprise|tenso|medo|fear)\]|` +
		// Fechamento de Estilo ou Emoção
		`\[\/(?:estilo|style|emocao|emoção|emotion|alegre|cheerful|triste|sad|bravo|irritado|angry|animado|empolgado|excited|calmo|calm|gritando|shouting|amigavel|amigável|friendly|esperancoso|esperançoso|hopeful|feliz|happy|serio|sério|firme|surpreso|surprise|tenso|medo|fear)\]|<\/mstts:express-as>|` +
		// 18: Fillers conversacionais: [hum], [entendi], [certo], [deixa-ver]
		`\[(hum|hmm|entendi|certo|deixa-ver|filler:[^\]]+)\]` +
		`)`)
)

// normalizeEmotion mapeia apelidos em português e inglês para a emoção oficial do CapCut
// (happy, calm, angry, sad, surprise, fear, excited, neutral) e estilo do Azure/Edge TTS.
func normalizeEmotion(raw string) (emotion string, style string, scale string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ""
	}

	parts := strings.SplitN(raw, ":", 2)
	name := strings.ToLower(strings.TrimSpace(parts[0]))
	scale = "1.0"
	if len(parts) > 1 {
		customScale := strings.TrimSpace(parts[1])
		if customScale != "" {
			scale = customScale
		}
	}

	switch name {
	case "feliz", "happy", "alegre", "cheerful":
		return "happy", "cheerful", scale
	case "empolgado", "animado", "excited":
		return "excited", "excited", scale
	case "calmo", "calm", "suave", "tranquilo":
		return "calm", "calm", scale
	case "firme", "serio", "sério", "bravo", "irritado", "angry", "nervoso":
		return "angry", "angry", scale
	case "triste", "sad", "melancolico", "melancólico":
		return "sad", "sad", scale
	case "surpreso", "surpresa", "surprise":
		return "surprise", "excited", scale
	case "tenso", "medo", "fear":
		return "fear", "unfriendly", scale
	case "gritando", "shouting", "grito":
		return "angry", "shouting", scale
	case "amigavel", "amigável", "friendly":
		return "happy", "friendly", scale
	case "esperancoso", "esperançoso", "hopeful":
		return "happy", "hopeful", scale
	case "neutro", "neutral", "natural", "normal", "padrao", "padrão", "none", "off":
		return "neutral", "", "0"
	default:
		return name, name, scale
	}
}

// normalizeStyle converte apelidos e nomes em português para os estilos oficiais do Azure Speech
func normalizeStyle(s string) string {
	_, style, _ := normalizeEmotion(s)
	return style
}

// buildSpeechSegment cria um bloco de fala com roteamento de provedor, trilha ambiente, filtro de telefonia, estilo e emoção.
func buildSpeechSegment(text, voice, style, emotion, emotionScale, rate, pitch, ambient string, styleDegree float64, telephony bool) Segment {
	return Segment{
		Type:            SegmentSpeech,
		Provider:        ProviderEdge,
		Text:            text,
		Voice:           voice,
		Style:           style,
		StyleDegree:     styleDegree,
		Emotion:         emotion,
		EmotionScale:    emotionScale,
		Rate:            rate,
		Pitch:           pitch,
		Volume:          "+0%",
		AmbientTrack:    ambient,
		TelephonyFilter: telephony,
	}
}

// ParseSegments divide o texto em blocos sequenciais de fala, silêncio e efeitos sonoros com roteamento de provedor.
func ParseSegments(text, defaultVoice, defaultRate, defaultPitch string, voiceMap map[string]string) []Segment {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	matches := reTag.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return []Segment{
			buildSpeechSegment(text, defaultVoice, "", "", "", defaultRate, defaultPitch, "", 1.0, false),
		}
	}

	var segments []Segment
	currVoice := defaultVoice
	currRate := defaultRate
	currPitch := defaultPitch
	currStyle := ""
	currStyleDegree := 1.0
	currEmotion := ""
	currEmotionScale := ""
	currAmbient := ""
	currTelephony := false
	lastIdx := 0

	for _, matchIdx := range matches {
		startTag, endTag := matchIdx[0], matchIdx[1]

		// 1. Texto anterior à tag (se houver)
		if startTag > lastIdx {
			prevText := strings.TrimSpace(text[lastIdx:startTag])
			if prevText != "" {
				segments = append(segments, buildSpeechSegment(prevText, currVoice, currStyle, currEmotion, currEmotionScale, currRate, currPitch, currAmbient, currStyleDegree, currTelephony))
			}
		}

		fullTag := text[startTag:endTag]
		submatches := reTag.FindStringSubmatch(fullTag)

		// 2. Avaliar tipo da tag
		if len(submatches) > 1 {
			if submatches[1] != "" || submatches[2] != "" || submatches[3] != "" {
				// Pausa / Silêncio
				pauseVal := submatches[1]
				if pauseVal == "" {
					pauseVal = submatches[2]
				}
				if pauseVal == "" {
					pauseVal = submatches[3]
				}
				dur := parseDuration(pauseVal)
				if dur > 0 {
					segments = append(segments, Segment{
						Type:         SegmentSilence,
						Provider:     ProviderSilence,
						Voice:        currVoice,
						Duration:     dur,
						AmbientTrack: currAmbient,
					})
				}
			} else if submatches[4] != "" || submatches[5] != "" {
				// Troca de Voz
				voiceVal := strings.TrimSpace(submatches[4])
				if voiceVal == "" {
					voiceVal = strings.TrimSpace(submatches[5])
				}
				resolvedVoice := resolveVoiceName(voiceVal, voiceMap, defaultVoice)
				currVoice = resolvedVoice
			} else if submatches[6] != "" {
				// Velocidade
				speedVal := strings.TrimSpace(submatches[6])
				currRate = formatRateString(speedVal)
			} else if submatches[7] != "" {
				// Tom (Pitch)
				pitchVal := strings.TrimSpace(submatches[7])
				currPitch = formatPitchString(pitchVal)
			} else if submatches[8] != "" {
				// SFX formato [som:efeito:5s] ou [som:efeito]
				sfxName := strings.TrimSpace(submatches[8])
				var dur time.Duration
				if submatches[9] != "" {
					dur = parseDuration(submatches[9])
				}
				segments = append(segments, Segment{
					Type:            SegmentSFX,
					Provider:        ProviderSFX,
					SFXType:         sfxName,
					Duration:        dur,
					AmbientTrack:    currAmbient,
					TelephonyFilter: currTelephony,
				})
			} else if submatches[10] != "" {
				// SFX com duração explícita [teclado:5s], [callcenter:3s]
				sfxName := strings.TrimSpace(submatches[10])
				dur := parseDuration(submatches[11])
				segments = append(segments, Segment{
					Type:            SegmentSFX,
					Provider:        ProviderSFX,
					SFXType:         sfxName,
					Duration:        dur,
					AmbientTrack:    currAmbient,
					TelephonyFilter: currTelephony,
				})
			} else if submatches[12] != "" {
				// Fechamento de Telefone ou Ambiente
				closed := strings.ToLower(strings.TrimSpace(submatches[12]))
				if closed == "telefone" {
					currTelephony = false
				} else {
					currAmbient = ""
				}
			} else if submatches[13] != "" {
				// Abertura de Telefone ou Ambiente
				ambName := strings.TrimSpace(submatches[13])
				if strings.EqualFold(ambName, "telefone") {
					currTelephony = true
				} else {
					closingTag := "[/" + strings.ToLower(ambName) + "]"
					// Se o texto tiver a tag de fechamento correspondente adiante, ativa como ambiente envolvente
					if strings.Contains(strings.ToLower(text[endTag:]), closingTag) {
						currAmbient = ambName
					} else {
						// Caso não haja fechamento, executa como SFX pontual
						segments = append(segments, Segment{
							Type:            SegmentSFX,
							Provider:        ProviderSFX,
							SFXType:         ambName,
							AmbientTrack:    currAmbient,
							TelephonyFilter: currTelephony,
						})
					}
				}
			} else if submatches[14] != "" || submatches[15] != "" {
				// Abertura de Estilo ou Emoção: [estilo:cheerful], [emocao:feliz], [emocao:firme:1.5]
				tagVal := strings.TrimSpace(submatches[14])
				if tagVal == "" {
					tagVal = strings.TrimSpace(submatches[15])
				}
				em, st, sc := normalizeEmotion(tagVal)
				currEmotion = em
				currEmotionScale = sc
				currStyle = st
				if len(submatches) > 16 && submatches[16] != "" {
					if deg, err := strconv.ParseFloat(submatches[16], 64); err == nil && deg > 0 {
						currStyleDegree = deg
						currEmotionScale = submatches[16]
					}
				} else if deg, err := strconv.ParseFloat(sc, 64); err == nil && deg > 0 {
					currStyleDegree = deg
				}
			} else if submatches[17] != "" {
				// Emoção direta [alegre], [triste], [feliz], [calmo], etc
				em, st, sc := normalizeEmotion(submatches[17])
				currEmotion = em
				currEmotionScale = sc
				currStyle = st
				currStyleDegree = 1.0
			} else if submatches[18] != "" {
				// Filler conversacional [hum], [entendi], [certo], [deixa-ver]
				segments = append(segments, Segment{
					Type:            SegmentSFX,
					Provider:        ProviderSFX,
					SFXType:         strings.TrimSpace(submatches[18]),
					AmbientTrack:    currAmbient,
					TelephonyFilter: currTelephony,
				})
			} else {
				// Fechamento de estilo ou emoção [/estilo], [/emocao], [/alegre], etc
				currStyle = ""
				currStyleDegree = 1.0
				currEmotion = ""
				currEmotionScale = ""
			}
		}

		lastIdx = endTag
	}

	// 3. Texto restante após a última tag
	if lastIdx < len(text) {
		remaining := strings.TrimSpace(text[lastIdx:])
		if remaining != "" {
			segments = append(segments, buildSpeechSegment(remaining, currVoice, currStyle, currEmotion, currEmotionScale, currRate, currPitch, currAmbient, currStyleDegree, currTelephony))
		}
	}

	// 4. Limpar e normalizar pontuações soltas nas fronteiras de segmentos
	return cleanSegmentBoundaries(segments)
}

// cleanSegmentBoundaries garante que vírgulas e pontuações no início de um segmento pertençam ao segmento anterior
func cleanSegmentBoundaries(segments []Segment) []Segment {
	if len(segments) <= 1 {
		return segments
	}

	var cleaned []Segment
	for i := 0; i < len(segments); i++ {
		seg := segments[i]
		if seg.Type == SegmentSpeech {
			trimmed := strings.TrimSpace(seg.Text)
			for len(trimmed) > 0 && (trimmed[0] == ',' || trimmed[0] == ';' || trimmed[0] == ':') {
				punct := string(trimmed[0])
				trimmed = strings.TrimSpace(trimmed[1:])
				if len(cleaned) > 0 && cleaned[len(cleaned)-1].Type == SegmentSpeech {
					cleaned[len(cleaned)-1].Text += punct
				}
			}
			seg.Text = trimmed
		}
		if seg.Type != SegmentSpeech || seg.Text != "" {
			cleaned = append(cleaned, seg)
		}
	}
	return cleaned
}

// parseDuration converte valores como "2s", "500ms", "1.5s", "1000" para time.Duration.
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}

	if strings.HasSuffix(s, "ms") {
		numStr := strings.TrimSuffix(s, "ms")
		n, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return time.Duration(n * float64(time.Millisecond))
	}

	if strings.HasSuffix(s, "s") {
		numStr := strings.TrimSuffix(s, "s")
		n, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return time.Duration(n * float64(time.Second))
	}

	// Se não tiver sufixo, assume milissegundos se > 50 ou segundos se <= 10
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	if n <= 10 {
		return time.Duration(n * float64(time.Second))
	}
	return time.Duration(n * float64(time.Millisecond))
}

// knownCommonNeuralVoices mapeia todos os nomes de vozes para seus identificadores neurais oficiais
var knownCommonNeuralVoices = map[string]string{
	// 🇧🇷 Português (Brasil)
	"thalita":   "pt-BR-ThalitaMultilingualNeural",
	"antonio":   "pt-BR-AntonioNeural",
	"francisca": "pt-BR-FranciscaNeural",
	"fabio":     "pt-BR-FabioNeural",

	// 🇵🇹 Português (Portugal)
	"duarte": "pt-PT-DuarteNeural",
	"raquel": "pt-PT-RaquelNeural",

	// 🇺🇸 Inglês (EUA)
	"andrew":      "en-US-AndrewMultilingualNeural",
	"emma":        "en-US-EmmaMultilingualNeural",
	"brian":       "en-US-BrianMultilingualNeural",
	"ava":         "en-US-AvaMultilingualNeural",
	"guy":         "en-US-GuyNeural",
	"jenny":       "en-US-JennyNeural",
	"aria":        "en-US-AriaNeural",
	"christopher": "en-US-ChristopherNeural",

	// 🇬🇧 Inglês (Reino Unido)
	"sonia":  "en-GB-SoniaNeural",
	"ryan":   "en-GB-RyanNeural",
	"maisie": "en-GB-MaisieNeural",

	// 🇪🇸 Espanhol (Espanha & México)
	"alvaro": "es-ES-AlvaroNeural",
	"elvira": "es-ES-ElviraNeural",
	"ximena": "es-ES-XimenaNeural",
	"dalia":  "es-MX-DaliaNeural",
	"jorge":  "es-MX-JorgeNeural",

	// 🇫🇷 Francês (França)
	"denise":   "fr-FR-DeniseNeural",
	"henri":    "fr-FR-HenriNeural",
	"vivienne": "fr-FR-VivienneMultilingualNeural",
	"remy":     "fr-FR-RemyMultilingualNeural",

	// 🇮🇹 Italiano (Itália)
	"diego":    "it-IT-DiegoNeural",
	"elsa":     "it-IT-ElsaNeural",
	"giuseppe": "it-IT-GiuseppeMultilingualNeural",

	// 🇩🇪 Alemão (Alemanha)
	"katja":     "de-DE-KatjaNeural",
	"killian":   "de-DE-KillianNeural",
	"seraphina": "de-DE-SeraphinaMultilingualNeural",
	"florian":   "de-DE-FlorianMultilingualNeural",

	// 🇯🇵 Japonês (Japão)
	"nanami": "ja-JP-NanamiNeural",
	"keita":  "ja-JP-KeitaNeural",
}

// stripAccents normaliza caracteres acentuados para facilitar o matching de nomes
func stripAccents(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "õ", "o", "ô", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	)
	return replacer.Replace(strings.ToLower(s))
}

// resolveVoiceName mapeia apelidos como "Antonio", "Antônio", "Thalita", "Francisca" para os IDs neurais completos.
func resolveVoiceName(name string, voiceMap map[string]string, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}

	// Se já for um ID de voz completo (ex: pt-BR-AntonioNeural)
	if strings.Contains(name, "-") && strings.HasSuffix(name, "Neural") {
		return name
	}

	cleanName := stripAccents(name)

	// 1. Procurar na tabela de vozes neurais diretas
	if fullID, ok := knownCommonNeuralVoices[cleanName]; ok {
		return fullID
	}

	// 2. Procurar no mapa de vozes do servidor (alias -> fullId)
	for alias, fullID := range voiceMap {
		if strings.EqualFold(alias, name) || strings.EqualFold(fullID, name) {
			return fullID
		}
		cleanFull := stripAccents(fullID)
		cleanAlias := stripAccents(alias)
		if cleanAlias == cleanName || strings.Contains(cleanFull, cleanName) {
			return fullID
		}
	}

	return fallback
}

func formatRateString(speedVal string) string {
	speedVal = strings.TrimSuffix(strings.TrimSpace(speedVal), "x")
	val, err := strconv.ParseFloat(speedVal, 64)
	if err != nil || val <= 0 {
		return "+0%"
	}
	percentage := int(math.Round((val - 1.0) * 100))
	if percentage >= 0 {
		return "+" + strconv.Itoa(percentage) + "%"
	}
	return strconv.Itoa(percentage) + "%"
}

func formatPitchString(pitchVal string) string {
	pitchVal = strings.TrimSpace(pitchVal)
	if pitchVal == "" || pitchVal == "0" || pitchVal == "0Hz" {
		return "+0Hz"
	}
	if strings.HasSuffix(pitchVal, "Hz") || strings.HasSuffix(pitchVal, "%") {
		if !strings.HasPrefix(pitchVal, "+") && !strings.HasPrefix(pitchVal, "-") {
			return "+" + pitchVal
		}
		return pitchVal
	}
	if !strings.HasPrefix(pitchVal, "+") && !strings.HasPrefix(pitchVal, "-") {
		return "+" + pitchVal + "Hz"
	}
	return pitchVal + "Hz"
}
