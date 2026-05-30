# Rust /rewrite Microservice — Clean Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Same outcome as the Go plan ([[2026-05-29-go-rewrite-microservice]]) — extract `/rewrite` into a standalone microservice — but in Rust. Compile-time-enforced hexagonal layering via Cargo workspace crates. Used as a deeper learning vehicle for Rust language + architecture in one go. Runs alongside NestJS (and optionally alongside the Go service, if both are built).

**Architecture:** Cargo workspace with one crate per architectural layer. The dependency rule is enforced by Cargo's manifest — `domain` crate's `Cargo.toml` cannot list `axum`/`sqlx`/`reqwest`, so accidental imports fail at build time. Async via Tokio. Ports defined as `trait`s in `domain`; adapters implement them in separate crates. Composition root in the `http` binary crate.

**Tech Stack:** Rust 1.78+, Tokio runtime, Axum (HTTP), sqlx (compile-time-checked SQL), jsonwebtoken, reqwest (HTTP client with streaming), `redis::aio`, `tracing` + `tracing-subscriber` (structured logs), `thiserror` (error types), `tower` + `tower-http` (middleware), `async-trait` (until async fn in trait fully ergonomic), `mockall` (test doubles), `criterion` (benchmarks).

**Why pick this over the Go plan:** stronger compile-time guarantees, lower runtime overhead, ownership model prevents data races by construction. Pick this when language + architecture learning is more important than time-to-ship. Reading time before first PR: ~20 hours of "The Rust Programming Language" + Tokio tutorial.

---

## ⛔ Gate: Async trait + lifetime fluency must come before Task 4

Tasks 1–3 (scaffold + JWT + DB) are mostly mechanical and forgiving. **Task 4 onward — Domain ports — is where async trait + lifetime questions force a real decision.** Before crossing that line:

1. **Read async-book.dev (Tokio chapter) + "async fn in trait" RFC stabilization notes.**
2. **Decide:** `async-trait` macro crate (small runtime cost, ergonomic) OR raw `impl Future` returns (zero cost, harder to write). Recommend `async-trait` for v1, revisit later.
3. **Know your Send + Sync bounds** — every trait method that runs in an Axum handler needs `Send`. Don't paper over this with `Box::pin` everywhere.

If you can't comfortably write `#[async_trait] pub trait UserRepo: Send + Sync { ... }` from memory, stop. Read more first.

---

## Boundary rules (write down day 1, never violate)

1. **NestJS owns the schema.** Rust uses `sqlx::query!` macro which validates SQL against the live Postgres at compile time. No Rust-side migrations.
2. **Cargo workspace enforces the dependency rule.** `domain/Cargo.toml` lists ONLY serde, thiserror, chrono, async-trait, futures-core. Anything else there = lint failure in code review.
3. **Each crate has its own error type.** `domain::Error`, `usecase::Error`, `adapter_pg::Error`. Convert at boundaries via `From` impls. Never `unwrap()` outside tests.
4. **No `.unwrap()` / `.expect()` in src/. Only in tests + `main.rs` startup.** Add `#![forbid(clippy::unwrap_used, clippy::expect_used)]` to lib.rs.
5. **JWT_SECRET shared via env.** Same hash check on both sides; rotate via dual-secret window or move to RS256.
6. **Postgres role split.** Rust service uses `dr_rewrite_rs` role: R on `users`/`plans`/`ai_providers`, RW on `usage_logs`.
7. **Async cancellation is non-negotiable.** Every long-running future receives a `tokio_util::sync::CancellationToken` or `Drop` cleanup; no leaked tasks.
8. **Each crate has its own test target.** Domain tests live in `domain/tests/`; integration tests for adapters in `adapter_pg/tests/` (with `testcontainers`).

---

## File Structure

```
backend-rewrite-rs/                            # new sibling of /backend/ and /backend-rewrite-go/
├── Cargo.toml                                 # workspace root
├── Cargo.lock
├── rust-toolchain.toml                        # pin to stable 1.78
├── .cargo/config.toml                         # mold linker, rustflags
├── crates/
│   ├── domain/
│   │   ├── Cargo.toml                         # deps: serde, thiserror, chrono, async-trait, futures-core, uuid
│   │   └── src/
│   │       ├── lib.rs                         # pub use everything
│   │       ├── user.rs                        # struct User, impl CheckQuota
│   │       ├── rewrite_request.rs             # struct RewriteRequest with private constructor
│   │       ├── tone.rs                        # enum Tone with FromStr
│   │       ├── ports.rs                       # trait UserRepo, AiProvider, UsageWriter, RateLimiter
│   │       └── errors.rs                      # thiserror enum
│   ├── usecase/
│   │   ├── Cargo.toml                         # deps: domain, futures, tokio (sync only), thiserror
│   │   └── src/
│   │       ├── lib.rs
│   │       └── rewrite.rs                     # pub async fn rewrite(...) generic over R: UserRepo, P: AiProvider
│   ├── adapter_pg/
│   │   ├── Cargo.toml                         # deps: domain, sqlx (postgres + chrono + uuid), tokio
│   │   └── src/lib.rs                         # impl UserRepo for PgUserRepo, impl UsageWriter for PgUsageWriter
│   ├── adapter_openai/
│   │   ├── Cargo.toml                         # deps: domain, reqwest (with stream), tokio, serde, futures
│   │   └── src/lib.rs                         # impl AiProvider for OpenAi
│   ├── adapter_anthropic/
│   ├── adapter_ollama/
│   ├── adapter_redis/
│   │   └── src/lib.rs                         # impl RateLimiter
│   ├── adapter_memory/                        # in-memory fakes (cfg(test) features in other crates)
│   │   └── src/lib.rs
│   └── http/                                  # binary crate — composition root
│       ├── Cargo.toml                         # deps: all adapter_*, usecase, axum, tokio, tower, tracing, ...
│       └── src/
│           ├── main.rs                        # tokio::main, env load, wire adapters, start Axum
│           ├── config.rs                      # AppConfig via figment or envy
│           ├── error.rs                       # HttpError, IntoResponse impl
│           ├── auth.rs                        # JWT extractor (FromRequestParts impl)
│           ├── middleware/
│           │   ├── logging.rs
│           │   ├── correlation.rs
│           │   └── metrics.rs
│           └── handler/
│               ├── health.rs
│               └── rewrite.rs                 # POST /rewrite — SSE stream
├── migrations/                                # sqlx-cli mirror of NestJS migrations (read-only)
├── .sqlx/                                     # generated offline query metadata
├── Dockerfile                                 # multi-stage; final = distroless static, ~25 MB
├── docker-compose.dev.yml
├── deploy/
│   ├── service.yml
│   └── Caddyfile.snippet
├── scripts/
│   ├── deploy.sh
│   ├── sqlx-prepare-check.sh                  # CI gate: fail if `cargo sqlx prepare --check` dirty
│   └── load-test.sh
├── tests/                                     # workspace-level integration tests
└── README.md                                  # boundary rules + quick-start
```

**Existing files to modify (in main repo):**
- `Caddyfile` — add route for `/rewrite-rs` (Task 10) and later `/rewrite` (Task 12).
- `deploy.sh` — migration → NestJS → cargo sqlx prepare → Rust deploy ordering.

---

## Task 1: Workspace scaffold + Hello World + Docker

**Files:**
- Create: `Cargo.toml` (workspace), `crates/http/Cargo.toml`, `crates/http/src/main.rs`, `Dockerfile`, `docker-compose.dev.yml`, `.cargo/config.toml`, `rust-toolchain.toml`, `README.md`

- [ ] **Step 1: Init workspace**

```bash
mkdir backend-rewrite-rs && cd backend-rewrite-rs
cat > Cargo.toml <<'EOF'
[workspace]
resolver = "2"
members = [
  "crates/domain",
  "crates/usecase",
  "crates/adapter_pg",
  "crates/adapter_openai",
  "crates/adapter_anthropic",
  "crates/adapter_ollama",
  "crates/adapter_redis",
  "crates/adapter_memory",
  "crates/http",
]

[workspace.package]
edition = "2021"
rust-version = "1.78"

[workspace.dependencies]
tokio = { version = "1", features = ["full"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "1"
async-trait = "0.1"
futures = "0.3"
tracing = "0.1"
tracing-subscriber = { version = "0.3", features = ["env-filter", "json"] }
uuid = { version = "1", features = ["v4", "serde"] }
chrono = { version = "0.4", features = ["serde"] }
EOF
mkdir -p crates/http/src
```

- [ ] **Step 2: Minimal http crate**

```toml
# crates/http/Cargo.toml
[package]
name = "rewrite-rs-http"
version = "0.1.0"
edition.workspace = true

[dependencies]
tokio.workspace = true
axum = "0.7"
serde_json.workspace = true
tracing.workspace = true
tracing-subscriber.workspace = true
```

```rust
// crates/http/src/main.rs
use axum::{Router, routing::{get, post}, response::Json};
use serde_json::{json, Value};

async fn health() -> Json<Value> { Json(json!({"status":"ok","service":"rewrite-rs"})) }
async fn rewrite_stub() -> Json<Value> { Json(json!({"text":"Hello from Rust!","tone":"placeholder"})) }

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt().json().init();
    let app = Router::new()
        .route("/health", get(health))
        .route("/rewrite", post(rewrite_stub));
    let listener = tokio::net::TcpListener::bind("0.0.0.0:3002").await.unwrap();
    tracing::info!(addr = ?listener.local_addr().unwrap(), "listening");
    axum::serve(listener, app).await.unwrap();
}
```

- [ ] **Step 3: Multi-stage Dockerfile (distroless static, ~25 MB)**

```dockerfile
FROM rust:1.78-slim AS builder
WORKDIR /src
COPY . .
RUN apt-get update && apt-get install -y mold && \
    RUSTFLAGS="-C link-arg=-fuse-ld=mold" \
    cargo build --release --bin rewrite-rs-http

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=builder /src/target/release/rewrite-rs-http /server
USER nonroot:nonroot
EXPOSE 3002
ENTRYPOINT ["/server"]
```

- [ ] **Step 4: `.cargo/config.toml` — use mold linker for faster builds**

```toml
[target.x86_64-unknown-linux-gnu]
linker = "clang"
rustflags = ["-C", "link-arg=-fuse-ld=mold"]

[profile.dev]
debug = 1                # faster builds, still debuggable
incremental = true

[profile.release]
lto = "thin"
codegen-units = 1
strip = true
```

- [ ] **Step 5: Verify**

```bash
docker compose -f docker-compose.dev.yml up --build rewrite-rs
curl http://localhost:3002/health
curl -X POST http://localhost:3002/rewrite
```

- [ ] **Step 6: Commit**

```bash
git add backend-rewrite-rs
git commit -m "feat(rewrite-rs): workspace scaffold + Axum hello world + distroless Docker"
```

---

## Task 2: JWT auth extractor (Axum FromRequestParts)

**Files:**
- Create: `crates/http/src/auth.rs`, `crates/http/src/error.rs`
- Modify: `crates/http/Cargo.toml` (add jsonwebtoken), `crates/http/src/main.rs`
- Test: `crates/http/src/auth.rs` (mod tests)

- [ ] **Step 1: Add deps**

```toml
# crates/http/Cargo.toml additions
jsonwebtoken = "9"
serde.workspace = true
thiserror.workspace = true
```

- [ ] **Step 2: Error type**

```rust
// crates/http/src/error.rs
use axum::{http::StatusCode, response::{IntoResponse, Response}, Json};
use serde_json::json;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum HttpError {
    #[error("unauthorized: {0}")]
    Unauthorized(&'static str),
    #[error("forbidden")]
    Forbidden,
    #[error("bad request: {0}")]
    BadRequest(String),
    #[error("rate limited")]
    TooManyRequests,
    #[error("upstream unavailable")]
    ServiceUnavailable,
    #[error("internal: {0}")]
    Internal(#[from] anyhow::Error),
}

impl IntoResponse for HttpError {
    fn into_response(self) -> Response {
        let (status, msg) = match &self {
            HttpError::Unauthorized(m) => (StatusCode::UNAUTHORIZED, m.to_string()),
            HttpError::Forbidden => (StatusCode::FORBIDDEN, "forbidden".into()),
            HttpError::BadRequest(m) => (StatusCode::BAD_REQUEST, m.clone()),
            HttpError::TooManyRequests => (StatusCode::TOO_MANY_REQUESTS, "rate limited".into()),
            HttpError::ServiceUnavailable => (StatusCode::SERVICE_UNAVAILABLE, "no provider".into()),
            HttpError::Internal(_) => (StatusCode::INTERNAL_SERVER_ERROR, "internal".into()),
        };
        (status, Json(json!({"error": msg}))).into_response()
    }
}
```

- [ ] **Step 3: JWT extractor**

```rust
// crates/http/src/auth.rs
use axum::{
    extract::FromRequestParts, http::request::Parts, http::header::AUTHORIZATION,
    async_trait,
};
use jsonwebtoken::{decode, DecodingKey, Validation, Algorithm};
use serde::Deserialize;
use uuid::Uuid;
use crate::error::HttpError;

#[derive(Debug, Clone, Deserialize)]
pub struct Claims {
    pub sub: String,
    #[serde(default)]
    pub role: String,
    pub exp: usize,
}

#[derive(Debug, Clone)]
pub struct AuthUser {
    pub id: Uuid,
    pub role: String,
}

#[derive(Clone)]
pub struct JwtConfig { pub secret: Vec<u8> }

#[async_trait]
impl<S> FromRequestParts<S> for AuthUser
where
    S: Send + Sync,
    JwtConfig: axum::extract::FromRef<S>,
{
    type Rejection = HttpError;
    async fn from_request_parts(parts: &mut Parts, state: &S) -> Result<Self, Self::Rejection> {
        let cfg = JwtConfig::from_ref(state);
        let h = parts.headers.get(AUTHORIZATION)
            .and_then(|h| h.to_str().ok())
            .ok_or(HttpError::Unauthorized("missing authorization"))?;
        let token = h.strip_prefix("Bearer ").ok_or(HttpError::Unauthorized("not bearer"))?;
        let data = decode::<Claims>(token, &DecodingKey::from_secret(&cfg.secret), &Validation::new(Algorithm::HS256))
            .map_err(|_| HttpError::Unauthorized("invalid token"))?;
        let id = Uuid::parse_str(&data.claims.sub)
            .map_err(|_| HttpError::Unauthorized("bad sub claim"))?;
        Ok(AuthUser { id, role: data.claims.role })
    }
}
```

- [ ] **Step 4: Write the failing test**

```rust
// crates/http/src/auth.rs (mod tests)
#[cfg(test)]
mod tests {
    use super::*;
    use jsonwebtoken::{encode, EncodingKey, Header};
    use chrono::Utc;

    fn sign(secret: &[u8], sub: &str) -> String {
        let claims = Claims { sub: sub.into(), role: "user".into(), exp: (Utc::now().timestamp() + 3600) as usize };
        encode(&Header::new(Algorithm::HS256), &claims, &EncodingKey::from_secret(secret)).unwrap()
    }

    #[test]
    fn parses_a_valid_token() {
        let secret = b"test-secret";
        let t = sign(secret, "f47ac10b-58cc-4372-a567-0e02b2c3d479");
        let key = DecodingKey::from_secret(secret);
        let data = decode::<Claims>(&t, &key, &Validation::new(Algorithm::HS256)).unwrap();
        assert_eq!(data.claims.sub, "f47ac10b-58cc-4372-a567-0e02b2c3d479");
    }

    #[test]
    fn rejects_wrong_signature() {
        let t = sign(b"right", "f47ac10b-58cc-4372-a567-0e02b2c3d479");
        let key = DecodingKey::from_secret(b"wrong");
        assert!(decode::<Claims>(&t, &key, &Validation::new(Algorithm::HS256)).is_err());
    }
}
```

- [ ] **Step 5: Run + commit**

```bash
cargo test -p rewrite-rs-http
git commit -am "feat(rewrite-rs): JWT extractor (FromRequestParts) sharing NestJS HS256 secret"
```

---

## Task 3: Postgres via sqlx (compile-time SQL check)

**Files:**
- Create: `crates/adapter_pg/Cargo.toml`, `crates/adapter_pg/src/lib.rs`, `.sqlx/` (after prepare)
- Modify: workspace `Cargo.toml`

- [ ] **Step 1: Crate scaffold**

```toml
# crates/adapter_pg/Cargo.toml
[package]
name = "rewrite-rs-adapter-pg"
version = "0.1.0"
edition.workspace = true

[dependencies]
domain = { path = "../domain" }      # added in Task 4
sqlx = { version = "0.7", features = ["runtime-tokio-rustls", "postgres", "uuid", "chrono", "macros"] }
tokio.workspace = true
async-trait.workspace = true
chrono.workspace = true
uuid.workspace = true
thiserror.workspace = true
```

- [ ] **Step 2: Pool builder**

```rust
// crates/adapter_pg/src/lib.rs
use sqlx::postgres::{PgPool, PgPoolOptions};

pub async fn new_pool(url: &str) -> Result<PgPool, sqlx::Error> {
    PgPoolOptions::new()
        .max_connections(25)
        .min_connections(5)
        .connect(url)
        .await
}
```

- [ ] **Step 3: Generate offline metadata (CI gate)**

```bash
# Local one-time:
cargo install sqlx-cli --no-default-features --features postgres
export DATABASE_URL="postgres://draftright:draftright@localhost:5432/draftright"
cargo sqlx prepare --workspace
# Generates .sqlx/ — commit it. CI runs `cargo sqlx prepare --check` to catch drift.
```

- [ ] **Step 4: `scripts/sqlx-prepare-check.sh`**

```bash
#!/bin/bash
set -e
cargo sqlx prepare --workspace --check
```

Add to CI.

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(rewrite-rs): sqlx pool + offline query metadata"
```

---

## Task 4: Domain crate (entities + ports)

**Files:**
- Create: `crates/domain/Cargo.toml`, `crates/domain/src/{lib,user,rewrite_request,tone,ports,errors}.rs`
- Test: `crates/domain/tests/user_test.rs`

- [ ] **Step 1: Crate Cargo.toml — note the dep allow-list**

```toml
[package]
name = "rewrite-rs-domain"
version = "0.1.0"
edition.workspace = true

[dependencies]
serde.workspace = true
thiserror.workspace = true
async-trait.workspace = true
futures.workspace = true       # for Stream type in trait sig
chrono.workspace = true
uuid.workspace = true
# NOT permitted here: sqlx, reqwest, axum, tokio (full)
# Cargo will refuse if a downstream PR sneaks one in.
```

- [ ] **Step 2: Errors**

```rust
// crates/domain/src/errors.rs
use thiserror::Error;

#[derive(Debug, Error)]
pub enum Error {
    #[error("quota exceeded")]
    QuotaExceeded,
    #[error("user not found")]
    UserNotFound,
    #[error("no active ai provider")]
    NoProvider,
    #[error("invalid input: {0}")]
    InvalidInput(&'static str),
    #[error("provider error: {0}")]
    Provider(String),
}
```

- [ ] **Step 3: User entity with invariant**

```rust
// crates/domain/src/user.rs
use uuid::Uuid;
use serde::{Serialize, Deserialize};
use crate::errors::Error;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
pub struct UserId(pub Uuid);

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Plan {
    pub id: String,
    pub daily_limit: i32,    // 0 = unlimited
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct User {
    pub id: UserId,
    pub email: String,
    pub plan: Plan,
    pub usage_today: i32,
}

impl User {
    pub fn check_quota(&self) -> Result<(), Error> {
        if self.plan.daily_limit > 0 && self.usage_today >= self.plan.daily_limit {
            return Err(Error::QuotaExceeded);
        }
        Ok(())
    }
}
```

- [ ] **Step 4: Tone enum + RewriteRequest with private constructor**

```rust
// crates/domain/src/tone.rs
use serde::{Serialize, Deserialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Tone {
    Simple, Natural, Polished, Concise, Technical, Claude, Translate,
}

impl std::str::FromStr for Tone {
    type Err = crate::errors::Error;
    fn from_str(s: &str) -> Result<Self, Self::Err> {
        Ok(match s {
            "simple" => Tone::Simple,
            "natural" => Tone::Natural,
            "polished" => Tone::Polished,
            "concise" => Tone::Concise,
            "technical" => Tone::Technical,
            "claude" => Tone::Claude,
            "translate" => Tone::Translate,
            _ => return Err(crate::errors::Error::InvalidInput("unknown tone")),
        })
    }
}
```

```rust
// crates/domain/src/rewrite_request.rs
use crate::errors::Error;
use crate::tone::Tone;

#[derive(Debug, Clone)]
pub struct RewriteRequest {
    text: String,
    tone: Tone,
    lang: Option<String>,
}

impl RewriteRequest {
    pub fn new(text: String, tone: Tone, lang: Option<String>) -> Result<Self, Error> {
        let t = text.trim();
        if t.is_empty() || t.len() > 5000 {
            return Err(Error::InvalidInput("text length 1..=5000"));
        }
        Ok(Self { text: t.to_string(), tone, lang })
    }
    pub fn text(&self) -> &str { &self.text }
    pub fn tone(&self) -> Tone { self.tone }
    pub fn lang(&self) -> Option<&str> { self.lang.as_deref() }
}
```

- [ ] **Step 5: Ports**

```rust
// crates/domain/src/ports.rs
use async_trait::async_trait;
use futures::stream::Stream;
use std::pin::Pin;
use crate::user::{User, UserId};
use crate::rewrite_request::RewriteRequest;
use crate::errors::Error;

pub type TokenStream = Pin<Box<dyn Stream<Item = Result<String, Error>> + Send>>;

#[async_trait]
pub trait UserRepo: Send + Sync {
    async fn find(&self, id: UserId) -> Result<User, Error>;
    async fn increment_usage(&self, id: UserId) -> Result<(), Error>;
}

#[async_trait]
pub trait AiProvider: Send + Sync {
    async fn stream(&self, req: &RewriteRequest) -> Result<TokenStream, Error>;
    fn name(&self) -> &'static str;
}

#[async_trait]
pub trait UsageWriter: Send + Sync {
    async fn record(&self, log: UsageLog);
}

#[async_trait]
pub trait RateLimiter: Send + Sync {
    async fn allow(&self, id: UserId) -> Result<bool, Error>;
}

#[derive(Debug, Clone)]
pub struct UsageLog {
    pub user_id: UserId,
    pub provider_id: String,
    pub tone: crate::tone::Tone,
    pub input_chars: i32,
    pub output_chars: i32,
    pub latency_ms: i64,
}
```

- [ ] **Step 6: Tests**

```rust
// crates/domain/tests/user_test.rs
use rewrite_rs_domain::*;
use uuid::Uuid;

#[test]
fn quota_under_limit() {
    let u = User { id: UserId(Uuid::nil()), email: "x".into(), plan: Plan { id: "free".into(), daily_limit: 100 }, usage_today: 50 };
    assert!(u.check_quota().is_ok());
}
#[test]
fn quota_at_limit() {
    let u = User { id: UserId(Uuid::nil()), email: "x".into(), plan: Plan { id: "free".into(), daily_limit: 100 }, usage_today: 100 };
    assert!(matches!(u.check_quota(), Err(Error::QuotaExceeded)));
}
#[test]
fn quota_unlimited_plan_never_blocks() {
    let u = User { id: UserId(Uuid::nil()), email: "x".into(), plan: Plan { id: "pro".into(), daily_limit: 0 }, usage_today: 999_999 };
    assert!(u.check_quota().is_ok());
}
```

- [ ] **Step 7: Run + commit**

```bash
cargo test -p rewrite-rs-domain
git commit -am "feat(rewrite-rs): domain crate — entities + ports + invariants"
```

---

## Task 5: Use case crate (orchestration)

**Files:**
- Create: `crates/usecase/Cargo.toml`, `crates/usecase/src/{lib,rewrite}.rs`, `crates/usecase/tests/rewrite_test.rs`

- [ ] **Step 1: Use case function — generic over ports**

```rust
// crates/usecase/src/rewrite.rs
use rewrite_rs_domain::{
    user::UserId, rewrite_request::RewriteRequest,
    ports::{UserRepo, AiProvider, RateLimiter, TokenStream},
    errors::Error,
};

pub struct RewriteDeps<'a, R, P, L>
where
    R: UserRepo + 'a,
    P: AiProvider + 'a,
    L: RateLimiter + 'a,
{
    pub users: &'a R,
    pub provider: &'a P,
    pub rate_limit: &'a L,
}

pub async fn rewrite<R, P, L>(
    deps: RewriteDeps<'_, R, P, L>,
    user_id: UserId,
    req: RewriteRequest,
) -> Result<TokenStream, Error>
where
    R: UserRepo, P: AiProvider, L: RateLimiter,
{
    if !deps.rate_limit.allow(user_id).await? {
        return Err(Error::QuotaExceeded);
    }
    let user = deps.users.find(user_id).await?;
    user.check_quota()?;
    deps.users.increment_usage(user_id).await?;
    deps.provider.stream(&req).await
}
```

- [ ] **Step 2: Test with in-memory fakes (lives in `adapter_memory` crate, but tested here)**

```rust
// crates/usecase/tests/rewrite_test.rs
// Full test with hand-rolled fake repo + fake provider + fake limiter.
// Cases: happy path, quota exceeded, provider error, rate-limit denied.
```

- [ ] **Step 3: Commit**

```bash
cargo test -p rewrite-rs-usecase
git commit -am "feat(rewrite-rs): use case crate — rewrite orchestration generic over ports"
```

---

## Task 6: Real adapters — pg, openai, redis

**Files:**
- Create: `crates/adapter_pg/src/user_repo.rs`, `crates/adapter_pg/src/usage_writer.rs`, `crates/adapter_openai/src/lib.rs`, `crates/adapter_redis/src/lib.rs`
- Modify: `crates/adapter_*/Cargo.toml` (add domain dep)

- [ ] **Step 1: pg::PgUserRepo with sqlx::query_as! macro**

```rust
// crates/adapter_pg/src/user_repo.rs
use sqlx::{PgPool, Row};
use async_trait::async_trait;
use rewrite_rs_domain::{user::{User, UserId, Plan}, ports::UserRepo, errors::Error};

pub struct PgUserRepo { pool: PgPool }
impl PgUserRepo { pub fn new(pool: PgPool) -> Self { Self { pool } } }

#[async_trait]
impl UserRepo for PgUserRepo {
    async fn find(&self, id: UserId) -> Result<User, Error> {
        let row = sqlx::query!(
            r#"SELECT u.id, u.email, u.usage_today,
                      p.id as "plan_id!", p.daily_limit as "plan_daily!"
               FROM users u
               LEFT JOIN plans p ON p.id = u.plan_id
               WHERE u.id = $1"#,
            id.0
        )
        .fetch_optional(&self.pool)
        .await
        .map_err(|e| Error::Provider(e.to_string()))?
        .ok_or(Error::UserNotFound)?;
        Ok(User {
            id, email: row.email,
            plan: Plan { id: row.plan_id, daily_limit: row.plan_daily },
            usage_today: row.usage_today,
        })
    }
    async fn increment_usage(&self, id: UserId) -> Result<(), Error> {
        sqlx::query!("UPDATE users SET usage_today = usage_today + 1 WHERE id = $1", id.0)
            .execute(&self.pool).await
            .map_err(|e| Error::Provider(e.to_string()))?;
        Ok(())
    }
}
```

`sqlx::query!` validates the SQL against the live Postgres at compile time. Mistyped column or missing JOIN = build failure.

- [ ] **Step 2: openai::OpenAi (streaming via reqwest + futures)**

```rust
// crates/adapter_openai/src/lib.rs
// POST https://api.openai.com/v1/chat/completions  with stream=true
// Parse SSE chunks, forward content tokens as a Stream
// async fn stream(...) -> Pin<Box<dyn Stream<Item=Result<String,Error>>+Send>>
```

- [ ] **Step 3: redis::RedisRateLimiter (token bucket)**

```rust
// crates/adapter_redis/src/lib.rs
// Uses redis::aio with multiplexed connection. INCR + EXPIRE per minute.
// 60/min/user (config).
```

- [ ] **Step 4: Integration tests with `testcontainers`**

```rust
// crates/adapter_pg/tests/user_repo_integration.rs
// Spins up postgres:16-alpine via testcontainers-rs, applies NestJS migrations,
// inserts a user, calls PgUserRepo::find(...), asserts the mapping.
```

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(rewrite-rs): real adapters — pg, openai (stream), redis rate limiter"
```

---

## Task 7: HTTP handler with SSE streaming

**Files:**
- Create: `crates/http/src/handler/{health,rewrite}.rs`, `crates/http/src/middleware/{logging,correlation}.rs`
- Modify: `crates/http/src/main.rs` (wire router + state)

- [ ] **Step 1: SSE response with Axum**

```rust
// crates/http/src/handler/rewrite.rs
use axum::{
    Extension, Json, extract::State,
    response::sse::{Event, KeepAlive, Sse},
};
use futures::stream::{Stream, StreamExt};
use serde::Deserialize;
use std::convert::Infallible;
use rewrite_rs_domain::{user::UserId, rewrite_request::RewriteRequest, tone::Tone};
use rewrite_rs_usecase::rewrite::{rewrite, RewriteDeps};
use crate::{auth::AuthUser, error::HttpError, AppState};

#[derive(Deserialize)]
pub struct RewriteBody {
    text: String,
    tone: String,
    #[serde(default)]
    lang: Option<String>,
}

pub async fn rewrite_handler(
    State(state): State<AppState>,
    user: AuthUser,
    Json(body): Json<RewriteBody>,
) -> Result<Sse<impl Stream<Item = Result<Event, Infallible>>>, HttpError> {
    let tone: Tone = body.tone.parse()
        .map_err(|_| HttpError::BadRequest("invalid tone".into()))?;
    let req = RewriteRequest::new(body.text, tone, body.lang)
        .map_err(|_| HttpError::BadRequest("invalid input".into()))?;
    let deps = RewriteDeps {
        users: state.user_repo.as_ref(),
        provider: state.provider.as_ref(),
        rate_limit: state.rate_limiter.as_ref(),
    };
    let inner = rewrite(deps, UserId(user.id), req).await
        .map_err(|e| match e {
            rewrite_rs_domain::errors::Error::QuotaExceeded => HttpError::TooManyRequests,
            rewrite_rs_domain::errors::Error::NoProvider => HttpError::ServiceUnavailable,
            other => HttpError::Internal(anyhow::anyhow!(other.to_string())),
        })?;
    let events = inner.map(|chunk| {
        Ok::<_, Infallible>(match chunk {
            Ok(s) => Event::default().data(s),
            Err(e) => Event::default().event("error").data(e.to_string()),
        })
    });
    Ok(Sse::new(events).keep_alive(KeepAlive::default()))
}
```

- [ ] **Step 2: Compose state + router in main.rs**

```rust
// crates/http/src/main.rs (expanded)
#[derive(Clone)]
pub struct AppState {
    pub user_repo: Arc<dyn rewrite_rs_domain::ports::UserRepo>,
    pub provider:  Arc<dyn rewrite_rs_domain::ports::AiProvider>,
    pub rate_limiter: Arc<dyn rewrite_rs_domain::ports::RateLimiter>,
    pub jwt: JwtConfig,
}
// ... build state from config, build Axum router with .with_state(state)
```

- [ ] **Step 3: End-to-end test** — `tower::ServiceExt::oneshot` against an in-memory state.

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(rewrite-rs): SSE handler + middleware stack + Axum state composition"
```

---

## Task 8: Provider failover (Anthropic + Ollama + chain)

**Files:**
- Create: `crates/adapter_anthropic/src/lib.rs`, `crates/adapter_ollama/src/lib.rs`, `crates/usecase/src/provider_chain.rs`

- [ ] **Step 1: Anthropic adapter — mirror OpenAi shape, different auth + endpoint**
- [ ] **Step 2: Ollama adapter — local/remote, streaming**
- [ ] **Step 3: `ProviderChain` — wraps `Vec<Arc<dyn AiProvider>>`, falls back on transient errors**

```rust
// crates/usecase/src/provider_chain.rs
pub struct ProviderChain { providers: Vec<Arc<dyn AiProvider>> }

#[async_trait]
impl AiProvider for ProviderChain {
    async fn stream(&self, req: &RewriteRequest) -> Result<TokenStream, Error> {
        let mut last = None;
        for p in &self.providers {
            match p.stream(req).await {
                Ok(stream) => return Ok(stream),
                Err(e) if is_transient(&e) => { last = Some(e); continue; }
                Err(e) => return Err(e),
            }
        }
        Err(last.unwrap_or(Error::NoProvider))
    }
    fn name(&self) -> &'static str { "chain" }
}
```

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(rewrite-rs): multi-provider failover chain"
```

---

## Task 9: Observability (tracing + Prometheus + tower-http)

**Files:**
- Modify: `crates/http/Cargo.toml` (add tower-http, metrics-exporter-prometheus), `crates/http/src/main.rs`, `crates/http/src/middleware/`

- [ ] **Step 1: tower-http `TraceLayer` for HTTP access logs (JSON)**
- [ ] **Step 2: `metrics::counter!` + `metrics::histogram!` per request**
- [ ] **Step 3: `/metrics` endpoint via `metrics-exporter-prometheus`**
- [ ] **Step 4: `tracing` spans propagated through use case via `#[instrument]`**

```bash
git commit -m "feat(rewrite-rs): tracing + Prometheus metrics + tower-http access logs"
```

---

## Task 10: Caddy side-by-side + production deploy

**Files:**
- Modify: production `Caddyfile`, `docker-compose.prod.yml`

- [ ] **Step 1: Caddy route `/rewrite-rs` → Rust service**
- [ ] **Step 2: docker-compose.prod.yml adds `rewrite-rs` service**
- [ ] **Step 3: Smoke test on prod via real JWT**

```bash
git commit -m "deploy(rewrite-rs): Caddy /rewrite-rs route + production compose service"
```

---

## Task 11: Client opt-in flag (mobile A/B)

**Files:**
- Modify: `DraftRightMobile/lib/services/backend_client.dart`, hidden setting

- [ ] **Step 1: Add a hidden settings toggle "Use Rust /rewrite (beta)"**
- [ ] **Step 2: BackendClient.rewrite() hits `/rewrite-rs` when enabled**
- [ ] **Step 3: A/B on yourself for 2+ weeks; compare latency p50/p99, error rate vs NestJS (and vs Go if running both)**

```bash
git commit -m "feat(mobile): opt-in flag for Rust /rewrite service"
```

---

## Task 12: Full cutover (when proven)

- [ ] **Step 1: Caddy `/rewrite` → Rust service**
- [ ] **Step 2: Soak 1 week**
- [ ] **Step 3: Remove NestJS RewriteController**

```bash
git commit -m "deploy(rewrite-rs): cutover — Rust service owns /rewrite"
```

---

## Task 13 (future)

- Refund usage on AI provider failure (wrap `increment_usage` in saga-pattern compensation)
- RS256 JWT (NestJS signs with private key, Rust verifies with public key — no shared secret to rotate)
- gRPC interface via `tonic` for high-frequency clients
- Read replica via `sqlx::PgPool` with read/write split
- Benchmarks (`criterion`) — record p99 latency before/after each optimization
- Switch from `async-trait` to native `async fn in trait` once `Send` ergonomics fully stabilize

---

## Quick-reference: weekend pacing (calibrated for Rust learning curve)

| Weekend | Tasks | Hours | Outcome |
|---|---|---|---|
| 1 | Task 1 | 6-10 | Workspace builds, hello world via Docker. Fight cargo workspace + mold linker setup; emerge wiser. |
| 2 | Task 2 | 6-10 | JWT extractor works. First real wrestle with `FromRequestParts` + `Send + Sync` bounds. |
| 3 | Task 3 | 8-12 | sqlx connects, `.sqlx/` committed. Schema-aware compile errors feel magical. |
| 4 | Task 4 | 6-8 | Domain crate green. Most of the time spent on `async-trait` + `Send` bounds. |
| 5 | Task 5 | 6-8 | Use case compiles + tests pass. Generic-over-traits feels powerful. |
| 6-7 | Task 6 | 12-16 | Streaming adapter is the hardest week. Pin, Send, lifetimes all bite. |
| 8 | Task 7 | 8-10 | Axum SSE returns real tokens to curl. |
| 9 | Task 8 | 6-8 | Failover green. |
| 10 | Task 9 | 4-6 | `/metrics` works. |
| 11 | Task 10 | 4-8 | Prod side-by-side live. |
| 12 | Task 11 | 4 | Feature flag in mobile. |
| 13-15 | Soak | — | A/B observation. |
| 16 | Task 12 | 4 | Cutover. |

**Total: ~80-130 hours.** ~30-50% longer than Go. The extra time is mostly Tasks 1, 4, 6 — workspace ceremony, async traits, streaming through generics. Once those land, the rest moves faster than Go because the compiler holds your hand.

---

## Self-Review

- **Boundary discipline**: Cargo workspace + per-crate `Cargo.toml` allowlists make "domain depends on adapter" a literal build-time impossibility. Better guarantee than Go.
- **Schema ownership**: NestJS owns migrations. sqlx's compile-time check (with `.sqlx/` metadata committed) makes drift a build-time failure rather than a runtime surprise.
- **JWT sync**: HS256 shared secret today. RS256 upgrade path is Task 13.
- **Production safety**: parallel deploy → opt-in flag → 2-week soak → cutover. Same shape as the Go plan.
- **Test coverage**: domain 100% (pure logic); usecase 100% with in-memory fakes; adapter integration tests via `testcontainers`.
- **No `.unwrap()` in src/**: enforced by `#![forbid(clippy::unwrap_used)]` at each crate's lib.rs.
- **Cancellation safety**: every stream + DB call passes `&self` (not owned), Tokio's drop-on-cancel handles cleanup. No leaked tasks.
- **Operational complexity**: same as Go — one more Docker image, one more service, one more metrics endpoint.

---

## Honest comparison vs the Go plan

| | Go plan | Rust plan |
|---|---|---|
| Total weekend hours | 60-90 | 80-130 |
| First "hello world" | weekend 1 | weekend 1 (but harder) |
| Cognitive load weeks 1-4 | medium | high |
| Cognitive load months 3+ | low | medium (rewards study) |
| Compile-time architectural guarantees | linter + discipline | Cargo workspace + sqlx + ownership |
| Runtime safety guarantees | runtime panic possible (nil deref) | impossible (Option/Result) |
| Concurrency bug surface | `-race` catches at test time | borrow checker catches at build time |
| Ecosystem maturity for AI providers | OpenAI/Anthropic SDKs first-class | hand-rolled reqwest streaming (less ergonomic) |
| Final binary size | 15 MB | 25 MB |
| Idle RAM | 20 MB | 5 MB |
| Throughput on /rewrite (OpenAI-bound) | indistinguishable from Rust | indistinguishable from Go |
| Hiring market 2026 | 4x bigger than Rust | catching up but smaller |
| **Best fit when** | Time matters, want to ship soon | Learning depth + safety matter more than time |

**Recommendation order if budget allows both:**
1. Build the **Go service first**. You'll absorb microservice patterns + boundary discipline without fighting the language.
2. After it's running, **port the same architecture to Rust** as a language-learning exercise. You already know the boundaries; now you're just learning Rust through familiar problems.
3. Pick whichever you enjoy maintaining 6 months later for the long-term home of `/rewrite`.

---

## When to abandon this plan

1. **By Task 4**, if `async-trait` bounds keep you stuck — pause for a week, read the Tokio book, return.
2. **By Task 6**, if streaming through `Pin<Box<dyn Stream + Send>>` feels impossible — this is the wall most people hit. It IS solvable but ask for help (Rust users forum, Tokio Discord).
3. **By Task 7**, if Rust /rewrite isn't materially faster than the Go service — that's fine, the value was learning. Keep one service in prod, retire the other.

---

## Decision log (fill in as you build)

| Date | Decision | Why |
|---|---|---|
| 2026-05-29 | Plan written; started none of it | Studying microservices + Rust simultaneously, not in a rush |
| | | |
