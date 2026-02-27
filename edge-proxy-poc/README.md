# OpenFlare Edge Proxy POC

**Reverse proxy HTTP L7** avec cache in-memory, WAF basique et rate limiting.  
POC mono-instance, déployé via Docker Compose, testé E2E avec pytest.

> **⚠️ LIMITATIONS** — Ce POC est un exercice d'architecture. Il n'est PAS conçu pour la production.  
> Voir [Limitations](#limitations) et [docs/08-limitations-roadmap.md](docs/08-limitations-roadmap.md).

---

## Architecture rapide

```
Client → [Proxy Go :8080] → [TestSite Python :8000]
              ↓
         [Admin :8081]
         /metrics, /healthz, /readyz
         /admin/cache/*
```

**Pipeline de requête :**

```
RequestID → Logging → Recovery → WAF → RateLimit → Cache → Upstream → Response
```

---

## Démarrage rapide

### Prérequis

- Docker & Docker Compose v2
- Make (optionnel)

### Lancer

```bash
# Démarrer proxy + testsite
docker compose up --build -d

# Vérifier la santé
curl http://localhost:8080/healthz
curl http://localhost:8081/metrics
```

### Avec Make

```bash
make up       # docker compose up --build -d
make health   # curl healthz
make metrics  # curl metrics
make down     # docker compose down -v
```

### Exécuter les tests E2E

```bash
# Via Docker Compose (profil test)
docker compose --profile test run --rm e2e

# Via Make
make test-e2e
```

### Smoke test rapide

```bash
make smoke
# ou
./scripts/smoke.sh
```

---

## Configuration

Tous les fichiers de configuration sont dans `configs/` :

| Fichier | Description |
|---------|-------------|
| `proxy.dev.yaml` | Config principale (serveur, upstream, cache, sécurité) |
| `proxy.test.yaml` | Config pour environnement de test E2E |
| `rules.waf.yaml` | Règles WAF (shadow/enforce) |
| `cache-rules.yaml` | Règles de cache par path/regex |
| `rate-limit.yaml` | Règles de rate limiting |

### Variables d'environnement

```bash
ADMIN_TOKEN=change-me       # Token d'auth admin API
CONFIG_PATH=/etc/openflare/proxy.dev.yaml
WAF_RULES_PATH=/etc/openflare/rules.waf.yaml
CACHE_RULES_PATH=/etc/openflare/cache-rules.yaml
RATE_LIMIT_PATH=/etc/openflare/rate-limit.yaml
```

---

## Endpoints

### Proxy (port 8080)

Tous les endpoints du testsite sont accessibles via le proxy :

| Endpoint | Description |
|----------|-------------|
| `GET /` | Page HTML cacheable |
| `GET /static/*` | Assets statiques (JS, CSS, PNG) |
| `GET /api/time` | JSON timestamp (non cacheable) |
| `GET /api/random` | JSON aléatoire |
| `GET /api/echo` | Echo headers/query |
| `GET /cache/public-short` | Cache-Control: public, max-age=5 |
| `GET /cache/public-long` | Cache-Control: public, max-age=60 |
| `GET /cache/no-store` | Cache-Control: no-store |
| `GET /cache/private` | Cache-Control: private |
| `GET /cache/with-set-cookie` | Réponse avec Set-Cookie |
| `GET /cache/etag` | Support ETag / If-None-Match |
| `GET /cache/last-modified` | Support Last-Modified |
| `GET /search?q=...` | Echo query (WAF test) |
| `POST /submit` | Echo body (WAF test) |
| `GET /path-test/{value}` | Test normalisation path |
| `POST /auth/login` | Set session cookie |
| `GET /auth/profile` | Profil (requiert cookie) |
| `GET /private` | Requiert Authorization/cookie |
| `GET /slow?delay_ms=N` | Réponse lente |
| `GET /error/{code}` | Retourne code HTTP spécifique |
| `GET /debug/headers` | Echo headers reçus |
| `GET /healthz` | Health check |
| `GET /readyz` | Readiness check |

### Admin API (port 8081)

| Endpoint | Description |
|----------|-------------|
| `GET /admin/cache/stats` | Statistiques cache |
| `POST /admin/cache/purge` | Purge clé exacte |
| `POST /admin/cache/purge-prefix` | Purge par préfixe |
| `POST /admin/cache/purge-all` | Purge totale |
| `GET /metrics` | Métriques Prometheus |

Toutes les routes `/admin/*` nécessitent `Authorization: Bearer <token>`.

---

## Headers de debug

Quand `cache.debug_headers: true` :

| Header | Valeurs possibles |
|--------|-------------------|
| `X-Edge-Cache` | `HIT`, `MISS`, `BYPASS`, `STORE`, `ERROR` |
| `X-Request-ID` | UUID v4 unique par requête |
| `X-Edge-Cache-TTL` | TTL restant (secondes) |
| `X-Edge-Cache-Age` | Âge de l'entrée cache |

---

## Tests

### Tests unitaires Go

```bash
cd proxy && go test ./... -v
```

### Tests E2E Python

```bash
# Via Compose
docker compose --profile test run --rm e2e

# Tests spécifiques
docker compose --profile test run --rm e2e pytest -v -m cache
docker compose --profile test run --rm e2e pytest -v -m waf
docker compose --profile test run --rm e2e pytest -v -m ratelimit
docker compose --profile test run --rm e2e pytest -v -k purge
```

---

## Purge cache

```bash
# Purge une URL
curl -X POST http://localhost:8081/admin/cache/purge \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"key": "/cache/public-long"}'

# Purge par préfixe
curl -X POST http://localhost:8081/admin/cache/purge-prefix \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"prefix": "/static/"}'

# Purge totale
curl -X POST http://localhost:8081/admin/cache/purge-all \
  -H "Authorization: Bearer change-me"
```

---

## Troubleshooting

### Le proxy ne démarre pas

```bash
docker compose logs proxy
```
Vérifier que les fichiers de config sont valides YAML.

### Cache toujours MISS

- Vérifier les headers `Cache-Control` de l'origin
- Les requêtes avec `Cookie` ou `Authorization` sont BYPASS par défaut
- Vérifier `configs/cache-rules.yaml` pour les exclusions par path

### WAF bloque des requêtes légitimes

- Mode actuel : vérifier `mode` dans `configs/rules.waf.yaml`
- Passer en `shadow` pour logger sans bloquer
- Inspecter les logs : `docker compose logs proxy | grep waf_decision`

### Rate limiting trop agressif

- Ajuster `requests_per_second` et `burst` dans `configs/rate-limit.yaml`
- Vérifier les exemptions IP/path

### Timeout upstream

- Vérifier `response_header_timeout_ms` dans la config proxy
- Le testsite endpoint `/slow` simule des réponses lentes

---

## Limitations

> Ce projet est un **POC éducatif**, pas un CDN/WAF de production.

- **Mono-instance** : pas de distribution, pas de cluster
- **Cache en mémoire** : perdu au redémarrage, limité par la RAM
- **WAF basique** : regex simples, pas de signatures niveau commercial
- **Rate limiting** : token bucket par IP, pas de sliding window
- **Pas de TLS** : HTTP uniquement (TLS offloaded en amont)
- **Pas de HTTP/2-3** : HTTP/1.1 uniquement
- **Pas d'anti-DDoS** : pas de protection L3/L4
- **Pas de multi-upstream** : un seul backend configuré
- **Config statique** : reload nécessite un redémarrage

Voir [docs/08-limitations-roadmap.md](docs/08-limitations-roadmap.md) pour la roadmap.

---

## Structure du projet

```
edge-proxy-poc/
├── proxy/                      # Go reverse proxy
│   ├── cmd/edgeproxy/          # Point d'entrée
│   ├── internal/
│   │   ├── admin/              # Admin API handlers
│   │   ├── app/                # Application lifecycle
│   │   ├── cache/              # Cache in-memory + policy
│   │   ├── config/             # Configuration loading
│   │   ├── middleware/         # RequestID, logging, recovery
│   │   ├── normalize/          # URL/path normalization
│   │   ├── observability/      # Prometheus metrics
│   │   ├── proxy/              # Main proxy handler
│   │   ├── ratelimit/          # Token bucket rate limiter
│   │   ├── security/           # Forward headers
│   │   ├── server/             # HTTP server setup
│   │   ├── upstream/           # Upstream transport
│   │   └── waf/                # WAF engine
│   ├── Dockerfile
│   └── go.mod
├── testsite/                   # Python FastAPI origin
│   ├── app/
│   │   ├── main.py
│   │   ├── cache_cases.py
│   │   ├── auth_cases.py
│   │   ├── waf_cases.py
│   │   ├── api_cases.py
│   │   ├── robustness_cases.py
│   │   ├── debug_cases.py
│   │   └── static/
│   ├── requirements.txt
│   └── Dockerfile
├── e2e/                        # Python E2E tests
│   ├── tests/
│   │   ├── test_cache_basic.py
│   │   ├── test_cache_headers.py
│   │   ├── test_cache_bypass_cookie_auth.py
│   │   ├── test_waf_block_allow_shadow.py
│   │   ├── test_rate_limit.py
│   │   ├── test_upstream_timeout.py
│   │   ├── test_purge.py
│   │   ├── test_metrics_health.py
│   │   ├── test_normalization_bypass_cases.py
│   │   └── test_concurrency_smoke.py
│   ├── utils/
│   ├── conftest.py
│   ├── requirements.txt
│   └── Dockerfile
├── configs/                    # YAML configuration
├── scripts/                    # Operational scripts
├── docs/                       # Documentation
├── docker-compose.yml
├── Makefile
└── README.md
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [Cahier des charges](docs/00-cahier-des-charges.md) | Spécification complète |
| [Architecture](docs/01-architecture.md) | Architecture détaillée |
| [Pipeline requête](docs/02-pipeline-requete.md) | Pipeline de traitement |
| [Stratégie cache](docs/03-cache-strategy.md) | Politique de cache |
| [Stratégie WAF](docs/04-waf-strategy.md) | Politique WAF |
| [Threat model](docs/05-threat-model.md) | Modèle de menace |
| [Plan de tests](docs/06-test-plan.md) | Plan de tests |
| [Runbook ops](docs/07-operations-runbook.md) | Guide opérationnel |
| [Limitations & roadmap](docs/08-limitations-roadmap.md) | Limites et évolutions |

---

## Licence

MIT — voir [LICENSE](LICENSE).
