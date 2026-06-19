# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
