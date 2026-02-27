# Threat Model — OpenFlare Edge Proxy POC

## Scope

Ce modèle de menace couvre le périmètre du POC déployé localement via Docker Compose.
Il ne couvre PAS les menaces réseau/infrastructure (L3/L4).

## Assets à protéger

| Asset | Sensibilité | Description |
|-------|-------------|-------------|
| Réponses cachées | Haute | Ne doivent pas fuiter de données privées |
| Origin (testsite) | Moyenne | Ne doit pas être surchargé |
| Admin API | Haute | Accès restreint, opérations destructives |
| Configuration | Moyenne | Contient tokens, paramètres sécurité |
| Métriques | Basse | Informations de performance |

## Menaces et mitigations

### T1 — Cache Data Leak

**Description** : Données privées (cookie-dependent, auth-dependent) stockées en cache et servies à un autre utilisateur.

**Impact** : Critique

**Mitigation** :
- BYPASS par défaut si `Cookie` ou `Authorization` présent
- Ne pas stocker les réponses avec `Set-Cookie`, `Cache-Control: private/no-store`
- Ne pas stocker les réponses non-2xx
- Tests E2E dédiés

### T2 — Cache Poisoning

**Description** : Un attaquant forge une requête qui stocke une réponse malveillante en cache.

**Impact** : Élevé

**Mitigation** :
- Cache key stricte incluant host, path normalisé, query normalisée
- Pas de cache sur méthodes non-safe
- WAF pré-cache filtre les requêtes suspectes
- Normalisation du path avant cache key

### T3 — Path Traversal

**Description** : Accès à des fichiers ou routes non prévus via `../` ou variantes encodées.

**Impact** : Élevé

**Mitigation** :
- Normalisation du path (résolution dots)
- Règle WAF `block-path-traversal`
- Tests E2E avec variantes encodées

### T4 — SQL Injection / XSS (basique)

**Description** : Injection de SQL ou scripts via query params ou body.

**Impact** : Moyen (dépend de l'origin)

**Mitigation** :
- Règles WAF pour patterns courants
- Mode shadow pour test, enforce pour production
- **Limité** : patterns simples uniquement, pas OWASP CRS

### T5 — ReDoS (Regular Expression Denial of Service)

**Description** : Regex WAF crafted pour causer un backtracking exponentiel.

**Impact** : Élevé (DoS du proxy)

**Mitigation** :
- Moteur RE2 de Go (pas de backtracking)
- Compilation des regex au démarrage
- Pas de regex dynamiques issues du trafic

### T6 — Origin Overload

**Description** : Trop de requêtes atteignent l'origin (cache miss, cache bypass).

**Impact** : Moyen

**Mitigation** :
- Rate limiting par IP/path
- Singleflight sur cache miss
- Timeouts stricts vers origin

### T7 — Admin API Abuse

**Description** : Accès non autorisé à l'admin API (purge, stats, config).

**Impact** : Élevé

**Mitigation** :
- Bind sur port séparé (:8081)
- Authentification par token (`Authorization: Bearer <token>`)
- Réseau Docker interne (non exposé par défaut)

### T8 — Memory Exhaustion

**Description** : Cache non borné ou bodies trop gros causent un OOM.

**Impact** : Élevé

**Mitigation** :
- Limites `max_entries` et `max_bytes` sur le cache
- Limite `max_request_body_bytes` sur les requêtes
- Limite `max_body_inspect_bytes` sur l'inspection WAF
- Éviction LRU quand limites atteintes

### T9 — Request Smuggling

**Description** : Exploitation de différences d'interprétation HTTP entre proxy et origin.

**Impact** : Élevé

**Mitigation** :
- Utilisation de `httputil.ReverseProxy` Go (implémentation standard)
- Limites de taille headers
- Pas de pipeline HTTP/1.0
- **Limitation** : pas de tests exhaustifs de smuggling en v1

### T10 — Header Injection

**Description** : Injection de headers malveillants via `X-Forwarded-*` spoofé.

**Impact** : Moyen

**Mitigation** :
- `trust_x_forwarded_for: false` par défaut
- Réécriture des headers forwarded par le proxy
- Pas de propagation aveugle des headers client

## Matrice de risque

```
Impact ↑
  Critique │ T1
  Élevé    │ T2 T3 T5 T7 T8 T9
  Moyen    │ T4 T6 T10
  Bas      │
           └─────────────────────→ Probabilité
              Basse  Moyenne  Haute
```

## Menaces hors scope

| Menace | Raison |
|--------|--------|
| DDoS L3/L4 volumétrique | Nécessite infrastructure réseau |
| DDoS L7 sophistiqué | Rate limiting basique seulement |
| Bots avancés | Pas de JS challenge |
| Supply chain attack | Hors scope POC |
| Zero-day WAF bypass | Règles explicites seulement |
| TLS attacks | Pas de TLS termination en v1 |
