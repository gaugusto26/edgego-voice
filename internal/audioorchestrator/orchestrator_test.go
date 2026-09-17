package audioorchestrator

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"edgego-voice/internal/edgetts"
)

func TestParseSegments(t *testing.T) {
	text := "Olá! [pausa: 2s] Este é o primeiro teste. [voz: Antonio] [velocidade: 1.2x] Agora falando mais rápido com outra voz! [pause: 500ms] <break time=\"1.5s\"/> Finalizando."

	voiceMap := map[string]string{
		"pt-BR-AntonioNeural": "Antonio",
		"pt-BR-ThalitaMultilingualNeural": "Thalita",
	}

	segments := ParseSegments(text, "pt-BR-ThalitaMultilingualNeural", "+0%", "+0Hz", voiceMap)

	if len(segments) != 7 {
		t.Fatalf("Esperava 7 segmentos, obteve %d: %+v", len(segments), segments)
	}

	// 1. Fala inicial
	if segments[0].Type != SegmentSpeech || segments[0].Text != "Olá!" || segments[0].Voice != "pt-BR-ThalitaMultilingualNeural" {
		t.Errorf("Segmento 0 incorreto: %+v", segments[0])
	}

	// 2. Silêncio de 2 segundos
	if segments[1].Type != SegmentSilence || segments[1].Duration != 2*time.Second {
		t.Errorf("Segmento 1 (silêncio) incorreto: %+v", segments[1])
	}

	// 3. Fala intermédia
	if segments[2].Type != SegmentSpeech || !strings.Contains(segments[2].Text, "Este é o primeiro teste.") {
		t.Errorf("Segmento 2 incorreto: %+v", segments[2])
	}

	// 4. Fala com voz trocada (Antonio) e velocidade +20%
	if segments[3].Type != SegmentSpeech || segments[3].Voice != "pt-BR-AntonioNeural" || segments[3].Rate != "+20%" {
		t.Errorf("Segmento 3 (troca de voz) incorreto: %+v", segments[3])
	}

	// 5. Pausa de 500ms
	if segments[4].Type != SegmentSilence || segments[4].Duration != 500*time.Millisecond {
		t.Errorf("Segmento 4 (500ms) incorreto: %+v", segments[4])
	}

	// 6. Pausa SSML de 1.5s
	if segments[5].Type != SegmentSilence || segments[5].Duration != 1500*time.Millisecond {
		t.Errorf("Segmento 5 (1.5s SSML) incorreto: %+v", segments[5])
	}

	// 7. Fala final
	if segments[6].Type != SegmentSpeech || segments[6].Text != "Finalizando." {
		t.Errorf("Segmento 6 incorreto: %+v", segments[6])
	}
}

func TestVoiceResolution(t *testing.T) {
	voiceMap := map[string]string{
		"alloy": "pt-BR-ThalitaMultilingualNeural",
		"echo":  "it-IT-GiuseppeMultilingualNeural",
	}

	testCases := []struct {
		input    string
		expected string
	}{
		{"pt-BR-AntonioNeural", "pt-BR-AntonioNeural"},
		{"Antonio", "pt-BR-AntonioNeural"},
		{"Antônio", "pt-BR-AntonioNeural"},
		{"antônio", "pt-BR-AntonioNeural"},
		{"Francisca", "pt-BR-FranciscaNeural"},
		{"Thalita", "pt-BR-ThalitaMultilingualNeural"},
		{"Andrew", "en-US-AndrewMultilingualNeural"},
		{"Álvaro", "es-ES-AlvaroNeural"},
		{"alvaro", "es-ES-AlvaroNeural"},
		{"alloy", "pt-BR-ThalitaMultilingualNeural"},
		{"echo", "it-IT-GiuseppeMultilingualNeural"},
		{"pt-PT-RaquelNeural", "pt-PT-RaquelNeural"},
	}

	fallback := "pt-BR-ThalitaMultilingualNeural"

	for _, tc := range testCases {
		res := resolveVoiceName(tc.input, voiceMap, fallback)
		if res != tc.expected {
			t.Errorf("Para input '%s', esperava '%s', obteve '%s'", tc.input, tc.expected, res)
		}
	}
}

func TestGenerateSilenceMP3(t *testing.T) {
	dur := 1 * time.Second
	silence, err := GenerateSilence("mp3", dur)
	if err != nil {
		t.Fatalf("Erro ao gerar silêncio MP3: %v", err)
	}

	if len(silence) == 0 {
		t.Fatalf("Buffer de silêncio MP3 está vazio")
	}

	// 1s com frames de 24ms (144 bytes) deve ter ~41 frames = ~5904 bytes
	if len(silence) < 5000 {
		t.Errorf("Tamanho do buffer de silêncio menor que o esperado: %d bytes", len(silence))
	}

	// Testar silêncio específico para voz CapCut (64kbps, 192 bytes/frame)
	silenceCapCut, err := GenerateSilenceForVoice("mp3", "wenrou", dur)
	if err != nil {
		t.Fatalf("Erro ao gerar silêncio MP3 para CapCut: %v", err)
	}
	if len(silenceCapCut) < 7000 {
		t.Errorf("Tamanho do silêncio CapCut menor que o esperado: %d bytes", len(silenceCapCut))
	}

	// Header MPEG-2 Layer III 64kbps 24kHz Mono
	if silenceCapCut[0] != 0xFF || silenceCapCut[1] != 0xF3 || silenceCapCut[2] != 0x84 || silenceCapCut[3] != 0xC4 {
		t.Errorf("Header de silêncio CapCut incorreto: %X %X %X %X", silenceCapCut[0], silenceCapCut[1], silenceCapCut[2], silenceCapCut[3])
	}
}

func TestStripID3(t *testing.T) {
	// Cria um ID3v2 fictício de 15 bytes de dados (total 25 bytes)
	fakeID3 := []byte{
		'I', 'D', '3', 0x04, 0x00, 0x00, // ID3v2.4
		0x00, 0x00, 0x00, 0x0F, // Synchsafe size = 15
	}
	fakeID3 = append(fakeID3, make([]byte, 15)...)
	audioPayload := []byte{0xFF, 0xF3, 0x84, 0xC4, 0x01, 0x02, 0x03}
	fullAudio := append(fakeID3, audioPayload...)

	stripped := StripID3(fullAudio)
	if len(stripped) != len(audioPayload) || stripped[0] != 0xFF || stripped[1] != 0xF3 {
		t.Errorf("StripID3 não removeu o header corretamente: %X", stripped)
	}

	// Se não tiver ID3, retorna inalterado
	noID3 := StripID3(audioPayload)
	if len(noID3) != len(audioPayload) {
		t.Errorf("StripID3 alterou áudio sem ID3")
	}
}

func TestGenerateSilenceWAV(t *testing.T) {
	dur := 500 * time.Millisecond
	wavSilence, err := GenerateSilence("wav", dur)
	if err != nil {
		t.Fatalf("Erro ao gerar silêncio WAV: %v", err)
	}

	if len(wavSilence) < 44 {
		t.Fatalf("Header WAV inválido ou curto demais: %d bytes", len(wavSilence))
	}

	if string(wavSilence[0:4]) != "RIFF" || string(wavSilence[8:12]) != "WAVE" {
		t.Errorf("Assinatura WAV inválida")
	}
}

func TestParseEmotionAndSFXSegments(t *testing.T) {
	text := "Olá a todos! [pausa: 500ms] [alegre]Temos excelentes notícias![/alegre] [teclado] Até mais."

	segments := ParseSegments(text, "pt-BR-FranciscaNeural", "+0%", "+0Hz", nil)

	if len(segments) != 5 {
		t.Fatalf("Esperava 5 segmentos, obteve %d: %+v", len(segments), segments)
	}

	// Seg 0: Fala neutra (Edge)
	if segments[0].Provider != ProviderEdge || segments[0].Text != "Olá a todos!" || segments[0].Style != "" {
		t.Errorf("Segmento 0 inválido: %+v", segments[0])
	}

	// Seg 1: Pausa de 500ms
	if segments[1].Type != SegmentSilence || segments[1].Duration != 500*time.Millisecond {
		t.Errorf("Segmento 1 (Pausa) inválido: %+v", segments[1])
	}

	// Seg 2: Fala (Edge)
	if segments[2].Provider != ProviderEdge || segments[2].Text != "Temos excelentes notícias!" {
		t.Errorf("Segmento 2 inválido: %+v", segments[2])
	}

	// Seg 3: SFX Teclado (Direto)
	if segments[3].Type != SegmentSFX || segments[3].Provider != ProviderSFX || segments[3].SFXType != "teclado" {
		t.Errorf("Segmento 3 (SFX teclado) inválido: %+v", segments[3])
	}

	// Seg 4: Fala neutra (Edge)
	if segments[4].Provider != ProviderEdge || segments[4].Text != "Até mais." || segments[4].Style != "" {
		t.Errorf("Segmento 4 inválido: %+v", segments[4])
	}
}

func TestParseAmbientBlockAndDurationSFX(t *testing.T) {
	// 1. Bloco de ambiente envolvente
	text1 := "[callcenter] Olá tudo bem, como posso ajudar você hoje? [/callcenter]"
	segs1 := ParseSegments(text1, "pt-BR-FranciscaNeural", "+0%", "+0Hz", nil)
	if len(segs1) != 1 {
		t.Fatalf("Esperava 1 segmento com trilha ambiente, obteve %d", len(segs1))
	}
	if segs1[0].AmbientTrack != "callcenter" {
		t.Errorf("Esperava AmbientTrack=callcenter, obteve: %s", segs1[0].AmbientTrack)
	}

	// 2. SFX com duração programada
	text2 := "Só um momento vou consultar no sistema [teclado:5s] Pronto!"
	segs2 := ParseSegments(text2, "pt-BR-FranciscaNeural", "+0%", "+0Hz", nil)
	if len(segs2) != 3 {
		t.Fatalf("Esperava 3 segmentos, obteve %d", len(segs2))
	}
	if segs2[1].Type != SegmentSFX || segs2[1].SFXType != "teclado" || segs2[1].Duration != 5*time.Second {
		t.Errorf("Segmento 1 de SFX com duração inválido: %+v", segs2[1])
	}
}

func TestTelephonyTagAndFillers(t *testing.T) {
	text := "[telefone] [callcenter] Olá! [hum] Deixe-me consultar seu protocolo [teclado:2s]. [/callcenter] [/telefone]"
	segs := ParseSegments(text, "pt-BR-FranciscaNeural", "+0%", "+0Hz", nil)

	var speechCount, fillerCount, keyboardCount int
	for _, s := range segs {
		if s.Type == SegmentSpeech {
			speechCount++
			if !s.TelephonyFilter {
				t.Errorf("Segmento de fala deveria ter TelephonyFilter=true: %+v", s)
			}
			if s.AmbientTrack != "callcenter" {
				t.Errorf("Segmento de fala deveria ter AmbientTrack=callcenter: %+v", s)
			}
		}
		if s.Type == SegmentSFX && s.SFXType == "hum" {
			fillerCount++
		}
		if s.Type == SegmentSFX && s.SFXType == "teclado" {
			keyboardCount++
			if !s.TelephonyFilter {
				t.Errorf("Segmento de teclado deveria ter TelephonyFilter=true: %+v", s)
			}
		}
	}

	if speechCount == 0 || fillerCount == 0 || keyboardCount == 0 {
		t.Fatalf("Esperava encontrar fala, filler e teclado. Encontrou speech=%d filler=%d teclado=%d", speechCount, fillerCount, keyboardCount)
	}
}

type mockSynthesizerWithID3 struct{}

func (m *mockSynthesizerWithID3) Synthesize(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, error) {
	// Simula saída do CapCut: ID3v2 header seguido de frames MPEG-2 24kHz 64kbps
	id3Header := []byte{'I', 'D', '3', 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0A}
	id3Header = append(id3Header, make([]byte, 10)...)
	frame := []byte{0xFF, 0xF3, 0x84, 0xC4}
	frame = append(frame, make([]byte, 188)...) // Total 192 bytes
	return append(id3Header, frame...), nil
}

func TestSynthesizeOrchestrated_CapCutWithPause(t *testing.T) {
	mock := &mockSynthesizerWithID3{}
	opts := edgetts.SynthesizeOptions{
		Voice:  "wenrou",
		Format: "mp3",
		Rate:   "1.0",
		Pitch:  "+0Hz",
	}

	text := "Primeira parte. [pausa: 500ms] Segunda parte."
	audio, metrics, err := SynthesizeOrchestrated(context.Background(), mock, text, opts, nil)
	if err != nil {
		t.Fatalf("Falha na síntese orquestrada com pausa CapCut: %v", err)
	}

	if len(audio) == 0 {
		t.Fatalf("Áudio resultante está vazio")
	}

	// O áudio final NÃO PODE conter a sequência 'I', 'D', '3' em nenhuma posição
	if bytes.Contains(audio, []byte("ID3")) {
		t.Errorf("Áudio final contém cabeçalho ID3 residual que quebra o tocador no browser!")
	}

	// O primeiro frame DEVE começar com o sync word MPEG-2 24kHz 64kbps
	if audio[0] != 0xFF || audio[1] != 0xF3 || audio[2] != 0x84 || audio[3] != 0xC4 {
		t.Errorf("Início do áudio não possui o sync word MPEG-2 24kHz 64kbps esperado: %X %X %X %X", audio[0], audio[1], audio[2], audio[3])
	}

	if metrics.EdgeChars == 0 {
		t.Errorf("EdgeChars deveria ser computado")
	}
}

func TestParseSegments_Emotions(t *testing.T) {
	// 1. Tag básica de emoção com fechamento
	text1 := "[emocao:feliz]Hoje eu acordei muito contente![/emocao]"
	segs1 := ParseSegments(text1, "wenrou", "1.0", "+0Hz", nil)
	if len(segs1) != 1 {
		t.Fatalf("Esperava 1 segmento, obteve %d: %+v", len(segs1), segs1)
	}
	if segs1[0].Text != "Hoje eu acordei muito contente!" {
		t.Errorf("Texto contém colchetes ou tag residual: '%s'", segs1[0].Text)
	}
	if segs1[0].Emotion != "happy" {
		t.Errorf("Esperava emotion 'happy', obteve '%s'", segs1[0].Emotion)
	}
	if segs1[0].EmotionScale != "1.0" {
		t.Errorf("Esperava emotion_scale '1.0', obteve '%s'", segs1[0].EmotionScale)
	}

	// 2. Tag com intensidade explícita
	text2 := "[emocao:firme:1.5]Preste atenção no relatório.[/emocao]"
	segs2 := ParseSegments(text2, "shangren", "1.0", "+0Hz", nil)
	if len(segs2) != 1 {
		t.Fatalf("Esperava 1 segmento, obteve %d", len(segs2))
	}
	if segs2[0].Text != "Preste atenção no relatório." {
		t.Errorf("Texto incorreto: '%s'", segs2[0].Text)
	}
	if segs2[0].Emotion != "angry" {
		t.Errorf("Esperava emotion 'angry', obteve '%s'", segs2[0].Emotion)
	}
	if segs2[0].EmotionScale != "1.5" {
		t.Errorf("Esperava emotion_scale '1.5', obteve '%s'", segs2[0].EmotionScale)
	}

	// 3. Múltiplas emoções intercaladas
	text3 := "[emocao:calmo]Fique tranquilo,[/emocao] [emocao:triste]as coisas vão melhorar.[/emocao]"
	segs3 := ParseSegments(text3, "wenrou", "1.0", "+0Hz", nil)
	if len(segs3) != 2 {
		t.Fatalf("Esperava 2 segmentos distintos, obteve %d: %+v", len(segs3), segs3)
	}
	if segs3[0].Emotion != "calm" || strings.Contains(segs3[0].Text, "[") {
		t.Errorf("Segmento 0 incorreto: %+v", segs3[0])
	}
	if segs3[1].Emotion != "sad" || strings.Contains(segs3[1].Text, "[") {
		t.Errorf("Segmento 1 incorreto: %+v", segs3[1])
	}

	// 4. Tag direta [surpreso]
	text4 := "[surpreso]Nossa, que legal![/surpreso]"
	segs4 := ParseSegments(text4, "luiz", "1.0", "+0Hz", nil)
	if len(segs4) != 1 {
		t.Fatalf("Esperava 1 segmento, obteve %d", len(segs4))
	}
	if segs4[0].Emotion != "surprise" || segs4[0].Text != "Nossa, que legal!" {
		t.Errorf("Segmento direto incorreto: %+v", segs4[0])
	}
}

type mockRecordingSynthesizer struct {
	calledOpts []edgetts.SynthesizeOptions
}

func (m *mockRecordingSynthesizer) Synthesize(ctx context.Context, opts edgetts.SynthesizeOptions) ([]byte, error) {
	m.calledOpts = append(m.calledOpts, opts)
	return []byte{0xFF, 0xF3, 0x84, 0xC4}, nil
}

func TestSynthesizeOrchestrated_EmotionPropagation(t *testing.T) {
	mock := &mockRecordingSynthesizer{}
	opts := edgetts.SynthesizeOptions{
		Voice:  "DiT_pt_male_wenrou",
		Format: "mp3",
		Rate:   "1.0",
		Pitch:  "+0Hz",
	}

	text := "[emocao:feliz]Parabéns pela conquista![/emocao]"
	_, _, err := SynthesizeOrchestrated(context.Background(), mock, text, opts, nil)
	if err != nil {
		t.Fatalf("Erro na síntese: %v", err)
	}

	if len(mock.calledOpts) != 1 {
		t.Fatalf("Esperava 1 chamada de síntese, obteve %d", len(mock.calledOpts))
	}

	call := mock.calledOpts[0]
	if call.Emotion != "happy" {
		t.Errorf("Esperava chamada com emotion='happy', obteve '%s'", call.Emotion)
	}
	if call.EmotionScale != "1.0" {
		t.Errorf("Esperava chamada com emotion_scale='1.0', obteve '%s'", call.EmotionScale)
	}
	if call.Text != "Parabéns pela conquista!" {
		t.Errorf("Texto repassado para o TTS ainda possui colchetes: '%s'", call.Text)
	}
}

func TestSynthesizeOrchestratedWithSubtitles_Offsets(t *testing.T) {
	mock := &mockRecordingSynthesizer{}
	opts := edgetts.SynthesizeOptions{
		Voice:  "pt-BR-FranciscaNeural",
		Format: "mp3",
		Rate:   "1.0",
		Pitch:  "+0Hz",
	}

	text := "Primeira frase. [pausa: 2s] Segunda frase."
	_, subResult, _, err := SynthesizeOrchestratedWithSubtitles(context.Background(), mock, text, opts, nil)
	if err != nil {
		t.Fatalf("Erro ao orquestrar com legendas: %v", err)
	}

	if len(subResult.Sentences) < 2 {
		t.Fatalf("Esperava no mínimo 2 sentenças legendadas, obteve %d", len(subResult.Sentences))
	}

	// A segunda sentença deve iniciar após os 2 segundos (2000ms) de pausa
	if subResult.Sentences[1].StartMs < 2000 {
		t.Errorf("Segunda sentença deveria iniciar após 2000ms devido à pausa, mas iniciou em %dms", subResult.Sentences[1].StartMs)
	}

	if !strings.Contains(subResult.SRT, "-->") {
		t.Errorf("SRT gerado inválido: %s", subResult.SRT)
	}
	if !strings.HasPrefix(subResult.VTT, "WEBVTT") {
		t.Errorf("VTT gerado inválido: %s", subResult.VTT)
	}
}




