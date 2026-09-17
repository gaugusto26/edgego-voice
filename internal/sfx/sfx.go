package sfx

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SFXType identifica o tipo de micro-efeito sonoro
type SFXType string

const (
	SFXKeyboard   SFXType = "keyboard"   // Efeito Teclado
	SFXCallcenter SFXType = "callcenter" // Callcenter Ambiente
	SFXNoise      SFXType = "noise"      // Ruído de fundo
)

// isMaleVoice verifica se a voz selecionada é masculina para direcionamento de gênero dos efeitos.
func isMaleVoice(voice string) bool {
	v := strings.ToLower(voice)
	maleKeywords := []string{
		"antonio", "antônio", "donato", "fabio", "fábio", "humberto", "julio", "júlio",
		"nicolau", "valerio", "valério", "macerio", "pedro", "rafael", "caio",
		"andrew", "brian", "william", "giuseppe", "remy", "florian", "davis", "jason", "tony",
	}
	for _, m := range maleKeywords {
		if strings.Contains(v, m) {
			return true
		}
	}
	return false
}

// NormalizeSFXType converte apelidos e tags de efeitos para SFXType
func NormalizeSFXType(name string) (SFXType, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.ReplaceAll(lower, " ", "_")
	switch lower {
	case "hum", "hmm", "filler:hmm", "filler:hum":
		return SFXType("hum"), true
	case "entendi", "filler:entendi":
		return SFXType("entendi"), true
	case "certo", "filler:certo":
		return SFXType("certo"), true
	case "deixa-ver", "filler:deixa-ver", "deixaver":
		return SFXType("deixa-ver"), true
	case "teclado", "efeito-teclado", "digitando", "keyboard", "typing":
		return SFXKeyboard, true
	case "callcenter", "callcenter-ambiente", "ambiente", "escritorio", "escritório":
		return SFXCallcenter, true
	case "ruido", "ruído", "ruido-fundo", "chiado", "noise":
		return SFXNoise, true
	default:
		return SFXType(lower), false
	}
}

// buildCandidateFileNames gera nomes de arquivos candidatos baseados no efeito e no contexto da voz
func buildCandidateFileNames(name, voice string) []string {
	cleanName := strings.TrimSpace(strings.ToLower(name))
	cleanName = strings.ReplaceAll(cleanName, "_", "-")

	var candidates []string

	// Se o efeito for teclado:
	if strings.Contains(cleanName, "teclado") || cleanName == "digitando" || cleanName == "keyboard" {
		candidates = append(candidates, "efeito-teclado", "teclado")
	}

	// Se o efeito for callcenter / ambiente:
	if strings.Contains(cleanName, "callcenter") || strings.Contains(cleanName, "ambiente") {
		candidates = append(candidates, "callcenter-ambiente", "callcenter", "ambiente")
	}

	// Se o efeito for ruído de fundo:
	if strings.Contains(cleanName, "ruido") || strings.Contains(cleanName, "ruído") || strings.Contains(cleanName, "noise") {
		candidates = append(candidates, "ruido-fundo", "ruido")
	}

	// Padrões específicos por voz completa ou curta (ex: pt-BR-FranciscaNeural_tosse, Francisca_tosse)
	if voice != "" {
		candidates = append(candidates, voice+"-"+cleanName, voice+"_"+cleanName)
		parts := strings.Split(voice, "-")
		if len(parts) >= 3 {
			shortVoice := parts[2]
			candidates = append(candidates, shortVoice+"-"+cleanName, shortVoice+"_"+cleanName)
		}
	}

	// Nome exato passado na tag
	candidates = append(candidates, cleanName)

	return candidates
}

// LoadCustomSFX tenta carregar um arquivo de áudio de efeito customizado na pasta ./sfx ou /app/sfx.
func LoadCustomSFX(name, voice string) ([]byte, bool) {
	cleanName := strings.TrimSpace(strings.ToLower(name))
	if cleanName == "" {
		return nil, false
	}

	searchDirs := []string{
		"sfx",
		"./sfx",
		"/app/sfx",
	}

	exts := []string{".wav", ".mp3", ".ogg", ".opus", ".aac"}
	candidates := buildCandidateFileNames(cleanName, voice)

	for _, dir := range searchDirs {
		for _, cand := range candidates {
			for _, ext := range exts {
				path := filepath.Join(dir, cand+ext)
				if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
					slog.Info("Efeito SFX carregado com sucesso do disco", "arquivo", path, "efeito", name, "voz", voice)
					return data, true
				}
			}
		}
	}

	return nil, false
}

// ConvertAudioBytesToFormat converte dados de áudio em memória para o formato especificado usando os parâmetros exatos do Edge TTS.
func ConvertAudioBytesToFormat(audio []byte, format string) ([]byte, error) {
	if len(audio) == 0 {
		return audio, nil
	}

	lowerFormat := strings.ToLower(format)
	// Se for WAV e já tiver cabeçalho RIFF, retorna direto sem chamar ffmpeg
	if (strings.Contains(lowerFormat, "wav") || strings.Contains(lowerFormat, "pcm")) && len(audio) > 4 && string(audio[:4]) == "RIFF" {
		return audio, nil
	}

	// Se for MP3 e já for MP3 (começa com sync word 0xFF 0xFB/F3/F2 ou ID3), retorna direto
	if strings.Contains(lowerFormat, "mp3") && len(audio) > 3 {
		if (audio[0] == 0xFF && (audio[1]&0xE0) == 0xE0) || (audio[0] == 'I' && audio[1] == 'D' && audio[2] == '3') {
			return audio, nil
		}
	}

	audioParams := GetAudioParams(format)
	args := []string{"-y", "-i", "pipe:0"}
	args = append(args, audioParams...)
	args = append(args, "pipe:1")

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(audio)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Warn("Falha ao converter áudio via ffmpeg, retornando buffer original", "error", err, "stderr", stderr.String())
		return audio, nil
	}

	return stdout.Bytes(), nil
}

// GetAudioForSFX carrega do arquivo customizado do usuário em ./sfx ou gera forma de onda acústica.
func GetAudioForSFX(sfxName, voice, format string) ([]byte, error) {
	// 1. Tenta carregar do disco (com resolução inteligente de gênero e contexto de voz)
	if customAudio, ok := LoadCustomSFX(sfxName, voice); ok {
		return ConvertAudioBytesToFormat(customAudio, format)
	}

	// 2. Fallback: gera áudio acústico puro sem fala
	sfxType, _ := NormalizeSFXType(sfxName)
	return GenerateAcousticSFX(sfxType, format)
}

// GenerateAcousticSFX gera um bloco de áudio de silêncio acústico puro no formato solicitado.
func GenerateAcousticSFX(sfxType SFXType, format string) ([]byte, error) {
	sampleRate := 24000
	duration := 0.25
	totalSamples := int(float64(sampleRate) * duration)
	pcm := make([]int16, totalSamples)

	wavBytes, err := encodeWAV(pcm, sampleRate)
	if err != nil {
		return nil, err
	}

	lowerFormat := strings.ToLower(format)
	if strings.Contains(lowerFormat, "wav") || strings.Contains(lowerFormat, "pcm") {
		return wavBytes, nil
	}

	return ConvertAudioBytesToFormat(wavBytes, format)
}

func encodeWAV(samples []int16, sampleRate int) ([]byte, error) {
	buf := new(bytes.Buffer)
	numChannels := uint16(1)
	bitsPerSample := uint16(16)
	byteRate := uint32(sampleRate * int(numChannels) * int(bitsPerSample/8))
	blockAlign := uint16(numChannels * (bitsPerSample / 8))
	dataSize := uint32(len(samples) * 2)

	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, numChannels)
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, byteRate)
	_ = binary.Write(buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(buf, binary.LittleEndian, bitsPerSample)

	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, dataSize)
	for _, sample := range samples {
		_ = binary.Write(buf, binary.LittleEndian, sample)
	}

	return buf.Bytes(), nil
}
