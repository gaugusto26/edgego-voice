package edgetts

// FormatInfo armazena o formato do Edge TTS e o MIME type correspondente.
type FormatInfo struct {
	EdgeFormat string
	MimeType   string
}

var formatMap = map[string]FormatInfo{
	"mp3": {
		EdgeFormat: "audio-24khz-48kbitrate-mono-mp3",
		MimeType:   "audio/mpeg",
	},
	"opus": {
		EdgeFormat: "webm-24khz-16bit-mono-opus",
		MimeType:   "audio/webm; codecs=opus",
	},
	// WhatsApp exige OGG/Opus. O Edge nao emite OGG: o servico converte o WebM
	// com ffmpeg antes de responder.
	"ogg": {
		EdgeFormat: "webm-24khz-16bit-mono-opus",
		MimeType:   "audio/ogg; codecs=opus",
	},
	"wav": {
		EdgeFormat: "riff-24khz-16bit-mono-pcm",
		MimeType:   "audio/wav",
	},
	"pcm": {
		EdgeFormat: "raw-24khz-16bit-mono-pcm",
		MimeType:   "audio/L16",
	},
	"aac": {
		EdgeFormat: "audio-24khz-48kbitrate-mono-mp3",
		MimeType:   "audio/aac",
	},
	"flac": {
		EdgeFormat: "riff-24khz-16bit-mono-pcm",
		MimeType:   "audio/flac",
	},
}

// GetFormatInfo retorna a configuração de áudio do Edge e MIME type para o formato solicitado.
func GetFormatInfo(format string) FormatInfo {
	if info, ok := formatMap[format]; ok {
		return info
	}
	return formatMap["mp3"]
}
