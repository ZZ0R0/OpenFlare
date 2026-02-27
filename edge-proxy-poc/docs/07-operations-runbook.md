# Operations Runbook — OpenFlare Edge Proxy POC

## Démarrage

### Démarrage complet

```bash
docker compose up --build -d
```

### Vérification santé

```bash
# Proxy
curl -s http://localhost:8080/healthz
# → {"status": "ok"}

# Testsite
curl -s http://localhost:8000/healthz
# → {"status": "ok"}
```

### Logs en temps réel

```bash
docker compose logs -f proxy
docker compose logs -f testsite
```

## Arrêt

```bash
docker compose down
# Avec suppression des volumes :
docker compose down -v
```

## Tests

### Lancer les tests E2E

```bash
docker compose run --rm e2e
# Ou avec un test spécifique :
docker compose run --rm e2e pytest tests/test_cache_basic.py -v
```

### Smoke test rapide

```bash
./scripts/smoke.sh
```

## Interprétation des headers

### `X-Edge-Cache`

| Valeur | Signification | Action |
|--------|--------------|--------|
| `HIT` | Servi depuis le cache | Normal pour assets statiques |
| `MISS` | Pas en cache, fetché depuis origin | Normal pour première requête |
| `BYPASS` | Non éligible au cache | Cookie, Auth, méthode non-GET/HEAD |
| `EXPIRED` | Cache expiré, re-fetch | TTL dépassé |

### Diagnostic BYPASS inattendu

1. Vérifier la présence de `Cookie` ou `Authorization` dans la requête
2. Vérifier que la méthode est GET ou HEAD
3. Vérifier les règles de cache (`configs/cache-rules.yaml`)
4. Vérifier les logs proxy pour la raison du bypass

### `X-Request-ID`

UUID unique par requête, utilisable pour chercher dans les logs :

```bash
docker compose logs proxy | grep "REQUEST_ID_HERE"
```

## Cache

### Purge par URL exacte

```bash
curl -X POST http://localhost:8081/admin/cache/purge \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"url": "/static/app.v1.js"}'
```

### Purge par préfixe

```bash
curl -X POST http://localhost:8081/admin/cache/purge-prefix \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"prefix": "/static/"}'
```

### Purge complète (dev/test uniquement)

```bash
curl -X POST http://localhost:8081/admin/cache/purge-all \
  -H "Authorization: Bearer change-me"
```

⚠️ **Attention** : la purge complète en production peut causer un pic de charge sur l'origin.

### Stats cache

```bash
curl -s http://localhost:8081/admin/cache/stats \
  -H "Authorization: Bearer change-me" | jq .
```

## WAF

### Changer le mode shadow → enforce

Modifier `configs/rules.waf.yaml` :

```yaml
mode: "enforce"  # était "shadow"
```

Puis redémarrer le proxy :

```bash
docker compose restart proxy
```

### Tester une règle WAF

```bash
# Test path traversal (doit être bloqué en enforce)
curl -v http://localhost:8080/../etc/passwd

# Test SQLi (doit être bloqué en enforce)
curl -v "http://localhost:8080/search?q=1%27%20OR%201=1--"

# Test XSS (log en shadow)
curl -v "http://localhost:8080/search?q=<script>alert(1)</script>"
```

### Lire les logs WAF

```bash
docker compose logs proxy | grep '"waf"'
```

## Rate Limiting

### Simuler un hit rate limit

```bash
# Envoyer 30 requêtes rapides
for i in $(seq 1 30); do
  curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/time
done
```

### Vérifier le comportement

Attendu : les premières requêtes → 200, puis 429.

## Métriques

### Accéder aux métriques Prometheus

```bash
curl -s http://localhost:8080/metrics | grep openflare
```

### Métriques clés

```
openflare_requests_total{method="GET", status="200"} 42
openflare_cache_operations_total{operation="hit"} 30
openflare_cache_operations_total{operation="miss"} 12
openflare_waf_decisions_total{action="block"} 2
openflare_ratelimit_total{decision="deny"} 5
```

## Timeout Origin

### Simuler un timeout

```bash
# L'origin répond après 5 secondes (timeout proxy à 3s)
curl -v "http://localhost:8080/slow?delay_ms=5000"
# Attendu : 504 Gateway Timeout
```

### Vérifier les logs

```bash
docker compose logs proxy | grep "upstream_timeout"
```

## Troubleshooting

### Le proxy ne démarre pas

1. Vérifier les logs : `docker compose logs proxy`
2. Vérifier la config YAML (erreurs de syntaxe)
3. Vérifier que le port 8080/8081 n'est pas utilisé

### Le testsite ne répond pas

1. Vérifier les logs : `docker compose logs testsite`
2. Vérifier que le port 8000 n'est pas utilisé
3. Vérifier le healthcheck : `curl http://localhost:8000/healthz`

### Les tests E2E échouent

1. Vérifier que proxy et testsite sont healthy
2. Vérifier les logs des deux services
3. Lancer un seul test pour isoler : `docker compose run --rm e2e pytest tests/test_cache_basic.py::test_static_asset_miss_then_hit -v`
4. Vérifier la config proxy utilisée (test vs dev)

### Cache ne fonctionne pas

1. Vérifier `cache.enabled: true` dans la config
2. Vérifier les headers de debug `X-Edge-Cache`
3. Vérifier que la requête est éligible (GET, pas de Cookie/Auth)
4. Vérifier les règles de cache
5. Consulter les stats cache : `/admin/cache/stats`
