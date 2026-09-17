package server

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"edgego-voice/internal/config"
)

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

func writeJSONError(w http.ResponseWriter, statusCode int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	var resp errorResponse
	resp.Error.Message = message
	resp.Error.Type = errType
	_ = json.NewEncoder(w).Encode(resp)
}

// SecurityHeadersMiddleware adiciona cabeçalhos defensivos essenciais recomendados pela OWASP.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware valida o cabeçalho Authorization: Bearer <API_KEY> com proteção contra timing-attacks.
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.RequireAPIKey {
				next.ServeHTTP(w, r)
				return
			}

			if cfg.APIKey == "" {
				slog.Error("Servidor configurado para exigir API_KEY, mas nenhuma chave foi definida no ambiente")
				writeJSONError(w, http.StatusInternalServerError, "Servidor não configurado para autenticação. A variável API_KEY não foi definida.", "server_error")
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "Chave de API ausente ou em formato inválido. Use o cabeçalho 'Authorization: Bearer SUA_CHAVE'.", "authentication_error")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.APIKey)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "Chave de API inválida.", "authentication_error")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// StructuredLoggerMiddleware realiza o log estruturado de cada requisição HTTP.
func StructuredLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		slog.Info("HTTP Request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.statusCode,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// RecoveryMiddleware captura qualquer panic inesperado e retorna HTTP 500.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("Panic recuperado no handler HTTP", "error", rec)
				writeJSONError(w, http.StatusInternalServerError, "Erro interno no servidor.", "internal_error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
}

func (rw *responseWriterWrapper) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
