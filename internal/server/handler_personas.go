package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"edgego-voice/internal/personas"
)

type PersonasHandler struct {
	manager *personas.Manager
}

func NewPersonasHandler(manager *personas.Manager) *PersonasHandler {
	return &PersonasHandler{manager: manager}
}

// ListPersonasHandler lista todas as personas.
func (h *PersonasHandler) ListPersonasHandler(w http.ResponseWriter, r *http.Request) {
	list := h.manager.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  list,
		"total": len(list),
	})
}

// GetPersonaHandler obtém os detalhes de uma persona pelo ID.
func (h *PersonasHandler) GetPersonaHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.manager.Get(id)
	if err != nil {
		if errors.Is(err, personas.ErrPersonaNotFound) {
			writeJSONError(w, http.StatusNotFound, "Persona não encontrada", "not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

// CreatePersonaHandler cria uma nova persona.
func (h *PersonasHandler) CreatePersonaHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // Limite de 1MB anti-DoS
	var input personas.Persona
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido: "+err.Error(), "invalid_request_error")
		return
	}

	created, err := h.manager.Create(input)
	if err != nil {
		if errors.Is(err, personas.ErrPersonaExists) {
			writeJSONError(w, http.StatusConflict, err.Error(), "conflict")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// UpdatePersonaHandler atualiza uma persona existente.
func (h *PersonasHandler) UpdatePersonaHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // Limite de 1MB anti-DoS
	id := chi.URLParam(r, "id")
	var input personas.Persona
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido: "+err.Error(), "invalid_request_error")
		return
	}

	updated, err := h.manager.Update(id, input)
	if err != nil {
		if errors.Is(err, personas.ErrPersonaNotFound) {
			writeJSONError(w, http.StatusNotFound, "Persona não encontrada", "not_found")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

// DeletePersonaHandler remove uma persona.
func (h *PersonasHandler) DeletePersonaHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.manager.Delete(id); err != nil {
		if errors.Is(err, personas.ErrPersonaNotFound) {
			writeJSONError(w, http.StatusNotFound, "Persona não encontrada", "not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Persona excluída com sucesso",
	})
}

// VoiceOption representa uma voz real verificada dos motores Edge TTS ou CapCut TTS.
type VoiceOption struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Language    string   `json:"language"`
	Gender      string   `json:"gender"`
	CountryCode string   `json:"country_code"`
	Flag        string   `json:"flag"`
	Locale      string   `json:"locale"`
	Description string   `json:"description"`
	Provider    string   `json:"provider"`              // "capcut" ou "edge"
	Personality string   `json:"personality,omitempty"` // Personalidade e sentimentos nativos
	Emotions    []string `json:"emotions,omitempty"`    // Emoções recomendadas
	Aliases     []string `json:"aliases,omitempty"`     // Aliases alternativos
}

// Catálogo com vozes verificadas do CapCut Expressivo e Microsoft Edge Neural
var voiceCatalog = []VoiceOption{
	// 🎭 CapCut TTS Expressivo - Português (Brasil) com Sentimentos Nativos
	{
		ID:          "DiT_pt_male_wenrou",
		Name:        "Wenrou (Carinhoso & Empático)",
		Language:    "Português (Brasil)",
		Gender:      "Masculino",
		CountryCode: "BR",
		Flag:        "🇧🇷",
		Locale:      "pt-BR",
		Description: "Voz suave, afetuosa e reconfortante com expressividade emocional nativa.",
		Provider:    "capcut",
		Personality: "Voz acolhedora e calorosa para atendimento humanizado e suporte empático.",
		Emotions:    []string{"calm", "happy", "neutral"},
		Aliases:     []string{"wenrou"},
	},
	{
		ID:          "DiT_pt_male_shangren",
		Name:        "Empresário Frio (Sério & Corporativo)",
		Language:    "Português (Brasil)",
		Gender:      "Masculino",
		CountryCode: "BR",
		Flag:        "🇧🇷",
		Locale:      "pt-BR",
		Description: "Voz executiva, séria, calculista e direta com presença de comando.",
		Provider:    "capcut",
		Personality: "Tom firme e racional para decisões, comunicados corporativos e finanças.",
		Emotions:    []string{"angry", "calm", "neutral"},
		Aliases:     []string{"empresario", "empresario-frio"},
	},
	{
		ID:          "ICL_br_male_212M_LuizS",
		Name:        "Luiz S / Contra (Locutor Dinâmico)",
		Language:    "Português (Brasil)",
		Gender:      "Masculino",
		CountryCode: "BR",
		Flag:        "🇧🇷",
		Locale:      "pt-BR",
		Description: "Voz forte, vibrante e dinâmica em estilo locução e comercial.",
		Provider:    "capcut",
		Personality: "Voz marcante e envolvente para trailers, podcasts e publicidade.",
		Emotions:    []string{"happy", "surprise", "neutral"},
		Aliases:     []string{"luiz", "contra"},
	},
	{
		ID:          "DiT_pt_female_youya",
		Name:        "Youya (Elegante & Serena)",
		Language:    "Português (Brasil)",
		Gender:      "Feminino",
		CountryCode: "BR",
		Flag:        "🇧🇷",
		Locale:      "pt-BR",
		Description: "Voz feminina refinada, aristocrática, serena e delicada.",
		Provider:    "capcut",
		Personality: "Voz aristocrática para concierges, spas e atendimento de luxo.",
		Emotions:    []string{"calm", "happy", "neutral"},
		Aliases:     []string{"youya"},
	},

	// 🎭 CapCut TTS Expressivo - Internacional (Alta Carga Emocional)
	{
		ID:          "en_female_emotional_moon_bigtts",
		Name:        "Emotional (Sentimento Intenso)",
		Language:    "Inglês (EUA)",
		Gender:      "Feminino",
		CountryCode: "US",
		Flag:        "🇺🇸",
		Locale:      "en-US",
		Description: "Voz feminina de alta expressividade emocional e dramática.",
		Provider:    "capcut",
		Personality: "Carga dramática intensa para narrativas e audiolivros.",
		Emotions:    []string{"sad", "happy", "fear"},
		Aliases:     []string{"emotional"},
	},
	{
		ID:          "en_male_dramaqueen_zachk",
		Name:        "Dramaaa (Teatral & Expressivo)",
		Language:    "Inglês (EUA)",
		Gender:      "Masculino",
		CountryCode: "US",
		Flag:        "🇺🇸",
		Locale:      "en-US",
		Description: "Voz teatral, hiperbólica e bem-humorada.",
		Provider:    "capcut",
		Personality: "Expressividade teatral para entretenimento.",
		Emotions:    []string{"surprise", "happy", "angry"},
		Aliases:     []string{"dramaaa", "drama"},
	},

	// 🌐 Microsoft Edge TTS - 🇧🇷 Português (Brasil)
	{ID: "pt-BR-ThalitaMultilingualNeural", Name: "Thalita (Multilíngue)", Language: "Português (Brasil)", Gender: "Feminino", CountryCode: "BR", Flag: "🇧🇷", Locale: "pt-BR", Description: "Voz neural fluida e natural, fala português e outros idiomas.", Provider: "edge"},
	{ID: "pt-BR-AntonioNeural", Name: "Antônio", Language: "Português (Brasil)", Gender: "Masculino", CountryCode: "BR", Flag: "🇧🇷", Locale: "pt-BR", Description: "Voz masculina madura, perfeita para suporte, vendas e notícias.", Provider: "edge"},
	{ID: "pt-BR-FranciscaNeural", Name: "Francisca", Language: "Português (Brasil)", Gender: "Feminino", CountryCode: "BR", Flag: "🇧🇷", Locale: "pt-BR", Description: "Voz clássica e calma, ideal para audiolivros e narrações.", Provider: "edge"},

	// 🇵🇹 Português (Portugal)
	{ID: "pt-PT-DuarteNeural", Name: "Duarte", Language: "Português (Portugal)", Gender: "Masculino", CountryCode: "PT", Flag: "🇵🇹", Locale: "pt-PT", Description: "Voz masculina em português europeu.", Provider: "edge"},
	{ID: "pt-PT-RaquelNeural", Name: "Raquel", Language: "Português (Portugal)", Gender: "Feminino", CountryCode: "PT", Flag: "🇵🇹", Locale: "pt-PT", Description: "Voz feminina em português europeu.", Provider: "edge"},

	// 🇺🇸 Inglês (Estados Unidos)
	{ID: "en-US-AndrewMultilingualNeural", Name: "Andrew (Multilíngue)", Language: "Inglês (EUA)", Gender: "Masculino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz masculina natural e internacional.", Provider: "edge"},
	{ID: "en-US-EmmaMultilingualNeural", Name: "Emma (Multilíngue)", Language: "Inglês (EUA)", Gender: "Feminino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz feminina versátil para produtos globais.", Provider: "edge"},
	{ID: "en-US-BrianMultilingualNeural", Name: "Brian (Multilíngue)", Language: "Inglês (EUA)", Gender: "Masculino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz corporativa de alta definição.", Provider: "edge"},
	{ID: "en-US-AvaMultilingualNeural", Name: "Ava (Multilíngue)", Language: "Inglês (EUA)", Gender: "Feminino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz expressiva para assistentes de IA.", Provider: "edge"},
	{ID: "en-US-GuyNeural", Name: "Guy", Language: "Inglês (EUA)", Gender: "Masculino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz masculina clássica americana."},
	{ID: "en-US-JennyNeural", Name: "Jenny", Language: "Inglês (EUA)", Gender: "Feminino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz feminina padrão para assistentes."},
	{ID: "en-US-AriaNeural", Name: "Aria", Language: "Inglês (EUA)", Gender: "Feminino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz feminina jovem e articulada."},
	{ID: "en-US-ChristopherNeural", Name: "Christopher", Language: "Inglês (EUA)", Gender: "Masculino", CountryCode: "US", Flag: "🇺🇸", Locale: "en-US", Description: "Voz masculina profissional."},

	// 🇬🇧 Inglês (Reino Unido)
	{ID: "en-GB-SoniaNeural", Name: "Sonia", Language: "Inglês (Reino Unido)", Gender: "Feminino", CountryCode: "GB", Flag: "🇬🇧", Locale: "en-GB", Description: "Voz feminina britânica elegante."},
	{ID: "en-GB-RyanNeural", Name: "Ryan", Language: "Inglês (Reino Unido)", Gender: "Masculino", CountryCode: "GB", Flag: "🇬🇧", Locale: "en-GB", Description: "Voz masculina britânica clara."},
	{ID: "en-GB-MaisieNeural", Name: "Maisie", Language: "Inglês (Reino Unido)", Gender: "Feminino", CountryCode: "GB", Flag: "🇬🇧", Locale: "en-GB", Description: "Voz jovem britânica expressiva."},

	// 🇪🇸 Espanhol (Espanha)
	{ID: "es-ES-AlvaroNeural", Name: "Álvaro", Language: "Espanhol (Espanha)", Gender: "Masculino", CountryCode: "ES", Flag: "🇪🇸", Locale: "es-ES", Description: "Voz masculina em espanhol europeu."},
	{ID: "es-ES-ElviraNeural", Name: "Elvira", Language: "Espanhol (Espanha)", Gender: "Feminino", CountryCode: "ES", Flag: "🇪🇸", Locale: "es-ES", Description: "Voz feminina em espanhol europeu."},
	{ID: "es-ES-XimenaNeural", Name: "Ximena", Language: "Espanhol (Espanha)", Gender: "Feminino", CountryCode: "ES", Flag: "🇪🇸", Locale: "es-ES", Description: "Voz conversacional espanhola."},

	// 🇲🇽 Espanhol (México)
	{ID: "es-MX-DaliaNeural", Name: "Dalia", Language: "Espanhol (México)", Gender: "Feminino", CountryCode: "MX", Flag: "🇲🇽", Locale: "es-MX", Description: "Voz feminina em espanhol latino."},
	{ID: "es-MX-JorgeNeural", Name: "Jorge", Language: "Espanhol (México)", Gender: "Masculino", CountryCode: "MX", Flag: "🇲🇽", Locale: "es-MX", Description: "Voz masculina em espanhol latino."},

	// 🇫🇷 Francês (França)
	{ID: "fr-FR-DeniseNeural", Name: "Denise", Language: "Francês (França)", Gender: "Feminino", CountryCode: "FR", Flag: "🇫🇷", Locale: "fr-FR", Description: "Voz feminina em francês clássico."},
	{ID: "fr-FR-HenriNeural", Name: "Henri", Language: "Francês (França)", Gender: "Masculino", CountryCode: "FR", Flag: "🇫🇷", Locale: "fr-FR", Description: "Voz masculina em francês."},
	{ID: "fr-FR-VivienneMultilingualNeural", Name: "Vivienne (Multilíngue)", Language: "Francês (França)", Gender: "Feminino", CountryCode: "FR", Flag: "🇫🇷", Locale: "fr-FR", Description: "Voz feminina francesa multilíngue."},
	{ID: "fr-FR-RemyMultilingualNeural", Name: "Remy (Multilíngue)", Language: "Francês (França)", Gender: "Masculino", CountryCode: "FR", Flag: "🇫🇷", Locale: "fr-FR", Description: "Voz masculina francesa multilíngue."},

	// 🇮🇹 Italiano (Itália)
	{ID: "it-IT-DiegoNeural", Name: "Diego", Language: "Italiano (Itália)", Gender: "Masculino", CountryCode: "IT", Flag: "🇮🇹", Locale: "it-IT", Description: "Voz masculina em italiano."},
	{ID: "it-IT-ElsaNeural", Name: "Elsa", Language: "Italiano (Itália)", Gender: "Feminino", CountryCode: "IT", Flag: "🇮🇹", Locale: "it-IT", Description: "Voz feminina em italiano."},
	{ID: "it-IT-GiuseppeMultilingualNeural", Name: "Giuseppe (Multilíngue)", Language: "Italiano (Itália)", Gender: "Masculino", CountryCode: "IT", Flag: "🇮🇹", Locale: "it-IT", Description: "Voz masculina italiana multilíngue."},

	// 🇩🇪 Alemão (Alemanha)
	{ID: "de-DE-KatjaNeural", Name: "Katja", Language: "Alemão (Alemanha)", Gender: "Feminino", CountryCode: "DE", Flag: "🇩🇪", Locale: "de-DE", Description: "Voz feminina em alemão."},
	{ID: "de-DE-KillianNeural", Name: "Killian", Language: "Alemão (Alemanha)", Gender: "Masculino", CountryCode: "DE", Flag: "🇩🇪", Locale: "de-DE", Description: "Voz masculina em alemão."},
	{ID: "de-DE-SeraphinaMultilingualNeural", Name: "Seraphina (Multilíngue)", Language: "Alemão (Alemanha)", Gender: "Feminino", CountryCode: "DE", Flag: "🇩🇪", Locale: "de-DE", Description: "Voz feminina alemã multilíngue."},
	{ID: "de-DE-FlorianMultilingualNeural", Name: "Florian (Multilíngue)", Language: "Alemão (Alemanha)", Gender: "Masculino", CountryCode: "DE", Flag: "🇩🇪", Locale: "de-DE", Description: "Voz masculina alemã multilíngue."},

	// 🇯🇵 Japonês (Japão)
	{ID: "ja-JP-NanamiNeural", Name: "Nanami", Language: "Japonês (Japão)", Gender: "Feminino", CountryCode: "JP", Flag: "🇯🇵", Locale: "ja-JP", Description: "Voz feminina em japonês."},
	{ID: "ja-JP-KeitaNeural", Name: "Keita", Language: "Japonês (Japão)", Gender: "Masculino", CountryCode: "JP", Flag: "🇯🇵", Locale: "ja-JP", Description: "Voz masculina em japonês."},
}

// ListVoicesHandler retorna o catálogo de vozes 100% verificadas.
func ListVoicesHandler(w http.ResponseWriter, r *http.Request) {
	for i := range voiceCatalog {
		if voiceCatalog[i].Provider == "" {
			voiceCatalog[i].Provider = "edge"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  voiceCatalog,
		"total": len(voiceCatalog),
	})
}
