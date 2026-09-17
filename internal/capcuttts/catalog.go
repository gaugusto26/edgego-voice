package capcuttts

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed Voice.json
var embeddedVoiceCatalogJSON []byte

// VoiceItem define a estrutura de uma voz no catálogo oficial do CapCut.
type VoiceItem struct {
	Lan         string `json:"lan"`
	Lang        string `json:"lang"`
	VoiceType   string `json:"voice_type"`
	DisplayName string `json:"display_name"`
	ResourceID  string `json:"resource_id"`
	CapturedAt  string `json:"captured_at,omitempty"`
}

// VoiceMetadata contém anotações ricas de personalidade e sentimentos recomendados.
type VoiceMetadata struct {
	Personality         string
	RecommendedEmotions []string
}

// voicePersonalities mapeia traços e sentimentos nativos conhecidos das vozes.
var voicePersonalities = map[string]VoiceMetadata{
	"DiT_pt_male_wenrou": {
		Personality:         "Voz suave, carinhosa, acolhedora e empática. Perfeita para suporte humanizado e tranquilização.",
		RecommendedEmotions: []string{"calm", "happy"},
	},
	"DiT_pt_male_shangren": {
		Personality:         "Voz séria, corporativa, firme e calculista. Ideal para cobrança, finanças e autoridade.",
		RecommendedEmotions: []string{"angry", "calm", "neutral"},
	},
	"ICL_br_male_212M_LuizS": {
		Personality:         "Voz forte, vibrante e dinâmica. Estilo locutor de rádio, podcast e comerciais de impacto.",
		RecommendedEmotions: []string{"happy", "surprise", "excited"},
	},
	"DiT_pt_female_youya": {
		Personality:         "Voz elegante, aristocrática, serena e refinada. Excelente para produtos de luxo e bem-estar.",
		RecommendedEmotions: []string{"calm", "happy"},
	},
	"en_female_emotional_moon_bigtts": {
		Personality:         "Voz com alta expressividade emocional e dramática.",
		RecommendedEmotions: []string{"sad", "happy", "fear"},
	},
	"en_male_dramaqueen_zachk": {
		Personality:         "Voz teatral, hiperbólica e bem-humorada.",
		RecommendedEmotions: []string{"surprise", "happy", "angry"},
	},
	"BV503_streaming": {
		Personality:         "Voz feminina jovem, energética e vibrante.",
		RecommendedEmotions: []string{"happy", "excited"},
	},
	"en_male_deadpool": {
		Personality:         "Tom irônico, sarcástico e descontraído.",
		RecommendedEmotions: []string{"happy", "angry"},
	},
}

// Catalog gerencia o catálogo em memória com resolução thread-safe.
type Catalog struct {
	mu            sync.RWMutex
	voices        []VoiceItem
	byVoiceType   map[string]VoiceItem
	byDisplayName map[string]VoiceItem
	byResourceID  map[string]VoiceItem
}

var (
	defaultCatalogInstance *Catalog
	defaultCatalogOnce     sync.Once
)

// DefaultCatalog retorna a instância singleton do catálogo de vozes embutido.
func DefaultCatalog() *Catalog {
	defaultCatalogOnce.Do(func() {
		defaultCatalogInstance = NewCatalogFromBytes(embeddedVoiceCatalogJSON)
	})
	return defaultCatalogInstance
}

// NewCatalogFromBytes inicializa um catálogo a partir de dados JSON.
func NewCatalogFromBytes(data []byte) *Catalog {
	var list []VoiceItem
	_ = json.Unmarshal(data, &list)

	c := &Catalog{
		voices:        list,
		byVoiceType:   make(map[string]VoiceItem, len(list)),
		byDisplayName: make(map[string]VoiceItem, len(list)),
		byResourceID:  make(map[string]VoiceItem, len(list)),
	}

	for _, v := range list {
		c.byVoiceType[strings.ToLower(v.VoiceType)] = v
		c.byDisplayName[strings.ToLower(v.DisplayName)] = v
		c.byResourceID[v.ResourceID] = v
	}

	return c
}

// ResolveVoice busca por voice_type, display_name, resource_id ou alias comum.
func (c *Catalog) ResolveVoice(query string) (VoiceItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		// Padrão: Wenrou
		if v, ok := c.byVoiceType["dit_pt_male_wenrou"]; ok {
			return v, true
		}
	}

	// 1. Apelidos amigáveis conhecidos
	switch q {
	case "wenrou", "capcut-wenrou", "suave", "carinhoso":
		q = "dit_pt_male_wenrou"
	case "empresario", "empresario-frio", "capcut-empresario", "frio":
		q = "dit_pt_male_shangren"
	case "contra", "luiz", "luizs", "capcut-luiz", "locutor":
		q = "icl_br_male_212m_luizs"
	case "youya", "capcut-youya", "elegante":
		q = "dit_pt_female_youya"
	case "emotional", "capcut-emotional":
		q = "en_female_emotional_moon_bigtts"
	case "dramaaa", "drama", "capcut-drama":
		q = "en_male_dramaqueen_zachk"
	}

	// 2. Busca exata por voice_type
	if v, ok := c.byVoiceType[q]; ok {
		return v, true
	}

	// 3. Busca por display_name
	if v, ok := c.byDisplayName[q]; ok {
		return v, true
	}

	// 4. Busca por resource_id
	if v, ok := c.byResourceID[query]; ok {
		return v, true
	}

	// 5. Busca parcial se houver prefixo
	for k, v := range c.byVoiceType {
		if strings.Contains(k, q) {
			return v, true
		}
	}

	return VoiceItem{}, false
}

// IsCapCutVoice verifica se o identificador pertence ao ecossistema do CapCut.
func (c *Catalog) IsCapCutVoice(voice string) bool {
	vLower := strings.ToLower(strings.TrimSpace(voice))
	if strings.HasPrefix(vLower, "dit_") ||
		strings.HasPrefix(vLower, "icl_") ||
		strings.HasPrefix(vLower, "bv") ||
		strings.HasPrefix(vLower, "multi_") ||
		strings.HasPrefix(vLower, "capcut") ||
		strings.Contains(vLower, "streaming") ||
		strings.Contains(vLower, "bigtts") ||
		vLower == "wenrou" ||
		vLower == "empresario" ||
		vLower == "luiz" ||
		vLower == "youya" ||
		vLower == "emotional" ||
		vLower == "dramaaa" {
		return true
	}

	_, found := c.ResolveVoice(voice)
	return found
}

// GetMetadata retorna a personalidade e sentimentos recomendados para a voz.
func GetMetadata(voiceType string) VoiceMetadata {
	if meta, ok := voicePersonalities[voiceType]; ok {
		return meta
	}
	return VoiceMetadata{
		Personality:         "Expressividade neural com modulação natural de sentimento.",
		RecommendedEmotions: []string{"neutral", "happy", "calm"},
	}
}

// AllVoices retorna todas as vozes do catálogo.
func (c *Catalog) AllVoices() []VoiceItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	copied := make([]VoiceItem, len(c.voices))
	copy(copied, c.voices)
	return copied
}
