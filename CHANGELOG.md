# Changelog

## [0.3.0] - 2026-11-04
### Added
- The access token is an RS256 JWT signed with the service private key. Claims are `sub`, `email`, `scope`, `iss`, `aud`, and `exp`.
- The refresh token is an opaque value. The client receives the raw token. The database stores its SHA-256 hex digest, a family id, and an expiry taken from `refresh_ttl_sec`.
- The user and the refresh row are inserted in one transaction. Postgres tables are `users` and `refresh_tokens` (`migrations/000001_create_users.up.sql`).
- Repositories live in `internal/repo/postgres`. The service opens the transaction through `storage.Transactor`.

### Notes

- `Login`, `Refresh`, and `Logout` still return `Unimplemented`.

[0.3.0]: https://github.com/ikondratev/auth-service/releases/tag/v0.3.0

## [0.2.0] - 2026-10-28

### Added

- gRPC `AuthService` in `api/auth/v1/auth.proto`: `Register`, `Login`, `Refresh`, and `Logout`. `Register`, `Login`, and `Refresh` return a `TokenPair`.
- The service is registered on the existing gRPC server. All four methods respond with `Unimplemented`.
- Publish workflow (`.github/workflows/publish.yml`) on a `v*` tag. The tag must match the `VERSION` file. The image name is the `DOCKERHUB_IMAGE` repository variable; Docker Hub login uses the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets.

### Notes

- `Register` does not create a user or issue tokens yet.
- `deploy/Dockerfile` still starts `/app/http`. The built binary is `/app/auth`.

[0.2.0]: https://github.com/ikondratev/auth-service/releases/tag/v0.2.0

## [0.1.0] - 2026-10-27

### Added

- gRPC server. Settings are loaded from `settings/{env}.settings.json`, logs go to a JSON `slog` logger, and the process shuts down on `SIGINT` and `SIGTERM` within `waiting_shutdown_sec`.
- gRPC health on the same port: `grpc.health.v1.Health/Check`. The status is `SERVING` after listen and `NOT_SERVING` before shutdown, so Kubernetes can use this port for readiness and liveness.
- Pull request checks: `golangci-lint` and `go test ./...` (`.github/workflows/ci.yml`, `.golangci.yml`).

### Notes

- `AuthService` methods are not part of this version. Register, login, refresh, and logout are not served yet.
- `deploy/Dockerfile` is present. Image publish is not set up.

[0.1.0]: https://github.com/ikondratev/auth-service/releases/tag/v0.1.0
