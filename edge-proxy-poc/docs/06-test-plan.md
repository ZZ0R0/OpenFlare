# Plan de Tests — OpenFlare Edge Proxy POC

## Stratégie de test

### Principes
- Tests **entièrement locaux** (pas de dépendance Internet)
- Tests lancés contre le **proxy**, pas directement contre l'origin
- Assertions **explicites** avec diagnostics utiles
- Timeouts **bornés** sur chaque test
- Couverture des **chemins nominaux et d'erreur**

### Niveaux de test

| Niveau | Technologie | Périmètre |
|--------|-------------|-----------|
| Unit | Go `testing` | Modules internes (cache, WAF, config, normalize, ratelimit) |
| Integration | Go `testing` | Combinaisons de modules |
| E2E | Python pytest + httpx | Pipeline complète via Docker Compose |
| Smoke | Shell + curl | Vérification rapide post-déploiement |

## Tests unitaires Go

### cache/

| Test | Description |
|------|-------------|
| `TestCacheKeyGeneration` | Vérifier la clé normalisée |
| `TestCacheEligibilityGET` | GET → eligible |
| `TestCacheEligibilityPOST` | POST → bypass |
| `TestCacheEligibilityWithCookie` | Cookie → bypass |
| `TestCacheEligibilityWithAuth` | Authorization → bypass |
| `TestCacheStorePolicyNoStore` | no-store → pas stocké |
| `TestCacheStorePolicyPrivate` | private → pas stocké |
| `TestCacheStorePolicySetCookie` | Set-Cookie → pas stocké |
| `TestCacheLRUEviction` | Éviction quand max atteint |
| `TestCacheTTLExpiration` | Entrée expirée non retournée |
| `TestCacheStats` | Compteurs corrects |

### waf/

| Test | Description |
|------|-------------|
| `TestPathTraversalMatch` | `../` et variantes détectés |
| `TestSQLiMatch` | Patterns SQLi détectés |
| `TestXSSMatch` | Patterns XSS détectés |
| `TestNormalRequestNoMatch` | Requête normale → pas de match |
| `TestShadowModeNoBlock` | Mode shadow → log sans block |
| `TestEnforceModeBlock` | Mode enforce → block |
| `TestDisabledRuleSkipped` | Règle disabled → ignorée |
| `TestBodySizeLimit` | Body trop gros → pas inspecté |

### config/

| Test | Description |
|------|-------------|
| `TestLoadValidConfig` | Config valide chargée |
| `TestLoadInvalidConfig` | Config invalide → erreur claire |
| `TestDefaultValues` | Valeurs par défaut appliquées |
| `TestMissingRequired` | Champ requis manquant → erreur |

### normalize/

| Test | Description |
|------|-------------|
| `TestNormalizePath` | Double slashes, dots, encoding |
| `TestNormalizeQuery` | Tri alphabétique, décodage |
| `TestPathTraversalNormalization` | `/../` résolu |

### ratelimit/

| Test | Description |
|------|-------------|
| `TestTokenBucketAllow` | Sous la limite → allow |
| `TestTokenBucketDeny` | Au-dessus → deny |
| `TestTokenBucketRefill` | Après attente → allow |
| `TestBurstCapacity` | Burst initial respecté |

## Tests E2E Python

### Configuration

```ini
# pytest.ini
[pytest]
testpaths = tests
timeout = 30
markers =
    cache: Cache-related tests
    waf: WAF-related tests
    ratelimit: Rate limiting tests
    smoke: Quick smoke tests
```

### Suite 1 : Cache basique (`test_cache_basic.py`)

| Test | Vérification |
|------|-------------|
| `test_static_asset_miss_then_hit` | 1er GET → MISS, 2ème → HIT, contenu identique |
| `test_head_request_cached` | HEAD request utilise le cache |
| `test_post_not_cached` | POST → BYPASS |
| `test_cache_stores_headers` | Headers origin préservés dans le cache |

### Suite 2 : Cache headers (`test_cache_headers.py`)

| Test | Vérification |
|------|-------------|
| `test_no_store_not_cached` | `Cache-Control: no-store` → pas stocké |
| `test_private_not_cached` | `Cache-Control: private` → pas stocké |
| `test_set_cookie_not_cached` | `Set-Cookie` → pas stocké |
| `test_public_short_ttl` | TTL court respecté |
| `test_public_long_ttl` | TTL long respecté |

### Suite 3 : Cache bypass (`test_cache_bypass_cookie_auth.py`)

| Test | Vérification |
|------|-------------|
| `test_cookie_bypass` | Requête avec Cookie → BYPASS |
| `test_authorization_bypass` | Requête avec Authorization → BYPASS |
| `test_bypass_no_store` | Bypass ne stocke pas en cache |

### Suite 4 : WAF (`test_waf_block_allow_shadow.py`)

| Test | Vérification |
|------|-------------|
| `test_waf_shadow_path_traversal` | Mode shadow : 200 + log WAF |
| `test_waf_enforce_path_traversal` | Mode enforce : 403 |
| `test_waf_enforce_sqli` | SQL injection → 403 |
| `test_waf_normal_request_passes` | Requête normale → 200 |
| `test_waf_xss_shadow` | XSS en shadow → 200 + log |

### Suite 5 : Rate limiting (`test_rate_limit.py`)

| Test | Vérification |
|------|-------------|
| `test_rate_limit_burst` | Burst de requêtes → certains 429 |
| `test_rate_limit_recovery` | Après délai → requêtes passent |
| `test_rate_limit_retry_after` | Header Retry-After présent |

### Suite 6 : Upstream timeout (`test_upstream_timeout.py`)

| Test | Vérification |
|------|-------------|
| `test_slow_origin_timeout` | `/slow?delay_ms=5000` → 504 |
| `test_slow_origin_within_timeout` | `/slow?delay_ms=100` → 200 |

### Suite 7 : Purge (`test_purge.py`)

| Test | Vérification |
|------|-------------|
| `test_purge_exact_key` | Store → purge → MISS |
| `test_purge_prefix` | Store multiple → purge prefix → MISS |
| `test_purge_all` | Store → purge all → tout MISS |

### Suite 8 : Metrics/health (`test_metrics_health.py`)

| Test | Vérification |
|------|-------------|
| `test_healthz` | `/healthz` → 200 |
| `test_readyz` | `/readyz` → 200 |
| `test_metrics_contains_keys` | `/metrics` contient métriques attendues |

### Suite 9 : Normalization (`test_normalization_bypass_cases.py`)

| Test | Vérification |
|------|-------------|
| `test_double_slash_normalized` | `//static//app.v1.js` → même cache key |
| `test_encoded_path_cached` | `/static/app%2Ev1.js` → cache cohérent |
| `test_query_order_irrelevant` | `?a=1&b=2` et `?b=2&a=1` → même cache key |

### Suite 10 : Concurrency (`test_concurrency_smoke.py`)

| Test | Vérification |
|------|-------------|
| `test_concurrent_requests_stable` | 50 requêtes concurrentes → pas de crash |
| `test_singleflight_effect` | Après warmup, 50 requêtes → ratio HIT élevé |

## Critères de passage

- [ ] Tous les tests E2E passent (`pytest` exit code 0)
- [ ] Zéro crash/panic du proxy pendant les tests
- [ ] Logs proxy cohérents (pas d'erreurs inattendues)
- [ ] Temps total tests < 2 minutes
- [ ] Tests reproductibles (relance = même résultat)
