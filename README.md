# ProxiLens

**Bluetooth proximity diagnostics for Ubuntu 24.04 LTS / GNOME**, written in Go.

> Current status: Phase 1 (BLE discovery and diagnostics). Automatic locking and unlocking are not implemented yet.

## Features in this phase

- Discover Bluetooth Low Energy devices through BlueZ/D-Bus.
- Display discovered devices and signal strength (RSSI) in a Fyne desktop dashboard.
- Provide a command-line scanner for troubleshooting.
- GitHub Actions CI checks formatting, tests, vet, and builds both applications.

## Requirements

- Ubuntu 24.04 LTS (GNOME)
- Go 1.22 or newer
- BlueZ
- Fyne build dependencies

Install system packages:

```bash
sudo apt update
sudo apt install -y golang-go bluez libgl1-mesa-dev xorg-dev
sudo systemctl enable --now bluetooth
```

## Run from source

```bash
git clone https://github.com/OWNER/REPOSITORY.git
cd proxilens
go mod tidy
go run ./cmd/proxilens-ui
```

CLI scanner:

```bash
go run ./cmd/proxilens-scan
```

## GitHub repository setup

1. Create a new **public or private repository** on GitHub named `proxilens`.
2. Replace `OWNER/REPOSITORY` in this README and `.github/ISSUE_TEMPLATE/config.yml` with your GitHub owner and repository.
3. Set the Go module path to your real repository path. For example, if your GitHub account is `my-account`, change `module github.com/local/proxilens` in `go.mod` to `module github.com/my-account/proxilens`, then replace all imports beginning with `github.com/local/proxilens/` with `github.com/my-account/proxilens/`.
4. Run `go mod tidy`, then commit **both** `go.mod` and `go.sum`.
5. Push the project to the `main` branch.

Example commands after creating the empty GitHub repository (replace the URL):

```bash
git init
git add .
git commit -m "chore: initial ProxiLens project"
git branch -M main
git remote add origin https://github.com/OWNER/REPOSITORY.git
git push -u origin main
```

The workflow in `.github/workflows/ci.yml` runs automatically on pushes to `main` and pull requests.

## Creating a release

Create a version tag such as `v0.1.0`, then create and publish a GitHub Release for that tag. The release workflow builds Linux amd64 binaries for the GUI and CLI, packages them into a `.tar.gz`, generates a SHA-256 checksum, and attaches both files to the GitHub Release.

## Development commands

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/proxilens-scan
go build ./cmd/proxilens-ui
```

## Current limitations and security

The iPhone and Apple Watch detection behavior must be validated on the target hardware. Apple devices may use private or rotating BLE addresses and do not necessarily advertise a stable, continuously identifiable signal.

RSSI is only a proximity heuristic, not proof of identity. A nearby BLE advertisement must never be sufficient to authenticate or unlock a user session. A future implementation should preserve normal GNOME/GDM authentication and use a cryptographically verifiable mechanism for any unlock-related feature.

## License

No license has been selected yet. Choose and add a license before publishing the repository if you want to grant others explicit reuse rights.
