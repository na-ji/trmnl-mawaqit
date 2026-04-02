package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

func main() {
	loadEnvFile(".env")

	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "./data/mawaqit.db")
	mawaqitBase := os.Getenv("MAWAQIT_API_BASE")
	clientID := os.Getenv("TRMNL_CLIENT_ID")
	clientSecret := os.Getenv("TRMNL_CLIENT_SECRET")

	fmt.Println("mawaqitBase: " + mawaqitBase)

	store, err := NewStore(dbPath)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	defer store.Close()

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"eq": func(a, b string) bool { return a == b },
	}).ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	h := &Handlers{
		store:        store,
		mawaqit:      NewMawaqitClient(mawaqitBase),
		tmpl:         tmpl,
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
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
