package sfx

import (
	"bytes"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FindSFXFilePath localiza o arquivo de áudio de efeito no sistema de arquivos.
func FindSFXFilePath(name, voice string) (string, bool) {
	cleanName := strings.TrimSpace(strings.ToLower(name))
	if cleanName == "" {
		return "", false
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
				if _, err := os.Stat(path); err == nil {
					return path, true
				}
			}
		}
	}

	return "", false
}

// GetAudioParams retorna os argumentos de formatação ffmpeg rigorosamente alinhados com o Edge TTS.
// No caso do padrão MP3, garante 24000 Hz, Mono e 48kbps para concatenação sem cortes ou saltos.
func GetAudioParams(format string) []string {
	lower := strings.ToLower(format)
	switch {
	case strings.Contains(lower, "wav") || strings.Contains(lower, "pcm"):
		return []string{"-ar", "24000", "-ac", "1", "-c:a", "pcm_s16le", "-f", "wav"}
	case strings.Contains(lower, "opus") || strings.Contains(lower, "webm"):
		return []string{"-ar", "24000", "-ac", "1", "-b:a", "32k", "-c:a", "libopus", "-f", "opus"}
	case strings.Contains(lower, "ogg"):
		return []string{"-ar", "24000", "-ac", "1", "-b:a", "48k", "-c:a", "libvorbis", "-f", "ogg"}
	case strings.Contains(lower, "aac"):
		return []string{"-ar", "24000", "-ac", "1", "-b:a", "48k", "-c:a", "aac", "-f", "adts"}
	default:
		// Padrão exato: audio-24khz-mono-mp3 sem headers de container que corrompem concatenação
		return []string{"-ar", "24000", "-ac", "1", "-b:a", "64k", "-c:a", "libmp3lame", "-write_xing", "0", "-id3v2_version", "0", "-f", "mp3"}
	}
}

// FindAllKeyboardSamples localiza todos os arquivos de teclado disponíveis na pasta sfx
func FindAllKeyboardSamples() []string {
	searchDirs := []string{"sfx", "./sfx", "/app/sfx"}
	names := []string{
		"teclado-to-spech.wav",
		"teclado-to-spech2.wav",
		"efeito-teclado.wav",
	}

	var found []string
	for _, name := range names {
		for _, dir := range searchDirs {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				found = append(found, p)
				break
			}
		}
	}
	return found
}

// GenerateHumanTypingSequence cria uma composição procedural realista de digitação humana
// alternando entre os samples com rajadas rápidas e micro-pausas naturais de reflexão/leitura na tela.
func GenerateHumanTypingSequence(duration time.Duration, format string, volume float64) ([]byte, error) {
	samples := FindAllKeyboardSamples()
	if len(samples) == 0 {
		return nil, fmt.Errorf("nenhum arquivo de teclado encontrado")
	}

	if duration <= 0 {
		duration = 1 * time.Second
	}
	if volume <= 0 {
		volume = 1.0
	}

	targetDuration := duration.Seconds()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	var filterInputs []string
	var filterParts []string
	currTime := 0.0
	inputIdx := 0

	for currTime < targetDuration {
		// 1. Rajada de digitação (sorteia um dos samples e dura entre 0.35s e 0.70s)
		sample := samples[r.Intn(len(samples))]
		burstDur := 0.35 + r.Float64()*0.35
		if currTime+burstDur > targetDuration {
			burstDur = targetDuration - currTime
		}

		filterInputs = append(filterInputs, "-t", fmt.Sprintf("%.2f", burstDur), "-i", sample)
		filterParts = append(filterParts, fmt.Sprintf("[%d:a]", inputIdx))
		inputIdx++
		currTime += burstDur

		if currTime >= targetDuration {
			break
		}

		// 2. Micro-pausa humana (o atendente olha para o campo na tela ou confere o dado: 250ms a 500ms)
		pauseDur := 0.25 + r.Float64()*0.25
		if currTime+pauseDur > targetDuration {
			pauseDur = targetDuration - currTime
		}

		filterInputs = append(filterInputs, "-f", "lavfi", "-t", fmt.Sprintf("%.2f", pauseDur), "-i", "anullsrc=r=24000:cl=mono")
		filterParts = append(filterParts, fmt.Sprintf("[%d:a]", inputIdx))
		inputIdx++
		currTime += pauseDur
	}

	concatFilter := fmt.Sprintf("%sconcat=n=%d:v=0:a=1,volume=%.2f[out]", strings.Join(filterParts, ""), inputIdx, volume)
	audioParams := GetAudioParams(format)

	args := []string{"-y"}
	args = append(args, filterInputs...)
	args = append(args,
		"-filter_complex", concatFilter,
		"-map", "[out]",
	)
	args = append(args, audioParams...)
	args = append(args, "pipe:1")

	cmd := exec.Command("ffmpeg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Info("Gerando digitação humana procedural com rajadas e pausas", "duracao_seg", targetDuration, "bursts", inputIdx, "volume", volume)
	if err := cmd.Run(); err != nil {
		slog.Warn("Falha ao gerar digitação procedural via ffmpeg, executando fallback", "error", err, "stderr", stderr.String())
		return nil, err
	}

	return stdout.Bytes(), nil
}

// GenerateDurationSFXWithVolume gera um efeito em loop contínuo com ganho específico e mesma taxa de amostragem.
func GenerateDurationSFXWithVolume(name, voice, format string, duration time.Duration, volume float64) ([]byte, error) {
	// Se for efeito de teclado, utiliza a simulação de digitação humana procedural com pausas e alternância
	if strings.Contains(strings.ToLower(name), "teclado") || strings.Contains(strings.ToLower(name), "digitando") {
		if humanAudio, err := GenerateHumanTypingSequence(duration, format, volume); err == nil && len(humanAudio) > 0 {
			return humanAudio, nil
		}
	}

	filePath, found := FindSFXFilePath(name, voice)
	if !found {
		return GetAudioForSFX(name, voice, format)
	}

	if duration <= 0 {
		duration = 1 * time.Second
	}
	if volume <= 0 {
		volume = 1.0
	}

	seconds := duration.Seconds()
	audioParams := GetAudioParams(format)

	filter := fmt.Sprintf("[0:a]volume=%.2f[a]", volume)

	args := []string{
		"-y",
		"-stream_loop", "-1",
		"-i", filePath,
		"-t", fmt.Sprintf("%.2f", seconds),
		"-filter_complex", filter,
		"-map", "[a]",
	}
	args = append(args, audioParams...)
	args = append(args, "pipe:1")

	cmd := exec.Command("ffmpeg", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Info("Gerando SFX em loop com duração", "sfx", name, "arquivo", filePath, "duracao_seg", seconds, "volume", volume)
	if err := cmd.Run(); err != nil {
		slog.Warn("Falha ao gerar SFX com duração via ffmpeg, executando fallback", "error", err, "stderr", stderr.String())
		return GetAudioForSFX(name, voice, format)
	}

	return stdout.Bytes(), nil
}

// GenerateDurationSFX gera um áudio de efeito que se repete em loop pelo tempo exato especificado.
func GenerateDurationSFX(name, voice, format string, duration time.Duration) ([]byte, error) {
	return GenerateDurationSFXWithVolume(name, voice, format, duration, 1.0)
}

// MixAmbientTrack sobrepõe o som de ambiente ao fundo da fala com ganho equilibrado e taxa alinhada a 24kHz.
func MixAmbientTrack(speechAudio []byte, ambientName, voice, format string, volume float64) ([]byte, error) {
	if len(speechAudio) == 0 {
		return speechAudio, nil
	}

	filePath, found := FindSFXFilePath(ambientName, voice)
	if !found {
		slog.Warn("Arquivo de ambiente não localizado no disco", "ambiente", ambientName, "voz", voice)
		return speechAudio, nil
	}

	if volume <= 0 {
		volume = 0.45 // Volume confortável de fundo (audível, sem competir com a fala)
	}

	slog.Info("Mixando trilha de ambiente com fala", "ambiente", ambientName, "arquivo", filePath, "volume", volume)

	audioParams := GetAudioParams(format)
	filter := fmt.Sprintf("[1:a]volume=%.2f[bg];[0:a][bg]amix=inputs=2:duration=first:dropout_transition=0:normalize=0", volume)

	args := []string{
		"-y",
		"-i", "pipe:0",
		"-stream_loop", "-1",
		"-i", filePath,
		"-filter_complex", filter,
	}
	args = append(args, audioParams...)
	args = append(args, "pipe:1")

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(speechAudio)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Warn("Falha ao mixar áudio ambiente com ffmpeg, mantendo áudio original", "error", err, "stderr", stderr.String())
		return speechAudio, nil
	}

	return stdout.Bytes(), nil
}

// ApplyTelephonyFilter aplica o processamento acústico DSP de canal telefônico (PSTN / G.711 / 3GPP):
// - Filtro passa-faixa estrito de 300Hz a 3400Hz (elimina o efeito de "voz de estúdio limpa demais").
// - Equalização de presença telefônica em 2.5kHz.
// - Compressão dinâmica suave para nivelamento de microfone headset de atendimento.
func ApplyTelephonyFilter(speechAudio []byte, format string) ([]byte, error) {
	if len(speechAudio) == 0 {
		return speechAudio, nil
	}

	audioParams := GetAudioParams(format)
	// Filtro DSP: passa-alta 300Hz + passa-baixa 3400Hz + pico de presença em 2.5kHz + compressor suave
	telephonyFilter := "highpass=f=300,lowpass=f=3400,equalizer=f=2500:t=q:w=1.2:g=2,acompressor=threshold=-14dB:ratio=2.5:attack=5:release=50"

	args := []string{
		"-y",
		"-i", "pipe:0",
		"-af", telephonyFilter,
	}
	args = append(args, audioParams...)
	args = append(args, "pipe:1")

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(speechAudio)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Info("Aplicando filtro acústico DSP de telefonia na voz", "tamanho_bytes", len(speechAudio))
	if err := cmd.Run(); err != nil {
		slog.Warn("Falha ao aplicar filtro DSP de telefonia, mantendo áudio original", "error", err, "stderr", stderr.String())
		return speechAudio, nil
	}

	return stdout.Bytes(), nil
}

