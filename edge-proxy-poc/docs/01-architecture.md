# Architecture — OpenFlare Edge Proxy POC

## Vue d'ensemble

OpenFlare est un reverse proxy HTTP L7 mono-instance, conçu comme un POC de fonctionnalités "edge proxy" :
cache HTTP, WAF, rate limiting, observabilité.

## Architecture macro

```mermaid
flowchart LR
    C[Client HTTP] --> P[Go Edge Proxy<br/>:8080 public<br/>:8081 admin]
    P -->|miss/pass| O[Python Test Site<br/>:8000]
    P --> M[/metrics<br/>Prometheus]
    P --> A[/admin/*<br/>Purge / Stats]
    T[Pytest E2E] --> P
    T -.->|référence| O
```

## Composants internes du proxy Go

```mermaid
flowchart TD
    subgraph "cmd/edgeproxy"
        MAIN[main.go<br/>Entry point]
    end

    subgraph "internal/app"
        APP[app.go<br/>Application lifecycle]
    end

    subgraph "internal/config"
        CFG[config.go<br/>YAML loading + validation]
    end

    subgraph "internal/server"
        SRV[server.go<br/>HTTP servers setup]
    end

    subgraph "internal/middleware"
        MW_RID[requestid.go<br/>Request ID generation]
        MW_LOG[logging.go<br/>Structured access logging]
        MW_REC[recovery.go<br/>Panic recovery]
    end

    subgraph "internal/normalize"
        NORM[normalize.go<br/>Path/query/header normalization]
    end

    subgraph "internal/waf"
        WAF[engine.go<br/>Rule matching + actions]
    end

    subgraph "internal/ratelimit"
        RL[limiter.go<br/>Token bucket per key]
    end

    subgraph "internal/cache"
        CACHE[cache.go<br/>In-memory LRU cache]
        POLICY[policy.go<br/>Eligibility + TTL rules]
    end

    subgraph "internal/proxy"
        PROXY[handler.go<br/>Reverse proxy + cache integration]
    end

    subgraph "internal/upstream"
        UPS[transport.go<br/>HTTP transport config]
    end

    subgraph "internal/observability"
        OBS[metrics.go<br/>Prometheus metrics]
    end

    subgraph "internal/admin"
        ADM[handler.go<br/>Admin API endpoints]
    end

    subgraph "internal/security"
        SEC[headers.go<br/>Header sanitization]
    end

    MAIN --> APP
    APP --> CFG
    APP --> SRV
    SRV --> MW_RID --> MW_LOG --> MW_REC
    MW_REC --> NORM --> WAF --> RL --> PROXY
    PROXY --> CACHE
    PROXY --> UPS
    PROXY --> POLICY
    SRV --> ADM
    APP --> OBS
```

## Stack technique

| Couche | Technologie | Justification |
|--------|-------------|---------------|
| Proxy | Go 1.22+ | Excellent support réseau, binaire unique |
| Origin test | Python 3.12 / FastAPI | Rapidité développement endpoints |
| Tests E2E | Python 3.12 / pytest + httpx | Framework mature pour tests HTTP |
| Conteneurs | Docker + Compose | Déploiement reproductible |
| Métriques | Prometheus client Go | Standard de facto |
| Config | YAML | Lisible, supporté nativement |

## Séparation des responsabilités

### Serveur public (:8080)

Gère le trafic proxy avec la pipeline complète :
Request ID → Limits → Normalize → WAF → Rate Limit → Cache → Proxy → Response

### Serveur admin (:8081)

Gère les opérations d'exploitation :
- Purge cache (exact, prefix, all)
- Stats cache
- Health/readiness (aussi disponible sur :8080)

### Configuration

Fichiers YAML séparés par domaine :
- `proxy.dev.yaml` : configuration runtime générale
- `rules.waf.yaml` : règles WAF
- `cache-rules.yaml` : règles de cache par path
- `rate-limit.yaml` : règles de rate limiting

## Flux de données

```mermaid
sequenceDiagram
    participant C as Client
    participant P as Proxy
    participant Ca as Cache
    participant O as Origin

    C->>P: GET /static/app.v1.js
    P->>P: Request ID, Normalize, WAF, Rate Limit
    P->>Ca: Cache lookup (key)
    alt Cache HIT
        Ca-->>P: Cached response
        P-->>C: 200 + X-Edge-Cache: HIT
    else Cache MISS
        P->>O: Forward request
        O-->>P: 200 + body
        P->>Ca: Store (if eligible)
        P-->>C: 200 + X-Edge-Cache: MISS
    end
```

## Réseau Docker Compose

```mermaid
flowchart LR
    subgraph "frontend network"
        E2E[e2e container]
        PROXY_FE[proxy :8080/:8081]
    end

    subgraph "backend network"
        PROXY_BE[proxy]
        ORIGIN[testsite :8000]
    end

    E2E --> PROXY_FE
    PROXY_BE --> ORIGIN
```
