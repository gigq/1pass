# Repository Guidelines

## Project Structure & Module Organization
This repository is a multi-module Go project organized around a hexagonal architecture:

- `1pass-core/` — core domain and services (no external dependencies).
- `1pass-parse/` — OPVault parsing logic.
- `1pass-up/` — update mechanism.
- `1pass-term/` — CLI/TUI interaction layer.
- `1pass-app/` — main application wiring, CLI/GUI entrypoints.
- `assets/` — demo OPVault data, gifs, and checksum fixtures.
- Build outputs: `bin/` (local builds) and `out/` (release artifacts).

## Build, Test, and Development Commands
Key commands live in the top-level `Makefile`:

- `make build` — runs lightweight tests and builds `bin/1pass`.
- `make run` — executes the locally built binary.
- `make test` — runs verbose tests for all modules.
- `make test-core` / `make test-parse` / `make test-term` / `make test-app` — module-specific tests.
- `make release` — builds the Linux amd64 release archive in `out/`.
- `make clean` — removes `bin/` and `out/`.

Example: `cd 1pass-core && go test ./...`

## Coding Style & Naming Conventions
- Go version is `1.15` (see module `go.mod` files).
- Use `gofmt`-formatted code (tabs for indentation).
- Package names are lowercase; exported identifiers use `CamelCase`.
- Test files use `*_test.go` and `TestXxx` naming.
- Keep `1pass-core` dependency-free; wire integrations in `1pass-app`.

## Testing Guidelines
- Tests use the standard Go toolchain (`go test`).
- Unit tests live within each module plus `1pass-app/test/...` for app-level coverage.
- Add or update tests alongside behavior changes; no explicit coverage threshold is enforced.

## Commit & Pull Request Guidelines
Commit history follows Conventional Commits, often with issue references:

- Format: `type(#issue): summary` (e.g., `fix(#44): notes padding CLI item details`).
- Occasional release/build commits use `build:` or `docs:` without issue numbers.

PR workflow:
- Open an issue first using the **Bug** or **Request** template and include the `pr` label.
- Branch naming: `<latest_release>/pr/<short_issue_title_with_underscores>/<issue_number>`
  (e.g., `1.0.0/pr/pretty_item_overview/#99`).
- PRs target `develop` and should include test notes and any relevant screenshots.

## Security & Configuration Notes
OPVault data is sensitive. Do not commit real vaults or secrets. Use `assets/onepassword_data/default` as fixtures and keep local paths in user configuration or CLI flags (e.g., `-v`).
