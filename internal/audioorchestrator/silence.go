package audioorchestrator

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"
)

// mp3SilenceFrame24kHz64kbpsMono representa um frame MPEG-2 Layer III válido de 24kHz 64kbps Mono (CapCut TTS)
// Duração de cada frame: 576 amostras / 24000 Hz = 24ms (192 bytes por frame).
var mp3SilenceFrame24kHz64kbpsMono = func() []byte {
	frame := make([]byte, 192)
	// Header MPEG 2 Layer III, 64kbps (0x8), 24kHz (0x4), Mono (0xC4), No CRC: 0xFF, 0xF3, 0x84, 0xC4
	frame[0] = 0xFF
	frame[1] = 0xF3
	frame[2] = 0x84
	frame[3] = 0xC4
	return frame
}()

// mp3SilenceFrame24kHz48kbpsMono representa um frame MPEG-2 Layer III válido de 24kHz 48kbps Mono (Edge TTS)
// Duração de cada frame: 576 amostras / 24000 Hz = 24ms (144 bytes por frame).
var mp3SilenceFrame24kHz48kbpsMono = func() []byte {
	frame := make([]byte, 144)
	// Header MPEG 2 Layer III, 48kbps (0x6), 24kHz (0x4), Mono (0xC4), No CRC: 0xFF, 0xF3, 0x64, 0xC4
	frame[0] = 0xFF
	frame[1] = 0xF3
	frame[2] = 0x64
	frame[3] = 0xC4
	return frame
}()

// mp3SilenceFrame44kHz128kbps representa um frame MPEG-1 Layer III padrão de 44.1kHz 128kbps
// Duração: 1152 amostras / 44100 Hz ≈ 26.12ms (417 bytes por frame).
var mp3SilenceFrame44kHz128kbps = func() []byte {
	frame := make([]byte, 417)
	// Header MPEG 1 Layer III, 128kbps, 44.1kHz, Joint Stereo, No CRC: 0xFF, 0xFB, 0x90, 0x64
	frame[0] = 0xFF
	frame[1] = 0xFB
	frame[2] = 0x90
	frame[3] = 0x64
	return frame
}()

// StripID3 remove o cabeçalho ID3v2 de dados de áudio MP3, se presente.
func StripID3(data []byte) []byte {
	if len(data) >= 10 && data[0] == 'I' && data[1] == 'D' && data[2] == '3' {
		size := int(data[6]&0x7F)<<21 | int(data[7]&0x7F)<<14 | int(data[8]&0x7F)<<7 | int(data[9]&0x7F)
		total := 10 + size
		if data[5]&0x10 != 0 {
			total += 10
		}
		if len(data) >= total {
			return data[total:]
		}
	}
	return data
}

// isCapCutVoiceIdentifier verifica se a voz pertence ao motor CapCut
func isCapCutVoiceIdentifier(voice string) bool {
	vLower := strings.ToLower(strings.TrimSpace(voice))
	return strings.HasPrefix(vLower, "dit_") ||
		strings.HasPrefix(vLower, "icl_") ||
		strings.HasPrefix(vLower, "bv") ||
		strings.Contains(vLower, "bigtts") ||
		strings.Contains(vLower, "dramaqueen") ||
		vLower == "wenrou" ||
		vLower == "empresario" ||
		vLower == "luiz" ||
		vLower == "youya" ||
		vLower == "emotional" ||
		vLower == "dramaaa"
}

// GenerateSilence gera um buffer de áudio com silêncio puro correspondente à duração e formato solicitados.
func GenerateSilence(format string, duration time.Duration) ([]byte, error) {
	return GenerateSilenceForVoice(format, "", duration)
}

// GenerateSilenceForVoice gera silêncio alinhado aos parâmetros exatos do motor de voz utilizado.
func GenerateSilenceForVoice(format, voice string, duration time.Duration) ([]byte, error) {
	if duration <= 0 {
		return nil, nil
	}

	isCapCut := isCapCutVoiceIdentifier(voice)

	f := strings.ToLower(strings.TrimSpace(format))
	switch f {
	case "mp3", "audio/mpeg":
		return generateMP3Silence(duration, isCapCut), nil
	case "wav", "audio/wav":
		return generateWAVSilence(duration, 24000, 1, 16), nil
	case "pcm", "raw":
		return generatePCMSilence(duration, 24000, 1, 16), nil
	case "opus", "ogg", "audio/opus", "audio/ogg":
		return generateMP3Silence(duration, isCapCut), nil
	default:
		return generateMP3Silence(duration, isCapCut), nil
	}
}

// generateMP3Silence gera frames MP3 de silêncio puro para a duração desejada com bitrate e taxa idênticos à voz.
func generateMP3Silence(duration time.Duration, isCapCut bool) []byte {
	frameDuration := 24 * time.Millisecond
	numFrames := int(duration / frameDuration)
	if numFrames <= 0 {
		numFrames = 1
	}

	frame := mp3SilenceFrame24kHz48kbpsMono
	if isCapCut {
		frame = mp3SilenceFrame24kHz64kbpsMono
	}

	frameLen := len(frame)
	buf := make([]byte, numFrames*frameLen)
	for i := 0; i < numFrames; i++ {
		copy(buf[i*frameLen:], frame)
	}
	return buf
}

// generatePCMSilence gera bytes zero (0x00) para áudio PCM puro.
func generatePCMSilence(duration time.Duration, sampleRate, channels, bitsPerSample int) []byte {
	bytesPerSec := sampleRate * channels * (bitsPerSample / 8)
	totalBytes := int(float64(bytesPerSec) * duration.Seconds())
	if totalBytes <= 0 {
		totalBytes = 2
	}
	return make([]byte, totalBytes)
}

// generateWAVSilence gera um arquivo WAV válido contendo silêncio puro.
func generateWAVSilence(duration time.Duration, sampleRate, channels, bitsPerSample int) []byte {
	pcmData := generatePCMSilence(duration, sampleRate, channels, bitsPerSample)
	dataLen := uint32(len(pcmData))

	buf := new(bytes.Buffer)
	buf.Grow(44 + len(pcmData))

	// RIFF header
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVE")

	// fmt subchunk
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16)) // Subchunk1Size
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))  // AudioFormat (1 = PCM)
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	byteRate := uint32(sampleRate * channels * (bitsPerSample / 8))
	_ = binary.Write(buf, binary.LittleEndian, byteRate)
	blockAlign := uint16(channels * (bitsPerSample / 8))
	_ = binary.Write(buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	// data subchunk
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, dataLen)
	buf.Write(pcmData)

	return buf.Bytes()
}
