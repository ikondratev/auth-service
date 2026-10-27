# Changelog

## [0.1.0] - 2026-10-27

### Added

- gRPC server. Settings are loaded from `settings/{env}.settings.json`, logs go to a JSON `slog` logger, and the process shuts down on `SIGINT` and `SIGTERM` within `waiting_shutdown_sec`.
- gRPC health on the same port: `grpc.health.v1.Health/Check`. The status is `SERVING` after listen and `NOT_SERVING` before shutdown, so Kubernetes can use this port for readiness and liveness.
- Pull request checks: `golangci-lint` and `go test ./...` (`.github/workflows/ci.yml`, `.golangci.yml`).

### Notes

- `AuthService` methods are not part of this version. Register, login, refresh, and logout are not served yet.
- `deploy/Dockerfile` is present. Image publish is not set up.

[0.1.0]: https://github.com/ikondratev/auth-service/releases/tag/v0.1.0
