[[Read in English](README.md)]

# TRMNL Mawaqit - Plugin Horaires de Prière

Un plugin [TRMNL](https://usetrmnl.com) qui affiche les horaires de prière islamiques sur votre appareil e-ink, à partir des données de [Mawaqit](https://mawaqit.net).

La seule configuration nécessaire est l'identifiant de votre mosquée (ex. `tawba-bussy-saint-georges`). Le plugin affiche les horaires de prière du jour avec la prochaine prière mise en évidence en gras.

## Fonctionnalités

- Horaires de prière en temps réel depuis les données Mawaqit
- Prochaine prière automatiquement mise en évidence en gras
- Horaires de Jumua (prière du vendredi) affichés en pied de page
- 4 variantes de mise en page TRMNL : plein écran, demi horizontal, demi vertical, quadrant
- Page de paramètres avec support i18n (anglais et français)
- Cache intelligent à deux niveaux : horaires de prière en cache jusqu'à Isha, rendu en cache jusqu'à la prochaine prière
- Journalisation structurée avec zerolog (sortie JSON ou console)

## Premiers pas

1. [Installer le plugin sur votre TRMNL](https://trmnl.com/plugin_settings/new?keyname=mawaqit)
2. Trouvez l'identifiant de votre mosquée sur [mawaqit.net](https://mawaqit.net) — c'est la dernière partie de l'URL : `mawaqit.net/fr/<identifiant-de-votre-mosquee>`
3. Entrez l'identifiant dans la page de paramètres du plugin

## Auto-hébergement

Si vous souhaitez héberger votre propre instance du serveur au lieu d'utiliser celle hébergée :

### Lancer avec Docker Compose

```bash
cp .env.example .env
# Modifiez .env avec vos identifiants TRMNL

docker compose up -d
```

Cela télécharge l'image pré-construite depuis `ghcr.io/na-ji/trmnl-mawaqit:main` et démarre le serveur avec l'API Mawaqit.

### Variables d'environnement

| Variable              | Requis | Défaut              | Description                                                                        |
|-----------------------|--------|---------------------|------------------------------------------------------------------------------------|
| `TRMNL_CLIENT_ID`     | Oui    | -                   | Client ID OAuth de l'enregistrement du plugin TRMNL                                |
| `TRMNL_CLIENT_SECRET` | Oui    | -                   | Secret client OAuth                                                                |
| `MAWAQIT_API_BASE`    | Oui    | -                   | URL de base de l'API Mawaqit non-officielle (cf https://github.com/mrsofiane/mawaqit-api) |
| `PORT`                | Non    | `8080`              | Port d'écoute HTTP                                                                 |
| `DB_PATH`             | Non    | `./data/mawaqit.db` | Chemin du fichier de base de données SQLite                                        |
| `LOG_FORMAT`          | Non    | `console`           | Format de sortie des logs : `console` pour le dev, `json` pour la production       |

### Enregistrement du plugin TRMNL

Lors de l'enregistrement de votre propre plugin sur le portail développeur TRMNL, configurez ces URLs (remplacez `BASE_URL` par l'URL publique de votre serveur) :

| Paramètre                        | URL                           |
|----------------------------------|-------------------------------|
| URL d'installation               | `{BASE_URL}/install`          |
| URL de callback d'installation   | `{BASE_URL}/install/callback` |
| URL de rendu du plugin           | `{BASE_URL}/markup`           |
| URL de gestion du plugin         | `{BASE_URL}/manage`           |
| URL de désinstallation           | `{BASE_URL}/uninstall`        |

## Développement

### Prérequis

- Go 1.25+ (utilise le routage method-pattern)
- Un compte développeur TRMNL avec un plugin enregistré (fournit `TRMNL_CLIENT_ID` et `TRMNL_CLIENT_SECRET`)

### Lancer en local

```bash
cp .env.example .env
# Modifiez .env avec vos identifiants TRMNL

# Définir les variables d'environnement
export TRMNL_CLIENT_ID=votre_client_id
export TRMNL_CLIENT_SECRET=votre_client_secret
# Lien vers l'API Mawaqit non-officielle https://github.com/mrsofiane/mawaqit-api
export MAWAQIT_API_BASE=https://mawaqit.naj.ovh/api/v1

go run .
```

Le serveur démarre sur le port 8080 par défaut. Visitez `http://localhost:8080/health` pour vérifier.

### Prévisualiser les templates

Affichez les 4 variantes de mise en page dans votre navigateur sans lancer le serveur :

```bash
go run . preview --slug=identifiant-de-votre-mosquee --timezone=Europe/Paris
```

Cela génère un fichier `preview.html` utilisant le CSS du framework TRMNL et l'ouvre dans votre navigateur par défaut.

### Structure du projet

```
trmnl-mawaqit/
├── main.go           # Point d'entrée, routeur, config, middleware de journalisation
├── handlers.go       # Handlers HTTP pour tous les endpoints TRMNL
├── mawaqit.go        # Client API Mawaqit avec cache TTL basé sur Isha
├── markup.go         # Calcul des horaires, rendu des templates, cache du rendu
├── i18n.go           # Traductions (EN/FR) et détection de la langue
├── preview.go        # Commande CLI de prévisualisation des templates
├── store.go          # Stockage SQLite des utilisateurs (CRUD)
├── cmd/healthcheck/  # Petit binaire de vérification de santé pour Docker HEALTHCHECK
├── templates/
│   ├── full.html              # Mise en page plein écran (800x480)
│   ├── half_horizontal.html   # Demi horizontal (800x240)
│   ├── half_vertical.html     # Demi vertical (400x480)
│   ├── quadrant.html          # Quadrant (400x240)
│   └── manage.html            # Formulaire de paramètres (i18n)
├── Dockerfile                 # Build multi-étapes avec image distroless
├── docker-compose.yml
└── .github/workflows/
    └── docker.yml             # CI : build et push vers GHCR
```

Tout le code Go est dans `package main` — pas besoin de sous-packages à cette échelle.

### Cycle de vie du plugin

Le plugin implémente le cycle de vie standard des plugins TRMNL :

```
L'utilisateur clique sur « Installer » sur TRMNL
        │
        ▼
  GET /install ──► Échange du code OAuth contre un token ──► Redirection vers TRMNL
        │
        ▼
  POST /install/callback ──► Stockage de l'utilisateur (UUID, token, fuseau horaire)
        │
        ▼
  GET /manage ──► Affichage du formulaire de configuration
  POST /manage ──► Sauvegarde de l'identifiant de la mosquée
        │
        ▼
  POST /markup ──► Vérification du cache ──► Récupération des données Mawaqit
                   ──► Calcul des horaires du jour ──► Détermination de la prochaine prière
                   ──► Rendu des 4 mises en page ──► Mise en cache jusqu'à la prochaine prière
                   ──► Retour JSON avec toutes les variantes
        │
        ▼
  POST /uninstall ──► Suppression de l'utilisateur
```

### Composants principaux

**`store.go`** -- Stockage SQLite via `modernc.org/sqlite` (Go pur, sans CGO). Stocke l'UUID, le token d'accès, l'identifiant de la mosquée et le fuseau horaire IANA. Mode WAL activé pour les lectures concurrentes.

**`mawaqit.go`** -- Client HTTP pour l'API Mawaqit. Récupère les données d'une mosquée par identifiant et met en cache les réponses jusqu'à l'heure d'Isha dans le fuseau horaire de la mosquée. Après la dernière prière du jour, le cache expire pour récupérer les données du lendemain.

**`markup.go`** -- Prend les données de l'API Mawaqit et le fuseau horaire de l'utilisateur, extrait les horaires de prière du jour, détermine la prochaine prière en comparant chaque horaire avec l'heure locale, et produit les 4 variantes de template. Si toutes les prières du jour sont passées, Fajr est marqué comme prochaine. Inclut un cache de rendu par utilisateur qui expire à la prochaine prière.

**`handlers.go`** -- Handlers HTTP implémentant le contrat plugin TRMNL. L'endpoint markup retourne un objet JSON avec 4 clés (`markup`, `markup_half_horizontal`, `markup_half_vertical`, `markup_quadrant`), chacune contenant le HTML rendu pour la taille d'affichage TRMNL correspondante.

**`i18n.go`** -- Traductions simples basées sur des maps pour l'anglais et le français. La langue est auto-détectée depuis l'en-tête `Accept-Language` du navigateur, avec possibilité de sélection manuelle via paramètre de requête.
