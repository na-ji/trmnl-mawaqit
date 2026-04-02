package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
)

type Handlers struct {
	store        *Store
	mawaqit      *MawaqitClient
	tmpl         *template.Template
	clientID     string
	clientSecret string
}

// Install GET /install
// TRMNL redirects users here with code + installation_callback_url
func (h *Handlers) Install(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	callbackURL := r.URL.Query().Get("installation_callback_url")

	if code == "" || callbackURL == "" {
		http.Error(w, "missing code or installation_callback_url", http.StatusBadRequest)
		return
	}

	// Exchange code for access token
	body := fmt.Sprintf(
		`{"code":%q,"client_id":%q,"client_secret":%q,"grant_type":"authorization_code"}`,
		code, h.clientID, h.clientSecret,
	)

	resp, err := http.Post(
		"https://usetrmnl.com/oauth/token",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		log.Printf("oauth token exchange failed: %v", err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("oauth token exchange returned %d: %s", resp.StatusCode, string(respBody))
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	// We don't need to store the token here — it arrives again in the success webhook.
	http.Redirect(w, r, callbackURL, http.StatusFound)
}

// InstallCallback POST /install/callback
// TRMNL sends this webhook after successful installation
func (h *Handlers) InstallCallback(w http.ResponseWriter, r *http.Request) {
	accessToken := extractBearerToken(r)
	if accessToken == "" {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}

	var payload struct {
		User struct {
			UUID         string `json:"uuid"`
			Name         string `json:"name"`
			TimeZoneIANA string `json:"time_zone_iana"`
		} `json:"user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if payload.User.UUID == "" {
		http.Error(w, "missing user uuid", http.StatusBadRequest)
		return
	}

	user := User{
		UUID:        payload.User.UUID,
		AccessToken: accessToken,
		MosqueSlug:  "",
		Timezone:    payload.User.TimeZoneIANA,
	}

	if err := h.store.SaveUser(user); err != nil {
		log.Printf("save user failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("user installed: uuid=%s timezone=%s", user.UUID, user.Timezone)
	w.WriteHeader(http.StatusOK)
}

// Manage GET /manage
// TRMNL opens this in an iframe/webview for the user to configure settings
func (h *Handlers) Manage(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		http.Error(w, "missing uuid", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUser(uuid)
	if err != nil {
		log.Printf("get user failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := struct {
		UUID       string
		MosqueSlug string
		Message    string
	}{
		UUID:       uuid,
		MosqueSlug: "",
	}
	if user != nil {
		data.MosqueSlug = user.MosqueSlug
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "manage.html", data); err != nil {
		log.Printf("render manage template: %v", err)
	}
}

// ManageSave POST /manage
// Form submission from the manage page
func (h *Handlers) ManageSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	uuid := r.FormValue("uuid")
	mosqueSlug := strings.TrimSpace(r.FormValue("mosque_slug"))

	if uuid == "" {
		http.Error(w, "missing uuid", http.StatusBadRequest)
		return
	}
	if mosqueSlug == "" {
		http.Error(w, "mosque slug is required", http.StatusBadRequest)
		return
	}

	// Validate the slug by fetching from Mawaqit API
	_, err := h.mawaqit.GetMosqueData(mosqueSlug)
	if err != nil {
		log.Printf("mawaqit validation failed for slug %q: %v", mosqueSlug, err)
	}

	if err := h.store.UpdateMosqueSlug(uuid, mosqueSlug); err != nil {
		log.Printf("update mosque slug failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := struct {
		UUID       string
		MosqueSlug string
		Message    string
	}{
		UUID:       uuid,
		MosqueSlug: mosqueSlug,
		Message:    "Settings saved successfully!",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "manage.html", data); err != nil {
		log.Printf("render manage template: %v", err)
	}
}

// Markup POST /markup
// TRMNL calls this to get rendered HTML for the device
func (h *Handlers) Markup(w http.ResponseWriter, r *http.Request) {
	accessToken := extractBearerToken(r)
	if accessToken == "" {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	userUUID := r.FormValue("user_uuid")
	if userUUID == "" {
		http.Error(w, "missing user_uuid", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUser(userUUID)
	if err != nil {
		log.Printf("get user failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	if user.MosqueSlug == "" {
		msg := `<div style="display:flex;align-items:center;justify-content:center;width:800px;height:480px;font-family:sans-serif;font-size:32px;text-align:center;padding:40px;box-sizing:border-box;">Please configure your mosque in the plugin settings.</div>`
		writeJSON(w, MarkupResult{
			Markup:          msg,
			MarkupHalfHoriz: msg,
			MarkupHalfVert:  msg,
			MarkupQuadrant:  msg,
		})
		return
	}

	data, err := h.mawaqit.GetMosqueData(user.MosqueSlug)
	if err != nil {
		log.Printf("fetch mawaqit data for %q: %v", user.MosqueSlug, err)
		http.Error(w, "failed to fetch prayer times", http.StatusBadGateway)
		return
	}

	tz := user.Timezone
	if tz == "" {
		tz = "UTC"
	}

	pd, err := buildPrayerDisplay(data, tz)
	if err != nil {
		log.Printf("build prayer display: %v", err)
		http.Error(w, "failed to compute prayer times", http.StatusInternalServerError)
		return
	}

	result, err := renderAllMarkup(h.tmpl, pd)
	if err != nil {
		log.Printf("render markup: %v", err)
		http.Error(w, "failed to render markup", http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// Uninstall POST /uninstall
// TRMNL sends this when the user uninstalls the plugin
func (h *Handlers) Uninstall(w http.ResponseWriter, r *http.Request) {
	accessToken := extractBearerToken(r)
	if accessToken == "" {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}

	var payload struct {
		UserUUID string `json:"user_uuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if payload.UserUUID == "" {
		http.Error(w, "missing user_uuid", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteUser(payload.UserUUID); err != nil {
		log.Printf("delete user failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("user uninstalled: uuid=%s", payload.UserUUID)
	w.WriteHeader(http.StatusOK)
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json response: %v", err)
	}
}
