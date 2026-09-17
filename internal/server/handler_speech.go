package server

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"edgego-voice/internal/audiocache"
	"edgego-voice/internal/audioorchestrator"
	"edgego-voice/internal/config"
	"edgego-voice/internal/edgetts"
	"edgego-voice/internal/personas"
	"edgego-voice/internal/subtitles"
	"edgego-voice/internal/textcleaner"
	"edgego-voice/internal/tts"
)

type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
	Pitch          string  `json:"pitch"`
	BreakComma     string  `json:"break_comma"`
	BreakPeriod    string  `json:"break_period"`
	Telephony      bool    `json:"telephony"`
	AutoBreath     bool    `json:"auto_breath"`
	Emotion        string  `json:"emotion,omitempty"`
	EmotionScale   string  `json:"emotion_scale,omitempty"`
	Style          string  `json:"style,omitempty"`
	Role           string  `json:"role,omitempty"`
	WithSubtitles  bool    `json:"with_subtitles,omitempty"`
	SubtitleFormat string  `json:"subtitle_format,omitempty"` // "srt", "vtt", "json"
}

// SpeechVerboseResponse representa a resposta estruturada contendo áudio em base64 e legendas completas.
type SpeechVerboseResponse struct {
	Audio      string                   `json:"audio"`
	Format     string                   `json:"format"`
	DurationMs int64                    `json:"duration_ms"`
	Subtitles  subtitles.SubtitleResult `json:"subtitles"`
}

type synthResultItem struct {
	audio     []byte
	subResult subtitles.SubtitleResult
}

type SpeechHandler struct {
	cfg             *config.Config
	ttsClient       tts.Synthesizer
	voiceManager    *edgetts.VoiceManager
	personasManager *personas.Manager
	cache           *audiocache.Cache
	singleFlight    *audiocache.Group[[]byte]
	singleFlightSub *audiocache.Group[synthResultItem]
}

func NewSpeechHandler(
	cfg *config.Config,
	ttsClient tts.Synthesizer,
	vm *edgetts.VoiceManager,
	pm *personas.Manager,
	cache *audiocache.Cache,
) *SpeechHandler {
	return &SpeechHandler{
		cfg:             cfg,
		ttsClient:       ttsClient,
		voiceManager:    vm,
		personasManager: pm,
		cache:           cache,
		singleFlight:    audiocache.NewGroup[[]byte](),
		singleFlightSub: audiocache.NewGroup[synthResultItem](),
	}
}

// flushWriter envolve http.ResponseWriter para enviar blocos de áudio imediatamente ao cliente (TTFB baixo).
type flushWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (w *flushWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return n, err
}

func truncateSnippet(s string) string {
	runes := []rune(s)
	if len(runes) > 60 {
		return string(runes[:57]) + "..."
	}
	return s
}

// GenerateSpeechHandler processa a rota OpenAI compatível POST /v1/audio/speech.
func (h *SpeechHandler) GenerateSpeechHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // Limite de 2MB para payload JSON de síntese (anti-DoS)
	var req SpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido: "+err.Error(), "invalid_request_error")
		return
	}

	if req.Input == "" {
		writeJSONError(w, http.StatusBadRequest, "O campo 'input' é obrigatório.", "invalid_request_error")
		return
	}

	// Verificar se foi solicitado via header X-Persona-ID
	personaID := r.Header.Get("X-Persona-ID")
	if personaID != "" {
		p, err := h.personasManager.Get(personaID)
		if err == nil {
			h.synthesizeForPersona(w, r, p, req)
			return
		}
	}

	// 1. Sanitizar texto
	text := req.Input
	if !h.cfg.RemoveFilter {
		text = textcleaner.CleanText(text)
	}

	if text == "" {
		writeJSONError(w, http.StatusBadRequest, "O texto fornecido está vazio após a limpeza.", "invalid_request_error")
		return
	}

	// 2. Preencher valores padrão
	voice := req.Voice
	if voice == "" {
		voice = h.cfg.DefaultVoice
	}
	realVoice := h.voiceManager.ResolveVoice(voice)

	format := req.ResponseFormat
	if format == "" {
		format = h.cfg.DefaultFormat
	}

	speed := req.Speed
	if speed == 0 {
		speed = h.cfg.DefaultSpeed
	}

	rate, err := edgetts.SpeedToRate(speed)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	pitch := req.Pitch
	if pitch == "" {
		pitch = "+0Hz"
	}

	formatInfo := edgetts.GetFormatInfo(format)

	// 3. Checar Cache SLRU em Memória (~0.1ms)
	cacheKey := audiocache.GenerateKeyWithEmotions(
		text,
		realVoice,
		format,
		req.Speed,
		pitch,
		h.cfg.RemoveFilter,
		req.Telephony,
		req.AutoBreath,
		req.Emotion,
		req.EmotionScale,
		req.Style,
	)

	cacheStatus := "MISS"
	var audioData []byte
	var subResult subtitles.SubtitleResult

	if cachedAudio, hit := h.cache.Get(cacheKey); hit {
		cacheStatus = "HIT"
		audioData = cachedAudio
		if cachedSub, hitSub := h.cache.Get(cacheKey + ":subtitles"); hitSub {
			_ = json.Unmarshal(cachedSub, &subResult)
		} else {
			durMs := audioorchestrator.ExactAudioDurationMs(audioData, format)
			sents := subtitles.GenerateFallbackCues(text, durMs)
			subResult = subtitles.SubtitleResult{
				Text:       text,
				DurationMs: durMs,
				Sentences:  sents,
				SRT:        subtitles.FormatSRT(sents),
				VTT:        subtitles.FormatVTT(sents),
			}
		}
	} else {
		// 4. Cache MISS -> Preparar opções e sintetizar áudio com legendas
		opts := edgetts.SynthesizeOptions{
			Text:            text,
			Voice:           realVoice,
			Rate:            rate,
			Pitch:           pitch,
			Language:        h.cfg.DefaultLanguage,
			Format:          format,
			BreakComma:      req.BreakComma,
			BreakPeriod:     req.BreakPeriod,
			TelephonyFilter: req.Telephony,
			AutoBreath:      req.AutoBreath,
			Emotion:         req.Emotion,
			EmotionScale:    req.EmotionScale,
			Style:           req.Style,
			Role:            req.Role,
		}

		res, err, _ := h.singleFlightSub.Do(cacheKey, func() (synthResultItem, error) {
			if cachedAudio2, hit2 := h.cache.Get(cacheKey); hit2 {
				var subRes subtitles.SubtitleResult
				if cachedSub2, hitSub2 := h.cache.Get(cacheKey + ":subtitles"); hitSub2 {
					_ = json.Unmarshal(cachedSub2, &subRes)
				}
				return synthResultItem{audio: cachedAudio2, subResult: subRes}, nil
			}

			data, sResult, _, sErr := audioorchestrator.SynthesizeOrchestratedWithSubtitles(
				r.Context(),
				h.ttsClient,
				text,
				opts,
				h.voiceManager.GetAllVoices(),
			)
			if sErr == nil && len(data) > 0 {
				h.cache.Set(cacheKey, data)
				if subJSON, mErr := json.Marshal(sResult); mErr == nil {
					h.cache.Set(cacheKey+":subtitles", subJSON)
				}
			}
			return synthResultItem{audio: data, subResult: sResult}, sErr
		})

		if err != nil {
			if r.Context().Err() != nil {
				slog.Warn("Cliente cancelou a requisição antes do término da síntese")
				return
			}
			slog.Error("Erro ao sintetizar áudio", "error", err)
			writeJSONError(w, http.StatusBadGateway, "Não foi possível sintetizar o áudio com a voz selecionada. Verifique os parâmetros ou tente outra voz.", "synthesizer_error")
			return
		}
		audioData = res.audio
		subResult = res.subResult
	}

	if req.ResponseFormat == "verbose_json" || req.WithSubtitles {
		resp := SpeechVerboseResponse{
			Audio:      base64.StdEncoding.EncodeToString(audioData),
			Format:     format,
			DurationMs: subResult.DurationMs,
			Subtitles:  subResult,
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Cache", cacheStatus)
		w.Header().Set("X-Subtitles-Duration-Ms", strconv.FormatInt(subResult.DurationMs, 10))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", formatInfo.MimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(audioData)))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"speech.%s\"", format))
	w.Header().Set("X-Cache", cacheStatus)
	w.Header().Set("X-Subtitles-Duration-Ms", strconv.FormatInt(subResult.DurationMs, 10))
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(audioData); err != nil {
		slog.Error("Erro ao enviar dados de áudio para o cliente", "error", err)
		return
	}
}

// GeneratePersonaSpeechHandler processa a rota dedicada POST /v1/persona/{id}/speech.
func (h *SpeechHandler) GeneratePersonaSpeechHandler(w http.ResponseWriter, r *http.Request) {
	personaID := chi.URLParam(r, "id")
	p, err := h.personasManager.Get(personaID)
	if err != nil {
		if errors.Is(err, personas.ErrPersonaNotFound) {
			writeJSONError(w, http.StatusNotFound, "Persona não encontrada", "not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	// Validação de autenticação se a persona tiver chave exclusiva (com proteção contra timing-attack)
	if h.cfg.RequireAPIKey {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		globalMatch := subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.APIKey)) == 1
		personaMatch := p.APIKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(p.APIKey)) == 1
		if token == "" || (!globalMatch && !personaMatch) {
			writeJSONError(w, http.StatusUnauthorized, "Chave de API inválida para esta Persona.", "authentication_error")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // Limite de 2MB anti-DoS
	var req SpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido: "+err.Error(), "invalid_request_error")
		return
	}

	if req.Input == "" {
		writeJSONError(w, http.StatusBadRequest, "O campo 'input' é obrigatório.", "invalid_request_error")
		return
	}

	h.synthesizeForPersona(w, r, p, req)
}

func (h *SpeechHandler) synthesizeForPersona(w http.ResponseWriter, r *http.Request, p personas.Persona, req SpeechRequest) {
	text := req.Input
	removeFilter := p.RemoveFilter || h.cfg.RemoveFilter
	if !removeFilter {
		text = textcleaner.CleanText(text)
	}

	if text == "" {
		writeJSONError(w, http.StatusBadRequest, "O texto fornecido está vazio após a limpeza.", "invalid_request_error")
		return
	}

	voice := p.Voice
	if req.Voice != "" {
		voice = req.Voice
	}
	realVoice := h.voiceManager.ResolveVoice(voice)

	format := p.Format
	if req.ResponseFormat != "" {
		format = req.ResponseFormat
	}
	if format == "" {
		format = "mp3"
	}

	speed := p.Speed
	if req.Speed > 0 {
		speed = req.Speed
	}
	if speed <= 0 {
		speed = 1.0
	}

	rate, err := edgetts.SpeedToRate(speed)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	pitch := p.Pitch
	if req.Pitch != "" {
		pitch = req.Pitch
	}
	if pitch == "" {
		pitch = "+0Hz"
	}

	telephony := p.Telephony
	if req.Telephony {
		telephony = true
	}
	autoBreath := p.AutoBreath
	if req.AutoBreath {
		autoBreath = true
	}

	formatInfo := edgetts.GetFormatInfo(format)

	emotion := p.Emotion
	if req.Emotion != "" {
		emotion = req.Emotion
	}
	emotionScale := p.EmotionScale
	if req.EmotionScale != "" {
		emotionScale = req.EmotionScale
	}
	style := p.Style
	if req.Style != "" {
		style = req.Style
	}
	role := p.Role
	if req.Role != "" {
		role = req.Role
	}

	// Checar Cache SLRU em Memória (~0.1ms)
	cacheKey := audiocache.GenerateKeyWithEmotions(
		text, realVoice, format, speed, pitch, removeFilter, telephony, autoBreath,
		emotion, emotionScale, style,
	)

	cacheStatus := "MISS"
	var audioData []byte
	var subResult subtitles.SubtitleResult

	if cachedAudio, hit := h.cache.Get(cacheKey); hit {
		cacheStatus = "HIT"
		audioData = cachedAudio
		if cachedSub, hitSub := h.cache.Get(cacheKey + ":subtitles"); hitSub {
			_ = json.Unmarshal(cachedSub, &subResult)
		} else {
			durMs := audioorchestrator.ExactAudioDurationMs(audioData, format)
			sents := subtitles.GenerateFallbackCues(text, durMs)
			subResult = subtitles.SubtitleResult{
				Text:       text,
				DurationMs: durMs,
				Sentences:  sents,
				SRT:        subtitles.FormatSRT(sents),
				VTT:        subtitles.FormatVTT(sents),
			}
		}
	} else {
		// Cache MISS -> Configurar parâmetros e sintetizar com legendas
		breakComma := p.BreakComma
		if req.BreakComma != "" {
			breakComma = req.BreakComma
		}
		breakPeriod := p.BreakPeriod
		if req.BreakPeriod != "" {
			breakPeriod = req.BreakPeriod
		}

		opts := edgetts.SynthesizeOptions{
			Text:            text,
			Voice:           realVoice,
			Rate:            rate,
			Pitch:           pitch,
			Language:        h.cfg.DefaultLanguage,
			Format:          format,
			BreakComma:      breakComma,
			BreakPeriod:     breakPeriod,
			TelephonyFilter: telephony,
			AutoBreath:      autoBreath,
			Emotion:         emotion,
			EmotionScale:    emotionScale,
			Style:           style,
			Role:            role,
		}

		res, err, _ := h.singleFlightSub.Do(cacheKey, func() (synthResultItem, error) {
			if cached, hit := h.cache.Get(cacheKey); hit {
				var subRes subtitles.SubtitleResult
				if cachedSub, hitSub := h.cache.Get(cacheKey + ":subtitles"); hitSub {
					_ = json.Unmarshal(cachedSub, &subRes)
				}
				return synthResultItem{audio: cached, subResult: subRes}, nil
			}

			data, sResult, _, sErr := audioorchestrator.SynthesizeOrchestratedWithSubtitles(
				r.Context(),
				h.ttsClient,
				text,
				opts,
				h.voiceManager.GetAllVoices(),
			)
			if sErr == nil && len(data) > 0 {
				h.cache.Set(cacheKey, data)
				if subJSON, mErr := json.Marshal(sResult); mErr == nil {
					h.cache.Set(cacheKey+":subtitles", subJSON)
				}
			}
			return synthResultItem{audio: data, subResult: sResult}, sErr
		})

		if err != nil {
			if r.Context().Err() != nil {
				slog.Warn("Cliente cancelou a requisição antes do término da síntese")
				return
			}
			slog.Error("Erro ao sintetizar áudio para persona", "persona", p.ID, "error", err)
			writeJSONError(w, http.StatusBadGateway, "Não foi possível sintetizar o áudio para esta persona. Tente novamente ou escolha outra voz.", "synthesizer_error")
			return
		}
		audioData = res.audio
		subResult = res.subResult
	}

	if req.ResponseFormat == "verbose_json" || req.WithSubtitles {
		resp := SpeechVerboseResponse{
			Audio:      base64.StdEncoding.EncodeToString(audioData),
			Format:     format,
			DurationMs: subResult.DurationMs,
			Subtitles:  subResult,
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Persona-ID", p.ID)
		w.Header().Set("X-Cache", cacheStatus)
		w.Header().Set("X-Subtitles-Duration-Ms", strconv.FormatInt(subResult.DurationMs, 10))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", formatInfo.MimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(audioData)))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.%s\"", p.ID, format))
	w.Header().Set("X-Persona-ID", p.ID)
	w.Header().Set("X-Cache", cacheStatus)
	w.Header().Set("X-Subtitles-Duration-Ms", strconv.FormatInt(subResult.DurationMs, 10))
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(audioData); err != nil {
		slog.Error("Erro ao enviar áudio de persona para o cliente", "error", err)
		return
	}
}

// GenerateSubtitlesHandler processa a rota POST /v1/audio/subtitles para extrair apenas legendas (.srt, .vtt ou .json).
func (h *SpeechHandler) GenerateSubtitlesHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var req SpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido: "+err.Error(), "invalid_request_error")
		return
	}

	if req.Input == "" {
		writeJSONError(w, http.StatusBadRequest, "O campo 'input' é obrigatório.", "invalid_request_error")
		return
	}

	text := req.Input
	if !h.cfg.RemoveFilter {
		text = textcleaner.CleanText(text)
	}
	if text == "" {
		writeJSONError(w, http.StatusBadRequest, "O texto fornecido está vazio após a limpeza.", "invalid_request_error")
		return
	}

	voice := req.Voice
	if voice == "" {
		voice = h.cfg.DefaultVoice
	}
	realVoice := h.voiceManager.ResolveVoice(voice)

	format := "mp3"
	speed := req.Speed
	if speed == 0 {
		speed = h.cfg.DefaultSpeed
	}
	rate, err := edgetts.SpeedToRate(speed)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	pitch := req.Pitch
	if pitch == "" {
		pitch = "+0Hz"
	}

	cacheKey := audiocache.GenerateKeyWithEmotions(
		text,
		realVoice,
		format,
		speed,
		pitch,
		h.cfg.RemoveFilter,
		req.Telephony,
		req.AutoBreath,
		req.Emotion,
		req.EmotionScale,
		req.Style,
	)

	subFormat := req.SubtitleFormat
	if subFormat == "" {
		if req.ResponseFormat == "vtt" || req.ResponseFormat == "json" || req.ResponseFormat == "srt" {
			subFormat = req.ResponseFormat
		} else {
			subFormat = "srt"
		}
	}

	// Checar cache de legendas (~0.1ms)
	if cachedSub, hit := h.cache.Get(cacheKey + ":subtitles"); hit {
		var subResult subtitles.SubtitleResult
		if err := json.Unmarshal(cachedSub, &subResult); err == nil {
			w.Header().Set("X-Cache", "HIT")
			writeSubtitleResponse(w, subResult, subFormat, "subtitles")
			return
		}
	}

	// Cache MISS -> sintetizar com legendas
	opts := edgetts.SynthesizeOptions{
		Text:            text,
		Voice:           realVoice,
		Rate:            rate,
		Pitch:           pitch,
		Language:        h.cfg.DefaultLanguage,
		Format:          format,
		BreakComma:      req.BreakComma,
		BreakPeriod:     req.BreakPeriod,
		TelephonyFilter: req.Telephony,
		AutoBreath:      req.AutoBreath,
		Emotion:         req.Emotion,
		EmotionScale:    req.EmotionScale,
		Style:           req.Style,
		Role:            req.Role,
	}

	res, err, _ := h.singleFlightSub.Do(cacheKey, func() (synthResultItem, error) {
		if cachedAudio, hit := h.cache.Get(cacheKey); hit {
			var subRes subtitles.SubtitleResult
			if cachedSub, hitSub := h.cache.Get(cacheKey + ":subtitles"); hitSub {
				_ = json.Unmarshal(cachedSub, &subRes)
			}
			return synthResultItem{audio: cachedAudio, subResult: subRes}, nil
		}

		data, sResult, _, sErr := audioorchestrator.SynthesizeOrchestratedWithSubtitles(
			r.Context(),
			h.ttsClient,
			text,
			opts,
			h.voiceManager.GetAllVoices(),
		)
		if sErr == nil && len(data) > 0 {
			h.cache.Set(cacheKey, data)
			if subJSON, mErr := json.Marshal(sResult); mErr == nil {
				h.cache.Set(cacheKey+":subtitles", subJSON)
			}
		}
		return synthResultItem{audio: data, subResult: sResult}, sErr
	})

	if err != nil {
		if r.Context().Err() != nil {
			slog.Warn("Cliente cancelou a requisição antes do término da síntese de legendas")
			return
		}
		slog.Error("Erro ao sintetizar legendas", "error", err)
		writeJSONError(w, http.StatusBadGateway, "Não foi possível gerar legendas para o texto informado. Tente novamente.", "synthesizer_error")
		return
	}

	w.Header().Set("X-Cache", "MISS")
	writeSubtitleResponse(w, res.subResult, subFormat, "subtitles")
}

// GeneratePersonaSubtitlesHandler processa a rota dedicada POST /v1/persona/{id}/subtitles.
func (h *SpeechHandler) GeneratePersonaSubtitlesHandler(w http.ResponseWriter, r *http.Request) {
	personaID := chi.URLParam(r, "id")
	p, err := h.personasManager.Get(personaID)
	if err != nil {
		if errors.Is(err, personas.ErrPersonaNotFound) {
			writeJSONError(w, http.StatusNotFound, "Persona não encontrada", "not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	// Validação de autenticação se a persona tiver chave exclusiva
	if h.cfg.RequireAPIKey {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		globalMatch := subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.APIKey)) == 1
		personaMatch := p.APIKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(p.APIKey)) == 1
		if token == "" || (!globalMatch && !personaMatch) {
			writeJSONError(w, http.StatusUnauthorized, "Chave de API inválida para esta Persona.", "authentication_error")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var req SpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido: "+err.Error(), "invalid_request_error")
		return
	}

	if req.Input == "" {
		writeJSONError(w, http.StatusBadRequest, "O campo 'input' é obrigatório.", "invalid_request_error")
		return
	}

	subFormat := req.SubtitleFormat
	if subFormat == "" {
		if req.ResponseFormat == "vtt" || req.ResponseFormat == "json" || req.ResponseFormat == "srt" {
			subFormat = req.ResponseFormat
		} else {
			subFormat = "srt"
		}
	}

	subResult, err := h.synthesizeSubtitlesForPersona(r, p, req)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "Não foi possível gerar legendas para esta persona. Tente novamente.", "synthesizer_error")
		return
	}

	writeSubtitleResponse(w, subResult, subFormat, p.ID)
}

func (h *SpeechHandler) synthesizeSubtitlesForPersona(r *http.Request, p personas.Persona, req SpeechRequest) (subtitles.SubtitleResult, error) {
	text := req.Input
	removeFilter := p.RemoveFilter || h.cfg.RemoveFilter
	if !removeFilter {
		text = textcleaner.CleanText(text)
	}
	if text == "" {
		return subtitles.SubtitleResult{}, errors.New("o texto fornecido está vazio após a limpeza")
	}

	voice := p.Voice
	if req.Voice != "" {
		voice = req.Voice
	}
	realVoice := h.voiceManager.ResolveVoice(voice)

	format := "mp3"
	speed := p.Speed
	if req.Speed > 0 {
		speed = req.Speed
	}
	if speed <= 0 {
		speed = 1.0
	}
	rate, err := edgetts.SpeedToRate(speed)
	if err != nil {
		return subtitles.SubtitleResult{}, err
	}

	pitch := p.Pitch
	if req.Pitch != "" {
		pitch = req.Pitch
	}
	if pitch == "" {
		pitch = "+0Hz"
	}

	telephony := p.Telephony || req.Telephony
	autoBreath := p.AutoBreath || req.AutoBreath
	emotion := p.Emotion
	if req.Emotion != "" {
		emotion = req.Emotion
	}
	emotionScale := p.EmotionScale
	if req.EmotionScale != "" {
		emotionScale = req.EmotionScale
	}
	style := p.Style
	if req.Style != "" {
		style = req.Style
	}
	role := p.Role
	if req.Role != "" {
		role = req.Role
	}

	cacheKey := audiocache.GenerateKeyWithEmotions(
		text, realVoice, format, speed, pitch, removeFilter, telephony, autoBreath,
		emotion, emotionScale, style,
	)

	if cachedSub, hit := h.cache.Get(cacheKey + ":subtitles"); hit {
		var subResult subtitles.SubtitleResult
		if err := json.Unmarshal(cachedSub, &subResult); err == nil {
			return subResult, nil
		}
	}

	breakComma := p.BreakComma
	if req.BreakComma != "" {
		breakComma = req.BreakComma
	}
	breakPeriod := p.BreakPeriod
	if req.BreakPeriod != "" {
		breakPeriod = req.BreakPeriod
	}

	opts := edgetts.SynthesizeOptions{
		Text:            text,
		Voice:           realVoice,
		Rate:            rate,
		Pitch:           pitch,
		Language:        h.cfg.DefaultLanguage,
		Format:          format,
		BreakComma:      breakComma,
		BreakPeriod:     breakPeriod,
		TelephonyFilter: telephony,
		AutoBreath:      autoBreath,
		Emotion:         emotion,
		EmotionScale:    emotionScale,
		Style:           style,
		Role:            role,
	}

	res, err, _ := h.singleFlightSub.Do(cacheKey, func() (synthResultItem, error) {
		if cachedAudio, hit := h.cache.Get(cacheKey); hit {
			var subRes subtitles.SubtitleResult
			if cachedSub, hitSub := h.cache.Get(cacheKey + ":subtitles"); hitSub {
				_ = json.Unmarshal(cachedSub, &subRes)
			}
			return synthResultItem{audio: cachedAudio, subResult: subRes}, nil
		}

		data, sResult, _, sErr := audioorchestrator.SynthesizeOrchestratedWithSubtitles(
			r.Context(),
			h.ttsClient,
			text,
			opts,
			h.voiceManager.GetAllVoices(),
		)
		if sErr == nil && len(data) > 0 {
			h.cache.Set(cacheKey, data)
			if subJSON, mErr := json.Marshal(sResult); mErr == nil {
				h.cache.Set(cacheKey+":subtitles", subJSON)
			}
		}
		return synthResultItem{audio: data, subResult: sResult}, sErr
	})

	if err != nil {
		return subtitles.SubtitleResult{}, err
	}
	return res.subResult, nil
}

func writeSubtitleResponse(w http.ResponseWriter, subResult subtitles.SubtitleResult, subFormat string, filename string) {
	subFormat = strings.ToLower(strings.TrimSpace(subFormat))
	switch subFormat {
	case "vtt":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.vtt\"", filename))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(subResult.VTT))
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", filename))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(subResult)
	default: // "srt"
		w.Header().Set("Content-Type", "application/x-subrip; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.srt\"", filename))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(subResult.SRT))
	}
}
