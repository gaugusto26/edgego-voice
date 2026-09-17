package audioorchestrator

import (
	"strings"
	"sync"
	"unicode"
)

// TagAwareChunker divide um fluxo contínuo de tokens de LLM em sentenças completas para síntese TTS,
// garantindo rigorosamente que tags de SFX ([teclado:3s]), pausas ([pausa: 500ms]), envelopes ([callcenter])
// e filtros ([telefone]) nunca sejam fragmentados ou quebrados no meio.
type TagAwareChunker struct {
	mu            sync.Mutex
	builder       strings.Builder
	inSquareTag   bool   // Rastreia se estamos dentro de [...]
	inAngleTag    bool   // Rastreia se estamos dentro de <...>
	activeAmbient string // Preserva envelopes de ambiente ativos entre sentenças
	inTelephony   bool   // Preserva modo telefone ativo entre sentenças
}

// NewTagAwareChunker cria um novo divisor de fluxo consciente de tags e pontuação.
func NewTagAwareChunker() *TagAwareChunker {
	return &TagAwareChunker{}
}

// Feed processa um novo token de texto do LLM e retorna sentenças prontas para síntese imediata (se houver).
func (c *TagAwareChunker) Feed(token string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var readyChunks []string

	for i := 0; i < len(token); i++ {
		ch := token[i]
		c.builder.WriteByte(ch)

		// Rastrear estado de abertura/fechamento de tags
		if ch == '[' {
			c.inSquareTag = true
			continue
		} else if ch == ']' {
			c.inSquareTag = false
			c.checkTagEnvelope()
			continue
		} else if ch == '<' {
			c.inAngleTag = true
			continue
		} else if ch == '>' {
			c.inAngleTag = false
			continue
		}

		// Se estivermos dentro de uma tag, NUNCA dividimos o chunk!
		if c.inSquareTag || c.inAngleTag {
			continue
		}

		// Critério de pontuação terminal forte (., !, ?, quebras de linha duplas)
		if ch == '.' || ch == '!' || ch == '?' || ch == '\n' {
			// Verificar se há conteúdo textual suficiente para uma sentença natural
			content := c.builder.String()
			clean := strings.TrimSpace(content)
			wordCount := countWords(clean)

			// Só emite se tiver pelo menos 2 palavras ou pontuação explícita de fim de frase
			if wordCount >= 1 && (ch != '.' || i == len(token)-1 || isBoundaryChar(token, i+1)) {
				chunk := c.finalizeChunk(content)
				if chunk != "" {
					readyChunks = append(readyChunks, chunk)
				}
				c.builder.Reset()
			}
		} else if ch == ',' || ch == ';' || ch == ':' {
			// Pontuação intermediária: só divide se já acumulou 6 ou mais palavras
			content := c.builder.String()
			wordCount := countWords(strings.TrimSpace(content))
			if wordCount >= 6 {
				chunk := c.finalizeChunk(content)
				if chunk != "" {
					readyChunks = append(readyChunks, chunk)
				}
				c.builder.Reset()
			}
		}
	}

	return readyChunks
}

// Flush esvazia o buffer restante ao final do fluxo do LLM.
func (c *TagAwareChunker) Flush() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	content := strings.TrimSpace(c.builder.String())
	c.builder.Reset()

	if content == "" {
		return nil
	}

	chunk := c.finalizeChunk(content)
	if chunk == "" {
		return nil
	}
	return []string{chunk}
}

// Reset reinicializa o chunker para uma nova conexão.
func (c *TagAwareChunker) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.builder.Reset()
	c.inSquareTag = false
	c.inAngleTag = false
	c.activeAmbient = ""
	c.inTelephony = false
}

// checkTagEnvelope inspeciona o buffer recente para atualizar envelopes de ambiente e telefone ativos.
func (c *TagAwareChunker) checkTagEnvelope() {
	cur := strings.ToLower(c.builder.String())

	// Rastrear ambiente
	if strings.Contains(cur, "[callcenter]") {
		c.activeAmbient = "callcenter"
	} else if strings.Contains(cur, "[/callcenter]") {
		c.activeAmbient = ""
	}

	if strings.Contains(cur, "[ruido]") || strings.Contains(cur, "[ruído]") {
		c.activeAmbient = "ruido"
	} else if strings.Contains(cur, "[/ruido]") || strings.Contains(cur, "[/ruído]") {
		c.activeAmbient = ""
	}

	// Rastrear telefone
	if strings.Contains(cur, "[telefone]") {
		c.inTelephony = true
	} else if strings.Contains(cur, "[/telefone]") {
		c.inTelephony = false
	}
}

// finalizeChunk formata a sentença emitida garantindo que envelopes abertos não sejam perdidos.
func (c *TagAwareChunker) finalizeChunk(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !hasSpokenOrSFXContent(trimmed) {
		return ""
	}

	// Se o chunk não tiver a tag de abertura do envelope ativo, injeta para preservar a mixagem
	result := trimmed
	if c.activeAmbient != "" && !strings.Contains(strings.ToLower(result), "["+c.activeAmbient) {
		result = "[" + c.activeAmbient + "]" + result
	}
	if c.inTelephony && !strings.Contains(strings.ToLower(result), "[telefone]") {
		result = "[telefone]" + result
	}

	return result
}

func hasSpokenOrSFXContent(s string) bool {
	if countWords(s) > 0 {
		return true
	}
	lower := strings.ToLower(s)
	return strings.Contains(lower, "[som:") || strings.Contains(lower, "[sfx:") ||
		strings.Contains(lower, "[teclado") ||
		strings.Contains(lower, "[hum") || strings.Contains(lower, "[entendi")
}

func countWords(s string) int {
	words := 0
	inWord := false
	inTag := false

	for _, r := range s {
		if r == '[' || r == '<' {
			inTag = true
			inWord = false
			continue
		}
		if r == ']' || r == '>' {
			inTag = false
			inWord = false
			continue
		}
		if inTag {
			continue
		}

		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				words++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return words
}

func isBoundaryChar(s string, idx int) bool {
	if idx >= len(s) {
		return true
	}
	return s[idx] == ' ' || s[idx] == '\n' || s[idx] == '\t'
}
