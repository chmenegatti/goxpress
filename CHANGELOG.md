# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Typed/constrained route parameters (#8): a `:param` segment may carry a
  `|matcher` constraint (`/users/:id|int`); a value the matcher rejects falls
  through to 404. Built-in matchers `int`, `uint`, `alpha`, `alnum`, `slug`,
  `uuid`, plus custom matchers via `Router.Param(name, func(string) bool)`. The
  check runs only on constrained segments, so unconstrained routes keep their
  zero-allocation hot path.

## [0.4.0] - 2026-06-19

### Added

- Automatic OpenAPI 3.1 generation (#22): route registration now returns a
  `*Route` with a fluent metadata API (`Summary`, `Description`, `Tags`,
  `Body`, `Query`, `Produces`, `Response`, `Hide`) plus generic `Body[T]`,
  `Query[T]` and `Produces[T]` helpers. `app.OpenAPI()` serves the generated
  document at `GET /openapi.json`. Path params, query params and struct schemas
  (reused via `components.schemas` `$ref`) are derived by reflection at setup
  time only — never per request. Core stays dependency-free; Swagger UI is
  deferred to a separate opt-in package.

## [0.3.0] - 2026-06-18

### Added

- Generics-based binding API (#19): package-level `Bind[T]`, `BindJSON[T]`,
  `BindQuery[T]` and `BindForm[T]` return the decoded value directly. They are a
  thin convenience layer over the existing pointer-based `Context` methods — no
  new binding logic, no breaking changes, stdlib only.
- Comparative router benchmarks vs gin, chi and echo (#10), living in a separate
  `benchmarks/` module so competitor dependencies stay out of the core. Run with
  `make bench-compare`; results table published in the README.
- Router path-matcher fuzz test (`FuzzRouterMatch`) (#9).

### Changed

- Raised test coverage above 90% across the library packages (#9), adding tests
  for Context helpers, binding, groups, `Mount`, `FromStd`, static file serving,
  the WebSocket control-frame paths, and the middleware default constructors.
  `make cover` now reports on library packages only (the runnable examples carry
  no tests).

## [0.2.0] - 2026-06-18

### Added

- Streaming responses (#3): `Context.SSEvent(event, data)` writes a Server-Sent
  Event with the correct `text/event-stream` headers and flushes it immediately
  (multi-line data is split into per-line `data:` fields); `Context.Flush`
  exposes manual flushing. A new dependency-free `ws` subpackage adds RFC 6455
  WebSocket support: `ws.Upgrade(c)` returns a `Conn` with `ReadMessage` /
  `WriteMessage` / `WriteText`, reassembling fragments and answering pings.
  Includes an `examples/streaming` program.
- HTML template rendering (#1): `Context.HTML(code, name, data)` renders a named
  template through a pluggable `Renderer` set on `Router.Renderer`. A
  `TemplateRenderer` adapts `html/template` out of the box, keeping the core
  free of third-party dependencies. Rendering is buffered, so a template error
  surfaces to the error handler without a partial response.
- Additional standard-library-only middleware (#6): `BasicAuth` (HTTP Basic
  authentication), `SecureHeaders` (X-Content-Type-Options, X-Frame-Options,
  CSP, Referrer-Policy, HSTS over TLS), `RateLimit` (per-IP token bucket),
  `BodyLimit` (max request body size), and `Decompress` (gzip request body
  decompression, the request-side counterpart to `Compress`).

## [0.1.0] - 2026-06-14

Initial public preview.

### Added

- Priority-ordered radix-tree routing engine with static, `:param` and
  `*catch-all` segments; `net/http`-compatible `Router` with a pooled `Context`
  (0 allocations on the routing hot path).
- HTTP verb helpers (`Get`/`Post`/`Put`/`Patch`/`Delete`/`Head`/`Options`),
  404/405 handling with an `Allow` header, and automatic trailing-slash
  redirects.
- Rich request `Context`: URL params, query/form accessors, headers, cookies, a
  request-scoped value store and `JSON`/`XML`/`String`/`Blob`/`NoContent`/
  `Redirect` response helpers backed by a status/size-tracking `ResponseWriter`.
- Onion-model middleware chain (`Next`/`Abort`/`IsAborted`), global `Use`, and
  `net/http` interop via `WrapH`, `WrapF` and `FromStd`.
- Route groups with shared prefixes and middleware (nestable) and chi-style
  `Mount` for sub-routers.
- Centralized error handling: `HTTPError`, `PanicError`, a customizable
  `ErrorHandler`, and built-in panic recovery.
- Request binding (`Bind`/`BindJSON`/`BindQuery`/`BindForm`) using standard-
  library JSON and reflection over `query`/`form` struct tags.
- `middleware/` package (standard-library only): `RequestID`, `RealIP`,
  `Logger`, `Recoverer`, `CORS`, `Timeout` and `Compress`.
- Project tooling: MIT license, golangci-lint configuration, Makefile, GitHub
  Actions CI, examples and package documentation.

[Unreleased]: https://github.com/chmenegatti/goxpress/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/chmenegatti/goxpress/releases/tag/v0.1.0
