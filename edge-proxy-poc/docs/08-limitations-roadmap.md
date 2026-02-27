# Limitations & Roadmap — OpenFlare Edge Proxy POC

## Limitations v1

### Fonctionnelles

| Limitation | Impact | Contournement |
|-----------|--------|---------------|
| Cache in-memory uniquement | Perte cache au redémarrage | Redémarrage = warm-up |
| Pas de revalidation ETag/If-None-Match | Re-fetch complet à expiration | TTL adaptés |
| Pas de stale-while-revalidate | Latence sur premier miss post-expiration | Singleflight atténue |
| Un seul upstream | Pas de failover | Configuration v2 |
| WAF règles basiques | Couverture limitée | Extensible via YAML |
| Rate limiting local | Pas de coordination multi-instance | Redis dans v2 |
| Pas de TLS termination | TLS à faire en amont ou pas de TLS | Ajouter dans v2 |
| Pas de compression | Pas de gzip/brotli au proxy | Origin ou Nginx en amont |

### Non-fonctionnelles

| Limitation | Détail |
|-----------|--------|
| Mono-instance | Pas de HA native |
| Mono-région | Pas de distribution géographique |
| Performance non optimisée | POC, pas benchmarké pour production |
| Pas de hot reload config | Redémarrage nécessaire |
| Pas de dashboard | Métriques Prometheus brutes |

### Sécurité

| Limitation | Détail |
|-----------|--------|
| WAF non OWASP CRS | Patterns simples uniquement |
| Pas d'anti-DDoS L3/L4 | Nécessite infrastructure |
| Pas d'anti-bot | Pas de JS challenge |
| Pas de TLS | Trafic non chiffré en v1 |
| Admin API token simple | Pas de rotation, pas de RBAC |

## Roadmap

### v1.1 — Stabilisation et améliorations

**Objectif** : Renforcer la v1 avec des améliorations de cache et résilience.

- [ ] Revalidation ETag / If-None-Match côté cache
- [ ] Stale-while-revalidate local
- [ ] Circuit breaker upstream (avec jitter et backoff)
- [ ] Cache backend disque simple (BoltDB ou fichiers)
- [ ] Dashboard stats minimal (HTML)
- [ ] Hot config reload (signal SIGHUP)
- [ ] Tests de charge légers (wrk / vegeta)
- [ ] Rate limit override par règle WAF

### v1.2 — Multi-upstream et observabilité avancée

**Objectif** : Support multi-origin et traces distribuées.

- [ ] Multi-upstream + health checking
- [ ] Failover automatique
- [ ] Sticky routing simple (cookie-based)
- [ ] DSL de règles plus riche (combinaisons AND/OR)
- [ ] Traces OpenTelemetry (local, Jaeger)
- [ ] Compression gzip/brotli configurable
- [ ] Tests E2E supplémentaires (edge cases)

### v2.0 — Production-ready features

**Objectif** : Fonctionnalités nécessaires pour un déploiement réel.

- [ ] TLS termination avec Let's Encrypt (ACME)
- [ ] Cache distribué (Redis backend)
- [ ] Rate limiting distribué (Redis)
- [ ] Invalidation pub/sub multi-instance
- [ ] API de management REST complète
- [ ] UI web d'administration
- [ ] RBAC pour l'admin API
- [ ] Métriques custom et alerting rules

### v3.0 — Scale et extensibilité

**Objectif** : Architecture extensible et scalable.

- [ ] Plugin system pour rules custom (Go plugins ou WASM)
- [ ] HTTP/3 / QUIC (si stable)
- [ ] Multi-région (coordination)
- [ ] Edge compute (scripts custom par route)
- [ ] Bot management basique
- [ ] API rate limiting avancé (quotas, sliding window)
- [ ] Intégration CI/CD native

## Décisions techniques à revisiter

| Décision v1 | Raison | Revisiter quand |
|-------------|--------|----------------|
| Cache in-memory | Simplicité POC | Quand persistence nécessaire |
| Un seul upstream | Simplicité | Quand failover nécessaire |
| WAF sans scoring | Prévisibilité | Quand finesse nécessaire |
| Rate limit local | Simplicité | Quand multi-instance |
| Pas de TLS | Scope POC | Avant tout déploiement réel |
| Go RE2 regex | Sécurité (anti-ReDoS) | Si PCRE features nécessaires |
