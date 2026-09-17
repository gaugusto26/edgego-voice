package audioorchestrator

import (
	"encoding/binary"
	"strings"
)

// ExactAudioDurationMs calcula a duração real e precisa de um buffer de áudio em milissegundos
// inspecionando os cabeçalhos de frames de MP3 ou chunks WAV.
func ExactAudioDurationMs(audio []byte, format string) int64 {
	if len(audio) == 0 {
		return 0
	}

	switch strings.ToLower(format) {
	case "wav":
		if dur := parseWAVDurationMs(audio); dur > 0 {
			return dur
		}
	default: // mp3, ou outros formatos codificados em mp3
		if dur := parseMP3DurationMs(audio); dur > 0 {
			return dur
		}
	}

	// Fallback para a estimativa heurística se os headers estiverem corrompidos
	return EstimateAudioDurationMs(audio, format)
}

// parseWAVDurationMs extrai a duração exata lendo os chunks 'fmt ' e 'data' do cabeçalho RIFF.
func parseWAVDurationMs(data []byte) int64 {
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0
	}

	var byteRate uint32
	var dataSize uint32
	idx := 12

	for idx+8 <= len(data) {
		chunkID := string(data[idx : idx+4])
		chunkSize := binary.LittleEndian.Uint32(data[idx+4 : idx+8])
		idx += 8

		if chunkID == "fmt " && chunkSize >= 16 && idx+16 <= len(data) {
			byteRate = binary.LittleEndian.Uint32(data[idx+8 : idx+12])
		} else if chunkID == "data" {
			dataSize = chunkSize
			if int(dataSize) > len(data)-idx {
				dataSize = uint32(len(data) - idx)
			}
			break
		}

		idx += int(chunkSize)
	}

	if byteRate > 0 && dataSize > 0 {
		return int64((uint64(dataSize) * 1000) / uint64(byteRate))
	}
	return 0
}

// Tabelas de Bitrate para MPEG-1 e MPEG-2/2.5 Layer III (em kbps)
var mp3BitrateTable = map[int][16]int{
	// MPEG-1 Layer III
	1: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
	// MPEG-2 / 2.5 Layer III
	2: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
}

// Tabelas de Amostragem (Hz)
var mp3SampleRateTable = map[int][4]int{
	1:   {44100, 48000, 32000, 0}, // MPEG-1
	2:   {22050, 24000, 16000, 0}, // MPEG-2
	25:  {11025, 12000, 8000, 0},  // MPEG-2.5
}

// parseMP3DurationMs itera por todos os frames MPEG Layer III somando as amostras
// para obter o tempo de reprodução exato sem suposições de bitrate constante.
func parseMP3DurationMs(data []byte) int64 {
	n := len(data)
	if n < 4 {
		return 0
	}

	idx := 0

	// 1. Pular tag ID3v2 inicial se existir
	if n >= 10 && data[0] == 'I' && data[1] == 'D' && data[2] == '3' {
		// O tamanho do ID3v2 é codificado como 4 bytes Synchsafe (7 bits cada)
		id3Size := (int(data[6]&0x7F) << 21) |
			(int(data[7]&0x7F) << 14) |
			(int(data[8]&0x7F) << 7) |
			int(data[9]&0x7F)
		idx = 10 + id3Size
	}

	var totalSamples int64 = 0
	var lastSampleRate int = 0
	validFrames := 0

	for idx+4 <= n {
		b0 := data[idx]
		b1 := data[idx+1]

		// Syncword: 11 bits consecutivos em 1 (0xFFE0)
		if b0 == 0xFF && (b1&0xE0) == 0xE0 {
			rawVer := (b1 >> 3) & 0x03
			rawLayer := (b1 >> 1) & 0x03

			// Queremos Layer III (rawLayer == 1)
			if rawLayer == 1 {
				var mpegVer int
				switch rawVer {
				case 3:
					mpegVer = 1   // MPEG-1
				case 2:
					mpegVer = 2   // MPEG-2
				case 0:
					mpegVer = 25  // MPEG-2.5
				default:
					idx++
					continue
				}

				b2 := data[idx+2]
				bitrateIdx := int((b2 >> 4) & 0x0F)
				sampleRateIdx := int((b2 >> 2) & 0x03)
				padding := int((b2 >> 1) & 0x01)

				// Validar índices
				tableKey := 1
				if mpegVer != 1 {
					tableKey = 2
				}
				bitrates := mp3BitrateTable[tableKey]
				sampleRates := mp3SampleRateTable[mpegVer]

				if bitrateIdx > 0 && bitrateIdx < 15 && sampleRateIdx < 3 {
					bitrate := bitrates[bitrateIdx] * 1000
					sampleRate := sampleRates[sampleRateIdx]

					if bitrate > 0 && sampleRate > 0 {
						var samplesPerFrame int
						var frameSize int

						if mpegVer == 1 {
							samplesPerFrame = 1152
							frameSize = (144 * bitrate / sampleRate) + padding
						} else {
							samplesPerFrame = 576
							frameSize = (72 * bitrate / sampleRate) + padding
						}

						if frameSize > 4 && idx+frameSize <= n {
							totalSamples += int64(samplesPerFrame)
							lastSampleRate = sampleRate
							validFrames++
							idx += frameSize
							continue
						}
					}
				}
			}
		}

		idx++
	}

	if validFrames > 0 && lastSampleRate > 0 {
		return (totalSamples * 1000) / int64(lastSampleRate)
	}

	return 0
}
