# StreaminTX - Renamed from MediaMTX

## Project Renaming Summary

This project was renamed from "mediamtx" to "streamintx". Below is a record of all changes made during the renaming session.

## Files Modified

### Build Configuration
- `scripts/binaries.mk`
  - Changed `BINARY_NAME = mediamtx` → `BINARY_NAME = streamintx`
  - Changed all `mediamtx.yml` references → `streamintx.yml`

### Configuration Files
- Renamed `mediamtx.yml` → `streamintx.yml`

### Core Application
- `internal/core/core.go`
  - Default config paths: `streamintx.yml`
  - System paths: `/usr/local/etc/streamintx.yml`, `/usr/etc/streamintx.yml`, `/etc/streamintx/streamintx.yml`
  - Help text updated to reference `streamintx.yml`

- `internal/conf/conf.go`
  - Log file: `streamintx.log`
  - Syslog prefix: `streamintx`
  - JWT claim key: `streamintx_permissions`

- `internal/core/upgrade.go`
  - Git repo URL: `https://github.com/bluenviron/streamintx`
  - Download URL pattern updated for `streamintx_*` archives
  - Executable name: `streamintx`
  - Success message: "StreaminTX upgraded successfully..."

- `internal/core/versiongetter/main.go`
  - Log message: "getting streamintx version..."

### Test Files
- `internal/conf/conf_test.go`
  - Updated references from `../../mediamtx.yml` → `../../streamintx.yml`

### Docker Files
- `docker/standard.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

- `docker/ffmpeg.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

- `docker/rpi.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

- `docker/ffmpeg-rpi.Dockerfile`
  - Archive names: `streamintx_*_linux_*.tar.gz`
  - Entrypoint: `/streamintx`

### Generated Files
- `internal/core/VERSION` - Created with `v0.0.1`

## Build Instructions

Before building, you need to generate embedded files:

```bash
# Option 1: Run go generate (downloads hls.js, creates VERSION from git)
go generate ./...

# Option 2: Manual (if not a git repo)
# VERSION file already created at internal/core/VERSION
cd internal/servers/hls && go run ./hlsjsdownloader && cd ../../..

# Build the binary
go build -o streamintx .
```

## Remaining Work (Optional)

The following items were NOT changed and may need updating if you want a complete rebrand:

1. **Go module path** (`go.mod`): Still `github.com/bluenviron/mediamtx`
   - Would require updating all import statements across ~225 files

2. **Internal package imports**: All files still import from `github.com/bluenviron/mediamtx/internal/...`

3. **Auth realm** (`internal/playback/server.go`): `Basic realm="mediamtx"`

4. **Temp directory prefixes** in tests: `mediamtx-playback`, `mediamtx-playback-fmp4-`

5. **Docker repository** (`scripts/dockerhub.mk`): `bluenviron/mediamtx`

6. **Documentation**: README.md and docs/ folder references

7. **GitHub workflow files**: `.github/workflows/`

## Notes

- The upgrade feature (`--upgrade` flag) now points to `github.com/bluenviron/streamintx` - update this URL when you create your own repository
- The binary will be named `streamintx` when compiled
- Config file should be named `streamintx.yml`
- Log file defaults to `streamintx.log`
