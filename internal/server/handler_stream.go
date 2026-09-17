package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"edgego-voice/internal/audiocache"
	"edgego-voice/internal/audioorchestrator"
	"edgego-voice/internal/edgetts"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Permite conexões CORS de qualquer origem
	},
}

// StreamMessage representa as mensagens trocadas via WebSocket.
type StreamMessage struct {
	Type        string  `json:"type"`                   // "config", "text", "flush", "stop", "done", "error"
	Text        string  `json:"text,omitempty"`         // Fragmento de texto / token do LLM
	Voice       string  `json:"voice,omitempty"`        // Voz desejada (ex: "pt-BR-FranciscaNeural")
	Format      string  `json:"format,omitempty"`       // Formato de áudio (mp3, wav, opus)
	Speed       float64 `json:"speed,omitempty"`        // Velocidade (ex: 1.0)
	Pitch       string  `json:"pitch,omitempty"`        // Tom (ex: "+0Hz")
	BreakComma  string  `json:"break_comma,omitempty"`  // Pausa após vírgulas (ex: "150ms")
	BreakPeriod string  `json:"break_period,omitempty"` // Pausa após pontos (ex: "350ms")
	Telephony   bool    `json:"telephony,omitempty"`    // Ativa filtro DSP telefônico
	AutoBreath  bool    `json:"auto_breath,omitempty"`  // Ativa micro-respiração orgânica
	Persona     string  `json:"persona,omitempty"`      // Slug da Persona
	Error       string  `json:"error,omitempty"`        // Mensagem de erro (servidor -> cliente)
	Sentence    string  `json:"sentence,omitempty"`     // Sentença que está sendo reproduzida
}

// StreamSpeechWebSocket gerencia a conexão bidirecional WebSocket para streaming de tokens do LLM em tempo real (TTFB < 250ms).
func (h *SpeechHandler) StreamSpeechWebSocket(w http.ResponseWriter, r *http.Request) {
	// 1. Validação opcional de autenticação via Header ou Query Param (?token= / ?key=) com proteção contra timing-attacks
	if h.cfg.RequireAPIKey {
		authHeader := r.Header.Get("Authorization")
		token := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token == "" {
			token = r.URL.Query().Get("key")
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.APIKey)) != 1 {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("Falha no upgrade para WebSocket", "error", err)
		return
	}
	defer conn.Close()

	// Limite defensivo para tamanho máximo de mensagens recebidas (1MB anti-DoS)
	conn.SetReadLimit(1 << 20)

	// Parâmetros padrão iniciais (podem vir de query params ou mensagem de config)
	voice := r.URL.Query().Get("voice")
	if voice == "" {
		voice = h.cfg.DefaultVoice
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = h.cfg.DefaultFormat
	}
	speed := 1.0
	if sp := r.URL.Query().Get("speed"); sp != "" {
		if f, err := strconv.ParseFloat(sp, 64); err == nil && f > 0 {
			speed = f
		}
	}
	pitch := r.URL.Query().Get("pitch")
	if pitch == "" {
		pitch = "+0Hz"
	}
	telephony := r.URL.Query().Get("telephony") == "true"
	autoBreath := r.URL.Query().Get("auto_breath") == "true"

	// Se especificou persona na query
	if pID := r.URL.Query().Get("persona"); pID != "" {
		if p, err := h.personasManager.Get(pID); err == nil {
			voice = p.Voice
			format = p.Format
			speed = p.Speed
			pitch = p.Pitch
			telephony = p.Telephony
			autoBreath = p.AutoBreath
		}
	}

	voiceMap := h.voiceManager.GetAllVoices()
	chunker := audioorchestrator.NewTagAwareChunker()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var writeMu sync.Mutex
	sendJSON := func(msg StreamMessage) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteJSON(msg)
	}

	sendBinary := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.BinaryMessage, data)
	}

	// Canal de sentenças prontas para síntese sequencial
	sentenceChan := make(chan string, 100)

	// Goroutine trabalhadora que sintetiza e envia áudios em ordem
	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case sentence, ok := <-sentenceChan:
				if !ok {
					return
				}

				sendJSON(StreamMessage{
					Type:     "sentence_start",
					Sentence: sentence,
				})

				rateStr, _ := edgetts.SpeedToRate(speed)
				if rateStr == "" {
					rateStr = "+0%"
				}

				opts := edgetts.SynthesizeOptions{
					Text:            sentence,
					Voice:           voice,
					Rate:            rateStr,
					Pitch:           pitch,
					Language:        h.cfg.DefaultLanguage,
					Format:          format,
					TelephonyFilter: telephony,
					AutoBreath:      autoBreath,
				}

				// Sintetizar a sentença com deduplicação de cache
				cacheKey := audiocache.GenerateKeyWithOptions(sentence, voice, format, speed, pitch, false, telephony, autoBreath)
				var audioData []byte
				var hit bool

				if cached, found := h.cache.Get(cacheKey); found {
					audioData = cached
					hit = true
				} else {
					data, sErr, _ := h.singleFlight.Do(cacheKey, func() ([]byte, error) {
						if c, f := h.cache.Get(cacheKey); f {
							return c, nil
						}
						res, _, err := audioorchestrator.SynthesizeOrchestrated(ctx, h.ttsClient, sentence, opts, voiceMap)
						if err == nil && len(res) > 0 {
							h.cache.Set(cacheKey, res)
						}
						return res, err
					})
					if sErr == nil {
						audioData = data
					} else {
						slog.Error("Erro na síntese do fragmento streaming", "error", sErr, "sentence", sentence)
						continue
					}
				}

				if len(audioData) > 0 {
					_ = sendBinary(audioData)
					sendJSON(StreamMessage{
						Type:     "sentence_end",
						Sentence: sentence,
						Text:     strconv.FormatBool(hit), // indica se foi cache HIT
					})
				}
			}
		}
	}()

	slog.Info("Cliente conectado ao streaming WebSocket de fala", "remote_addr", r.RemoteAddr, "voz", voice)

	// Loop principal de leitura do WebSocket
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.Debug("Conexão WebSocket finalizada", "remote_addr", r.RemoteAddr, "motivo", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			var msg StreamMessage
			if err := json.Unmarshal(p, &msg); err != nil {
				// Fallback: se não for JSON, trata o texto bruto como token
				rawText := string(p)
				for _, chunk := range chunker.Feed(rawText) {
					sentenceChan <- chunk
				}
				continue
			}

			switch strings.ToLower(msg.Type) {
			case "config":
				if msg.Voice != "" {
					voice = msg.Voice
				}
				if msg.Format != "" {
					format = msg.Format
				}
				if msg.Speed > 0 {
					speed = msg.Speed
				}
				if msg.Pitch != "" {
					pitch = msg.Pitch
				}
				telephony = msg.Telephony
				autoBreath = msg.AutoBreath
				if msg.Persona != "" {
					if p, err := h.personasManager.Get(msg.Persona); err == nil {
						voice = p.Voice
						format = p.Format
						speed = p.Speed
						pitch = p.Pitch
						telephony = p.Telephony
						autoBreath = p.AutoBreath
					}
				}
				sendJSON(StreamMessage{Type: "ready", Voice: voice, Format: format})

			case "text":
				if msg.Text != "" {
					for _, chunk := range chunker.Feed(msg.Text) {
						sentenceChan <- chunk
					}
				}

			case "flush":
				// Esvazia buffer e envia sentenças pendentes
				for _, chunk := range chunker.Flush() {
					sentenceChan <- chunk
				}

			case "stop":
				cancel()
				return

			default:
				if msg.Text != "" {
					for _, chunk := range chunker.Feed(msg.Text) {
						sentenceChan <- chunk
					}
				}
			}
		}
	}

	// Flush final ao desconectar
	for _, chunk := range chunker.Flush() {
		sentenceChan <- chunk
	}
	close(sentenceChan)
	workerWg.Wait()

	sendJSON(StreamMessage{Type: "done"})
}
