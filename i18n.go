package main

import (
	"net/http"
	"strings"
)

// Translations maps string keys to their translated values.
type Translations map[string]string

var translations = map[string]Translations{
	"en": {
		"PageTitle":    "Mawaqit Prayer Times - Settings",
		"Heading":      "Mawaqit Prayer Times",
		"Subtitle":     "Configure your mosque to display prayer times on your TRMNL device.",
		"FormLabel":    "Mosque Slug",
		"Placeholder":  "e.g. tawba-bussy-saint-georges",
		"HintPrefix":   "Find your mosque slug at mawaqit.net. It's the part after the URL, e.g. mawaqit.net/en/",
		"HintEmphasis": "your-mosque-slug",
		"Button":       "Save Settings",
		"SuccessMsg":   "Settings saved successfully!",
		"BackToTRMNL":  "Back to TRMNL",
	},
	"fr": {
		"PageTitle":    "Mawaqit Horaires de Prière - Paramètres",
		"Heading":      "Mawaqit Horaires de Prière",
		"Subtitle":     "Configurez votre mosquée pour afficher les horaires de prière sur votre appareil TRMNL.",
		"FormLabel":    "Identifiant de la mosquée",
		"Placeholder":  "ex. tawba-bussy-saint-georges",
		"HintPrefix":   "Trouvez l'identifiant de votre mosquée sur mawaqit.net. C'est la partie après l'URL, ex. mawaqit.net/",
		"HintEmphasis": "identifiant-de-votre-mosquee",
		"Button":       "Enregistrer",
		"SuccessMsg":   "Paramètres enregistrés avec succès !",
		"BackToTRMNL":  "Retour à TRMNL",
	},
}

// detectLang returns "fr" or "en" based on:
// 1. Explicit ?lang= query param (highest priority)
// 2. Accept-Language header
// 3. Default "en"
func detectLang(r *http.Request) string {
	if lang := r.URL.Query().Get("lang"); lang == "fr" || lang == "en" {
		return lang
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Language")), "fr") {
		return "fr"
	}
	return "en"
}

// T returns the translations map for the given language.
func T(lang string) Translations {
	if t, ok := translations[lang]; ok {
		return t
	}
	return translations["en"]
}
