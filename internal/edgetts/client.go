package edgetts

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"edgego-voice/internal/subtitles"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type metadataJSON struct {
	Metadata []struct {
		Type string `json:"Type"`
		Data struct {
			Offset   int64 `json:"Offset"`
			Duration int64 `json:"Duration"`
			Text     struct {
				Text         string `json:"Text"`
				Length       int    `json:"Length"`
				BoundaryType string `json:"BoundaryType"`
			} `json:"text"`
		} `json:"Data"`
	} `json:"Metadata"`
}

const (
	winEpoch           = 11644473600
	trustedClientToken = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	chromiumVersion    = "143.0.3650.75"
	secMSGECVersion    = "1-" + chromiumVersion
	edgeEndpoint       = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1"
	originHeader       = "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold"
	userAgentHeader    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 Edg/143.0.0.0"
)

// Client gerencia conexões de síntese no Microsoft Edge TTS com alta performance.
type Client struct {
	proxyURL *url.URL
	mu       sync.Mutex
}

// SynthesizeOptions define os parâmetros para a chamada de síntese.
type SynthesizeOptions struct {
	Text        string
	Voice       string
	Rate        string
	Pitch       string
	Volume      string
	Language    string
	Format      string
	BreakComma      string
	BreakPeriod     string
	TelephonyFilter bool
	AutoBreath      bool
	Emotion         string
	EmotionScale    string
	Style           string
	Role            string
}

// NewClient cria uma nova instância do cliente Edge TTS.
func NewClient(proxyStr string) *Client {
	c := &Client{}
	if proxyStr != "" {
		if u, err := url.Parse(proxyStr); err == nil {
			c.proxyURL = u
		}
	}
	return c
}

// extractLangFromVoice extrai o idioma a partir do identificador da voz (ex: "en-US" de "en-US-Andrew...").
func extractLangFromVoice(voice string) string {
	parts := strings.Split(voice, "-")
	if len(parts) >= 2 {
		return fmt.Sprintf("%s-%s", parts[0], parts[1])
	}
	return "pt-BR"
}

// generateSecMSGEC gera o token DRM baseado no timestamp Windows epoch e hash SHA256.
func generateSecMSGEC() string {
	ticks := time.Now().UTC().Unix()
	ticks += winEpoch
	ticks -= ticks % 300
	ticks *= 10000000 // intervalos de 100 nanossegundos

	strToHash := fmt.Sprintf("%d%s", ticks, trustedClientToken)
	hash := sha256.Sum256([]byte(strToHash))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

// generateMUID gera um ID aleatório para o cabeçalho Cookie.
func generateMUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

// dialConnection cria uma conexão WebSocket direta e otimizada com TCP_NODELAY.
func (c *Client) dialConnection(ctx context.Context) (*websocket.Conn, error) {
	connectionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	secMSGEC := generateSecMSGEC()

	wsURL := fmt.Sprintf(
		"%s?TrustedClientToken=%s&Sec-MS-GEC=%s&Sec-MS-GEC-Version=%s&ConnectionId=%s",
		edgeEndpoint, trustedClientToken, secMSGEC, secMSGECVersion, connectionID,
	)

	dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  10 * time.Second,
		EnableCompression: true,
		NetDialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	if c.proxyURL != nil {
		dialer.Proxy = http.ProxyURL(c.proxyURL)
	}

	headers := http.Header{}
	headers.Set("Origin", originHeader)
	headers.Set("User-Agent", userAgentHeader)
	headers.Set("Pragma", "no-cache")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Cookie", fmt.Sprintf("muid=%s", generateMUID()))

	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("falha ao conectar ao websocket Edge TTS (status %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("falha ao conectar ao websocket Edge TTS: %w", err)
	}

	return conn, nil
}

// Synthesize gera o áudio diretamente em memória (buffer de bytes).
func (c *Client) Synthesize(ctx context.Context, opts SynthesizeOptions) ([]byte, error) {
	audio, _, _, err := c.SynthesizeWithSubtitles(ctx, opts)
	return audio, err
}

// SynthesizeWithSubtitles gera o áudio e coleta as legendas de palavras e sentenças sincronizadas.
func (c *Client) SynthesizeWithSubtitles(ctx context.Context, opts SynthesizeOptions) ([]byte, []subtitles.WordTimestamp, []subtitles.SentenceCue, error) {
	var buf bytes.Buffer
	words, sentences, err := c.SynthesizeStreamWithSubtitles(ctx, opts, &buf)
	if err != nil {
		return nil, nil, nil, err
	}
	return buf.Bytes(), words, sentences, nil
}

// SynthesizeStream gera o áudio completo e íntegro com streaming em tempo real.
func (c *Client) SynthesizeStream(ctx context.Context, opts SynthesizeOptions, writer io.Writer) error {
	_, _, err := c.SynthesizeStreamWithSubtitles(ctx, opts, writer)
	return err
}

// SynthesizeStreamWithSubtitles gera o áudio em tempo real e retorna os metadados de legendas.
func (c *Client) SynthesizeStreamWithSubtitles(ctx context.Context, opts SynthesizeOptions, writer io.Writer) ([]subtitles.WordTimestamp, []subtitles.SentenceCue, error) {
	conn, err := c.dialConnection(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	// Garantir fechamento imediato se o contexto for cancelado
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	var words []subtitles.WordTimestamp
	var sentences []subtitles.SentenceCue

	requestID := strings.ReplaceAll(uuid.New().String(), "-", "")
	formatInfo := GetFormatInfo(opts.Format)
	timestampStr := time.Now().UTC().Format("Mon Jan 02 2006 15:04:05 GMT-0700 (Coordinated Universal Time)")

	// 1. Enviar Speech Config com metadados de limites de palavras e sentenças ativados
	configMessage := fmt.Sprintf(
		"X-Timestamp:%s\r\n"+
			"Content-Type:application/json; charset=utf-8\r\n"+
			"Path:speech.config\r\n\r\n"+
			`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"true","wordBoundaryEnabled":"true"},"outputFormat":"%s"}}}}`,
		timestampStr, formatInfo.EdgeFormat,
	)

	if err := conn.WriteMessage(websocket.TextMessage, []byte(configMessage)); err != nil {
		return nil, nil, fmt.Errorf("falha ao enviar speech.config: %w", err)
	}

	// Detectar automaticamente o idioma correto a partir da voz
	lang := opts.Language
	if lang == "" || lang == "pt-BR" {
		lang = extractLangFromVoice(opts.Voice)
	}

	// 2. Enviar SSML Payload Completo com suporte a pausas inteligentes e SSML avançado
	ssmlContent := BuildSSMLWithOptions(opts.Text, SSMLOptions{
		Voice:       opts.Voice,
		Rate:        opts.Rate,
		Pitch:       opts.Pitch,
		Volume:      opts.Volume,
		Lang:        lang,
		BreakComma:  opts.BreakComma,
		BreakPeriod: opts.BreakPeriod,
	})
	slog.Info("Enviando SSML para Edge TTS", "ssml", ssmlContent)
	ssmlMessage := fmt.Sprintf(
		"X-RequestId:%s\r\n"+
			"Content-Type:application/ssml+xml\r\n"+
			"X-Timestamp:%sZ\r\n"+
			"Path:ssml\r\n\r\n"+
			"%s",
		requestID, timestampStr, ssmlContent,
	)

	if err := conn.WriteMessage(websocket.TextMessage, []byte(ssmlMessage)); err != nil {
		return nil, nil, fmt.Errorf("falha ao enviar payload ssml: %w", err)
	}

	// 3. Receber os pacotes de áudio e fazer streaming imediato para o writer
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		msgType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) || ctx.Err() != nil {
				return words, sentences, nil
			}
			return nil, nil, fmt.Errorf("erro ao receber mensagens do websocket: %w", err)
		}

		switch msgType {
		case websocket.BinaryMessage:
			if len(message) < 2 {
				continue
			}
			headerLen := int(binary.BigEndian.Uint16(message[0:2]))
			if len(message) < 2+headerLen {
				continue
			}

			headerStr := string(message[2 : 2+headerLen])
			if strings.Contains(headerStr, "Path:audio") {
				audioChunk := message[2+headerLen:]
				if len(audioChunk) > 0 {
					if _, writeErr := writer.Write(audioChunk); writeErr != nil {
						return nil, nil, fmt.Errorf("erro ao gravar chunk de áudio: %w", writeErr)
					}
				}
			}

		case websocket.TextMessage:
			textMsg := string(message)
			if strings.Contains(textMsg, "Path:turn.end") {
				slog.Debug("Síntese concluída com sucesso", "request_id", requestID)
				// Refinar StartMs das sentenças usando os WordBoundary mais precisos
				sentences = refineSentenceStartMs(words, sentences)
				return words, sentences, nil
			}
			if strings.Contains(textMsg, "Path:audio.metadata") {
				if idx := strings.Index(textMsg, "\r\n\r\n"); idx != -1 {
					var meta metadataJSON
					if err := json.Unmarshal([]byte(textMsg[idx+4:]), &meta); err == nil {
						for _, item := range meta.Metadata {
							startMs := item.Data.Offset / 10000
							endMs := startMs + (item.Data.Duration / 10000)
							if item.Type == "WordBoundary" {
								words = append(words, subtitles.WordTimestamp{
									Word:    item.Data.Text.Text,
									StartMs: startMs,
									EndMs:   endMs,
								})
							} else if item.Type == "SentenceBoundary" {
								sentences = append(sentences, subtitles.SentenceCue{
									Index:   len(sentences) + 1,
									Text:    item.Data.Text.Text,
									StartMs: startMs,
									EndMs:   endMs,
								})
							}
						}
					}
				}
			}
		}
	}
}

// refineSentenceStartMs melhora a precisão do StartMs de cada SentenceCue
// buscando o primeiro WordBoundary ANTERIOR ou LEVEMENTE POSTERIOR ao SentenceBoundary.
//
// Regras de segurança:
//  1. Só aceita words cujo StartMs está NO MÁXIMO 50ms ANTES do boundary (antécipado = bom).
//  2. NÃO move o StartMs para FRENTE: se a word mais próxima está depois do boundary,
//     mantemos o SentenceBoundary original (que já é preciso o suficiente).
//
// Motivo: o Edge TTS não emite WordBoundary para palavras funcionais curtas ('Só', 'um',
// 'enquanto'). Sem essa guarda, o algoritmo pulava para a primeira palavra SUBSTANCIAL
// da sentença (ex: 'consulto'), corrompendo o seek e a sincronização.
func refineSentenceStartMs(words []subtitles.WordTimestamp, sentences []subtitles.SentenceCue) []subtitles.SentenceCue {
	if len(words) == 0 || len(sentences) == 0 {
		return sentences
	}

	const maxLeadMs int64 = 50 // janela de antecipação aceita (word antes do boundary)

	refined := make([]subtitles.SentenceCue, len(sentences))
	copy(refined, sentences)

	for i := range refined {
		sentenceStart := refined[i].StartMs

		// Busca a word mais próxima que ocorre ANTES do boundary (dentro de maxLeadMs)
		for _, w := range words {
			if w.StartMs >= sentenceStart-maxLeadMs && w.StartMs < sentenceStart {
				// Word ligeiramente antes do boundary: antecipa o início da legenda
				refined[i].StartMs = w.StartMs
				break
			}
			if w.StartMs >= sentenceStart {
				// Já passou do boundary sem encontrar word anterior: mantém original
				break
			}
		}
	}

	return refined
}
