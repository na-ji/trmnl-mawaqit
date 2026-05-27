package main

import (
	"net/http"
	"strings"
)

// Translations maps string keys to their translated values.
type Translations map[string]string

var translations = map[string]Translations{
	"en": {
		"PageTitle":     "Mawaqit Prayer Times - Settings",
		"Heading":       "Mawaqit Prayer Times",
		"Subtitle":      "Configure your mosque to display prayer times on your TRMNL device.",
		"FormLabel":     "Search for your mosque",
		"Placeholder":   "Type a mosque name or city...",
		"Hint":          "Start typing to search the Mawaqit directory, then pick your mosque from the list.",
		"CurrentLabel":  "Currently selected",
		"NoneSelected":  "No mosque selected yet.",
		"Searching":     "Searching...",
		"NoResults":     "No mosque found.",
		"SearchError":   "Search failed. Please try again.",
		"TooShort":      "Type at least 2 characters.",
		"Button":        "Save Settings",
		"SuccessMsg":    "Settings saved successfully!",
		"BackToTRMNL":   "Back to TRMNL",
	},
	"fr": {
		"PageTitle":     "Mawaqit Horaires de Prière - Paramètres",
		"Heading":       "Mawaqit Horaires de Prière",
		"Subtitle":      "Configurez votre mosquée pour afficher les horaires de prière sur votre appareil TRMNL.",
		"FormLabel":     "Rechercher votre mosquée",
		"Placeholder":   "Tapez un nom de mosquée ou une ville...",
		"Hint":          "Commencez à taper pour rechercher dans l'annuaire Mawaqit, puis choisissez votre mosquée dans la liste.",
		"CurrentLabel":  "Sélection actuelle",
		"NoneSelected":  "Aucune mosquée sélectionnée.",
		"Searching":     "Recherche...",
		"NoResults":     "Aucune mosquée trouvée.",
		"SearchError":   "Échec de la recherche. Veuillez réessayer.",
		"TooShort":      "Tapez au moins 2 caractères.",
		"Button":        "Enregistrer",
		"SuccessMsg":    "Paramètres enregistrés avec succès !",
		"BackToTRMNL":   "Retour à TRMNL",
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
