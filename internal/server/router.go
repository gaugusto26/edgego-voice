package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"edgego-voice/internal/audiocache"
	"edgego-voice/internal/config"
	"edgego-voice/internal/edgetts"
	"edgego-voice/internal/personas"
	"edgego-voice/internal/tts"
)

// SetupRouter configura as rotas, middlewares HTTP e arquivos estáticos da UI.
func SetupRouter(
	cfg *config.Config,
	ttsClient tts.Synthesizer,
	vm *edgetts.VoiceManager,
	pm *personas.Manager,
	cache *audiocache.Cache,
) http.Handler {
	r := chi.NewRouter()

	r.Use(RecoveryMiddleware)
	r.Use(SecurityHeadersMiddleware)
	r.Use(StructuredLoggerMiddleware)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Persona-ID"},
		ExposedHeaders:   []string{"Link", "Content-Disposition", "X-Persona-ID", "X-Cache", "X-Subtitles-Duration-Ms"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check com Telemetria Real do Runtime Go
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		items, bytesUsed, maxBytes := cache.Stats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "ok",
			"engine":        "edgego-voice",
			"version":       "v1.0",
			"goroutines":    runtime.NumGoroutine(),
			"alloc_mb":      float64(m.Alloc) / (1024 * 1024),
			"sys_mb":        float64(m.Sys) / (1024 * 1024),
			"heap_inuse_mb": float64(m.HeapInuse) / (1024 * 1024),
			"gc_runs":       m.NumGC,
			"cache_items":   items,
			"cache_bytes":   bytesUsed,
			"cache_max":     maxBytes,
		})
	})

	speechHandler := NewSpeechHandler(cfg, ttsClient, vm, pm, cache)
	personasHandler := NewPersonasHandler(pm)

	// API REST de Personas e Vozes
	r.Route("/api", func(r chi.Router) {
		r.Get("/voices", ListVoicesHandler)

		// Protegido por chave de API
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(cfg))
			r.Get("/personas", personasHandler.ListPersonasHandler)
			r.Post("/personas", personasHandler.CreatePersonaHandler)
			r.Get("/personas/{id}", personasHandler.GetPersonaHandler)
			r.Put("/personas/{id}", personasHandler.UpdatePersonaHandler)
			r.Delete("/personas/{id}", personasHandler.DeletePersonaHandler)
		})
	})

	// API v1 (OpenAI & Personas)
	r.Route("/v1", func(r chi.Router) {
		r.Get("/models", ListModelsHandler)
		r.Get("/voices", ListVoicesHandler)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(cfg))
			r.Post("/audio/speech", speechHandler.GenerateSpeechHandler)
			r.Post("/audio/subtitles", speechHandler.GenerateSubtitlesHandler)
			r.Get("/personas", personasHandler.ListPersonasHandler)
			r.Post("/personas", personasHandler.CreatePersonaHandler)
			r.Get("/personas/{id}", personasHandler.GetPersonaHandler)
			r.Put("/personas/{id}", personasHandler.UpdatePersonaHandler)
			r.Delete("/personas/{id}", personasHandler.DeletePersonaHandler)
		})

		// Rota dedicada de Personas (valida autenticação internamente)
		r.Post("/persona/{id}/speech", speechHandler.GeneratePersonaSpeechHandler)
		r.Post("/persona/{id}/subtitles", speechHandler.GeneratePersonaSubtitlesHandler)

		// Streaming WebSocket bidirecional para LLMs (valida autenticação internamente)
		r.Get("/audio/stream", speechHandler.StreamSpeechWebSocket)
	})

	// Servir o Frontend Web compilado do Next.js
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}
	webDir := filepath.Join(workDir, "web")
	fileServer(r, "/", http.Dir(webDir))

	return r
}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	})
}
