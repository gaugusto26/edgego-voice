package capcuttts

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrVoiceNotFound = errors.New("voz não encontrada no catálogo CapCut")
	ErrTaskFailed    = errors.New("tarefa de síntese falhou no servidor CapCut")
	ErrTaskTimeout   = errors.New("tempo limite esgotado aguardando síntese do CapCut")
	ErrEmptyAudioURL = errors.New("nenhuma URL de áudio retornada na resposta da síntese")
)

// SynthesizeOptions define os parâmetros para síntese com o motor CapCut.
type SynthesizeOptions struct {
	Text         string
	Voice        string
	Rate         string  // Taxa de velocidade (ex: "1.0", "1.15")
	Pitch        string  // Ex: "+0Hz"
	Emotion      string  // Emoção/Sentimento (ex: "happy", "calm", "sad", "angry")
	EmotionScale string  // Intensidade (ex: "0", "1.0", "1.5", "2.0")
	Style        string  // Estilo de fala
	Role         string  // Papel do personagem
	Format       string  // Formato desejado (mp3)
}

// Client gerencia a comunicação e geração de áudio no CapCut TTS de forma 100% nativa.
type Client struct {
	httpClient *http.Client
	device     DeviceConfig
	catalog    *Catalog
	baseURL    string
}

// NewClient cria uma nova instância do cliente CapCut TTS com pooling de conexões HTTP e suporte a proxy opcional.
func NewClient(proxyURL string) *Client {
	proxyFunc := http.ProxyFromEnvironment
	if proxyURL != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			proxyFunc = http.ProxyURL(parsed)
		}
	}

	transport := &http.Transport{
		Proxy:               proxyFunc,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression: false,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   35 * time.Second,
		},
		device:  DefaultDeviceConfig(),
		catalog: DefaultCatalog(),
		baseURL: defaultBaseURL,
	}
}

// IsVoiceSupported indica se a voz especificada pertence ao catálogo CapCut.
func (c *Client) IsVoiceSupported(voice string) bool {
	return c.catalog.IsCapCutVoice(voice)
}

// ResolveVoice busca e resolve a voz para VoiceType e ResourceID.
func (c *Client) ResolveVoice(voice string) (VoiceItem, bool) {
	return c.catalog.ResolveVoice(voice)
}

// SpeedToRate normaliza um float64 multiplicador de velocidade (ex: 1.15) para string de rate do SSML.
func SpeedToRate(speed float64) string {
	if speed <= 0 {
		return "1.0"
	}
	if speed < 0.5 {
		speed = 0.5
	} else if speed > 2.0 {
		speed = 2.0
	}
	return strconv.FormatFloat(speed, 'f', 2, 64)
}

// Synthesize executa a síntese completa: monta SSML com sentimento, envia a tarefa, aguarda via polling e baixa o MP3.
func (c *Client) Synthesize(ctx context.Context, opts SynthesizeOptions) ([]byte, error) {
	text := strings.TrimSpace(opts.Text)
	if text == "" {
		return nil, fmt.Errorf("o texto fornecido para síntese está vazio")
	}

	voiceItem, found := c.catalog.ResolveVoice(opts.Voice)
	if !found {
		// Fallback amigável para Wenrou
		voiceItem, found = c.catalog.ResolveVoice("DiT_pt_male_wenrou")
		if !found {
			return nil, fmt.Errorf("%w: %s", ErrVoiceNotFound, opts.Voice)
		}
	}

	rate := opts.Rate
	if rate == "" {
		rate = "1.0"
	}

	emotion := strings.TrimSpace(strings.ToLower(opts.Emotion))
	if emotion == "natural" || emotion == "neutral" || emotion == "neutro" {
		emotion = ""
	}

	emotionScale := strings.TrimSpace(opts.EmotionScale)
	if emotionScale == "" || emotion == "" {
		if emotion != "" {
			emotionScale = "1.0"
		} else {
			emotionScale = "0"
		}
	}

	babiParam := map[string]string{
		"feature_entrance":        "editor",
		"feature_entrance_detail": "editor-feature-text_to_speech",
		"feature_key":             "text_to_speech",
		"scenario":                "video_editor",
	}
	babiBytes, _ := json.Marshal(babiParam)
	babiStr := string(babiBytes)

	// Montar SSML com voice_type, resource_id, emoções e prosódia
	ssml := fmt.Sprintf(
		`<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="en-US">
    <voice name="%s" mock_tone_info="" platform="sami" resource_id="%s" emotion="%s" emotion_scale="%s" style="%s" role="%s" moyin_emotion="" is_clone_tone="false" need_subtitle_timestamp="false">
        <prosody rate="%s">%s</prosody>
    </voice>
</speak>`,
		voiceItem.VoiceType,
		voiceItem.ResourceID,
		emotion,
		emotionScale,
		opts.Style,
		opts.Role,
		rate,
		escapeXML(text),
	)

	extraInfo := `{"benefit_info":{}}`
	payloadSign, err := makeTTSPayloadSign(ssml, extraInfo, c.device.DeviceID, c.device.AID)
	if err != nil {
		return nil, fmt.Errorf("falha ao assinar payload TTS com RSA: %w", err)
	}

	payloadMap := map[string]interface{}{
		"audio_format":            "mp3",
		"babi_param":              babiStr,
		"credit_disable":          false,
		"extra_info":              extraInfo,
		"need_merge_voice":        false,
		"need_subtitle_timestamp": false,
		"scene":                   "text_to_speech",
		"sign":                    payloadSign,
		"ssml":                    ssml,
	}
	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar payload: %w", err)
	}

	contextID := uuid.New().String()
	bodyMap := map[string]interface{}{
		"bind_id":    uuid.New().String(),
		"can_queue":   true,
		"enter_from": "text_to_speech",
		"tasks": []map[string]interface{}{
			{
				"context":      contextID,
				"payload":      string(payloadBytes),
				"req_key":      "sami_text_to_speech",
				"task_version": "v3",
			},
		},
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar body de criação: %w", err)
	}
	bodyStr := string(bodyBytes)

	now := fmt.Sprintf("%d", time.Now().Unix())
	q := buildCommonQuery(c.device, babiStr, true)
	createURL := fmt.Sprintf("%s/lv/v1/common_task/new?%s", c.baseURL, q.Encode())
	signHeader := makeSignHeader(createURL, c.device.AppVR, now, c.device.TDID)

	req, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewBuffer([]byte(bodyStr)))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar requisição HTTP: %w", err)
	}
	req.Header = buildBaseHeaders(c.device, bodyStr, now, signHeader, true)

	startT := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha de conexão ao submeter tarefa CapCut: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler resposta da tarefa: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("servidor CapCut retornou HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var createRes struct {
		Ret     string `json:"ret"`
		ErrMsg  string `json:"errmsg"`
		Data    struct {
			Tasks []struct {
				ID    string `json:"id"`
				Token string `json:"token"`
			} `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &createRes); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON de criação: %w", err)
	}

	if len(createRes.Data.Tasks) == 0 {
		return nil, fmt.Errorf("nenhuma tarefa retornada pelo CapCut: %s", string(respBytes))
	}

	taskID := createRes.Data.Tasks[0].ID
	token := createRes.Data.Tasks[0].Token

	slog.Debug("Tarefa CapCut submetida",
		"task_id", taskID,
		"voice", voiceItem.VoiceType,
		"emotion", emotion,
		"elapsed_ms", time.Since(startT).Milliseconds())

	// Polling com intervalo resiliente e cancelamento imediato de contexto
	ticker := time.NewTicker(650 * time.Millisecond)
	defer ticker.Stop()

	queryURLPath := fmt.Sprintf("%s/lv/v1/common_task/query?%s", c.baseURL, buildCommonQuery(c.device, "", false).Encode())

	queryBodyMap := map[string]interface{}{
		"tasks": []map[string]interface{}{
			{
				"bind_id":      "",
				"id":           taskID,
				"req_key":      "sami_text_to_speech",
				"task_version": "v3",
				"token":        token,
			},
		},
	}
	qBodyBytes, _ := json.Marshal(queryBodyMap)
	qBodyStr := string(qBodyBytes)

	timeoutTimer := time.NewTimer(30 * time.Second)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("%w para task %s", ErrTaskTimeout, taskID)
		case <-ticker.C:
			nowQ := fmt.Sprintf("%d", time.Now().Unix())
			qSignHeader := makeSignHeader(queryURLPath, c.device.AppVR, nowQ, c.device.TDID)

			qReq, err := http.NewRequestWithContext(ctx, "POST", queryURLPath, bytes.NewBuffer([]byte(qBodyStr)))
			if err != nil {
				return nil, err
			}
			qReq.Header = buildBaseHeaders(c.device, qBodyStr, nowQ, qSignHeader, true)

			qResp, err := c.httpClient.Do(qReq)
			if err != nil {
				slog.Warn("Falha transitória no polling CapCut", "error", err)
				continue
			}

			qBytes, _ := io.ReadAll(qResp.Body)
			qResp.Body.Close()

			var qRes struct {
				Data struct {
					Tasks []struct {
						Status  string `json:"status"`
						Payload string `json:"payload"`
					} `json:"tasks"`
				} `json:"data"`
			}
			if err := json.Unmarshal(qBytes, &qRes); err != nil || len(qRes.Data.Tasks) == 0 {
				continue
			}

			st := qRes.Data.Tasks[0].Status
			if st == "succeed" || st == "success" {
				var pPayload struct {
					AudioSubtitles []struct {
						SpeechURL string `json:"speech_url"`
					} `json:"audio_subtitles"`
				}
				_ = json.Unmarshal([]byte(qRes.Data.Tasks[0].Payload), &pPayload)
				if len(pPayload.AudioSubtitles) == 0 || pPayload.AudioSubtitles[0].SpeechURL == "" {
					return nil, ErrEmptyAudioURL
				}

				audioURL := pPayload.AudioSubtitles[0].SpeechURL
				slog.Info("Áudio CapCut gerado com sucesso",
					"task_id", taskID,
					"voice", voiceItem.VoiceType,
					"emotion", emotion,
					"total_ms", time.Since(startT).Milliseconds())

				// Baixar os bytes de áudio gerados
				return c.downloadAudio(ctx, audioURL)
			} else if st == "failed" || st == "fail" {
				return nil, fmt.Errorf("%w: status %s na task %s", ErrTaskFailed, st, taskID)
			}
		}
	}
}

// downloadAudio baixa o arquivo de áudio diretamente do CDN do TikTok/ByteDance.
func (c *Client) downloadAudio(ctx context.Context, audioURL string) ([]byte, error) {
	// Tentativas de download com perfis de cabeçalhos comuns (sem referer restritivo)
	headerProfiles := []map[string]string{
		{
			"User-Agent": "Cronet/TTNetVersion:1d7cc3b1 2025-07-16 QuicVersion:52c2b40d 2025-04-03",
			"Accept":     "*/*",
		},
		{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Accept":     "*/*",
		},
		{
			"Accept": "*/*",
		},
	}

	var lastErr error
	for _, headers := range headerProfiles {
		dlReq, err := http.NewRequestWithContext(ctx, "GET", audioURL, nil)
		if err != nil {
			return nil, fmt.Errorf("falha ao criar requisição de download de áudio: %w", err)
		}

		for k, v := range headers {
			dlReq.Header.Set(k, v)
		}

		dlResp, err := c.httpClient.Do(dlReq)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "connection reset") {
				lastErr = fmt.Errorf("conexão com o CDN rejeitada/fechada (bloqueio do firewall de rede/FortiGate para tiktokcdn.com): %w", err)
			} else {
				lastErr = fmt.Errorf("falha de rede ao baixar áudio do CDN: %w", err)
			}
			continue
		}

		if dlResp.StatusCode == http.StatusOK {
			audioBytes, rErr := io.ReadAll(dlResp.Body)
			dlResp.Body.Close()
			if rErr != nil {
				return nil, fmt.Errorf("falha ao ler bytes do áudio: %w", rErr)
			}
			prefixCheck := string(audioBytes[:min(len(audioBytes), 256)])
			if strings.Contains(prefixCheck, "<!DOCTYPE") || strings.Contains(prefixCheck, "<html") {
				return nil, fmt.Errorf("provedor de voz retornou conteúdo inválido")
			}
			return StripID3(audioBytes), nil
		}

		_, _ = io.ReadAll(io.LimitReader(dlResp.Body, 1024))
		dlResp.Body.Close()
		slog.Warn("Provedor de voz rejeitou o download do áudio", "status", dlResp.StatusCode)
		lastErr = fmt.Errorf("provedor de voz indisponível (status HTTP %d)", dlResp.StatusCode)
	}

	return nil, lastErr
}
