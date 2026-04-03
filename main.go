package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func initLogger() {
	logFormat := envOr("LOG_FORMAT", "console")
	if logFormat == "json" {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.status).
			Dur("duration", time.Since(start)).
			Msg("request")
	})
}

func main() {
	if runPreview(os.Args) {
		return
	}

	loadEnvFile(".env")
	initLogger()

	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "./data/mawaqit.db")
	mawaqitBase := os.Getenv("MAWAQIT_API_BASE")
	clientID := os.Getenv("TRMNL_CLIENT_ID")
	clientSecret := os.Getenv("TRMNL_CLIENT_SECRET")

	log.Debug().Str("mawaqit_base", mawaqitBase).Msg("config loaded")

	store, err := NewStore(dbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("init store")
	}
	defer store.Close()

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"eq": func(a, b string) bool { return a == b },
	}).ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal().Err(err).Msg("parse templates")
	}

	h := &Handlers{
		store:        store,
		mawaqit:      NewMawaqitClient(mawaqitBase),
		tmpl:         tmpl,
		markupCache:  NewMarkupCache(),
		clientID:     clientID,
		clientSecret: clientSecret,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /install", h.Install)
	mux.HandleFunc("POST /install/callback", h.InstallCallback)
	mux.HandleFunc("GET /manage", h.Manage)
	mux.HandleFunc("POST /manage", h.ManageSave)
	mux.HandleFunc("POST /markup", h.Markup)
	mux.HandleFunc("POST /uninstall", h.Uninstall)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	addr := ":" + port
	log.Info().Str("addr", addr).Msg("starting server")
	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}
