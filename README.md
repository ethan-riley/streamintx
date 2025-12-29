# StreaminTX

A ready-to-use, zero-dependency real-time media server and media proxy that allows publishing, reading, proxying, recording, and playback of video and audio streams. StreaminTX acts as a "media router" that routes media streams from one end to the other.

## Features

### Core Streaming
- **Publish** live streams via SRT, WebRTC, RTSP, RTMP, HLS, MPEG-TS, RTP
- **Read** live streams via SRT, WebRTC, RTSP, RTMP, HLS
- **Automatic protocol conversion** between supported formats
- **Multi-path streaming** - serve multiple streams simultaneously on separate paths
- **Hot reload** - update configuration without disconnecting clients

### Recording & Playback
- **Record** streams to disk in fMP4 or MPEG-TS format
- **Playback** recorded streams on demand

### Security & Control
- **Authentication** via internal users, HTTP backend, or JWT tokens
- **Control API** for server management and monitoring
- **Prometheus metrics** for observability

### Advanced Features
- **Forward** streams to other servers
- **Proxy** requests to upstream servers
- **Hooks** - execute external commands on client events (connect, disconnect, publish, read)
- **Staging Database** - SQLite-based logging of all stream activity (last 72 hours)

### Platform Support
- Linux, Windows, macOS
- Single executable, no dependencies

## Quick Start

### Build from Source

```bash
# Generate required files
go generate ./...

# Build
go build -o streamintx .

# Run
./streamintx
```

### Configuration

Create a `streamintx.yml` configuration file (see `streamintx.yml` for all options):

```yaml
# Basic configuration
logLevel: info
api: yes
apiAddress: :9997

# Enable staging database for stream logging
stagingDB: yes
stagingDBPath: streamintx_staging.db
stagingDBRetentionPeriod: 72h

# RTMP server
rtmp: yes
rtmpAddress: :1935

# Path configuration
paths:
  all:
```

### Publish a Stream

```bash
# Using FFmpeg via RTMP
ffmpeg -re -i input.mp4 -c copy -f flv rtmp://localhost/mystream

# Using FFmpeg via RTSP
ffmpeg -re -i input.mp4 -c copy -f rtsp rtsp://localhost:8554/mystream
```

### Read a Stream

```bash
# Via RTMP
ffplay rtmp://localhost/mystream

# Via RTSP
ffplay rtsp://localhost:8554/mystream

# Via HLS (browser)
# Open http://localhost:8888/mystream
```

## API Reference

StreaminTX provides a comprehensive REST API for server control and monitoring.

### Server Info

```bash
# Get server info
curl http://localhost:9997/v3/config/global/get
```

### Paths Management

```bash
# List all paths
curl http://localhost:9997/v3/paths/list

# Get specific path
curl http://localhost:9997/v3/paths/get/mystream
```

### Connections Management

```bash
# List RTMP connections
curl http://localhost:9997/v3/rtmpconns/list

# List RTSP sessions
curl http://localhost:9997/v3/rtspsessions/list

# List WebRTC sessions
curl http://localhost:9997/v3/webrtcsessions/list

# List SRT connections
curl http://localhost:9997/v3/srtconns/list
```

### Staging Database API

The staging database tracks all stream activity for the configured retention period.

```bash
# List recent paths
curl http://localhost:9997/v3/staging/paths/list
curl "http://localhost:9997/v3/staging/paths/list?hours=24"

# Get specific path history
curl http://localhost:9997/v3/staging/paths/get/mystream

# List recent connections
curl http://localhost:9997/v3/staging/connections/list
curl "http://localhost:9997/v3/staging/connections/list?path=mystream"

# Get connections for a path
curl http://localhost:9997/v3/staging/connections/get/mystream

# Get staging database statistics
curl http://localhost:9997/v3/staging/stats
```

#### Staging Database Response Examples

**Path Record:**
```json
{
  "id": 1,
  "name": "mystream",
  "confName": "all",
  "sourceType": "rtmpConn",
  "sourceId": "uuid-here",
  "ready": false,
  "readyTime": "2025-12-29T14:55:49.156Z",
  "closedTime": "2025-12-29T15:30:22.123Z",
  "state": "published",
  "tracks": ["H264", "MPEG-4 Audio"],
  "bytesReceived": 331969114,
  "bytesSent": 16259752,
  "createdAt": "2025-12-29T14:55:49Z",
  "updatedAt": "2025-12-29T15:30:22Z"
}
```

**Connection Record:**
```json
{
  "id": 1,
  "connId": "uuid-here",
  "connType": "rtmp",
  "created": "2025-12-29T17:12:58.758Z",
  "closedTime": "2025-12-29T17:15:30.123Z",
  "remoteAddr": "192.168.1.100:58703",
  "state": "published",
  "path": "mystream",
  "query": "user=myuser&pass=xxx",
  "user": "myuser",
  "bytesReceived": 17662199,
  "bytesSent": 3467,
  "createdAt": "2025-12-29T20:12:58Z",
  "updatedAt": "2025-12-29T20:15:30Z"
}
```

#### State Values

| State | Description |
|-------|-------------|
| `idle` | Connection opened but not yet publishing/reading |
| `streaming` | Actively transmitting data |
| `published` | Completed normally by client |
| `timeout` | Connection lost unexpectedly |

### Recording Management

```bash
# List recordings
curl http://localhost:9997/v3/recordings/list

# Get specific recording
curl http://localhost:9997/v3/recordings/get/mystream

# Delete segment
curl -X DELETE http://localhost:9997/v3/recordings/deletesegment?path=mystream&start=2025-01-01T00:00:00Z
```

### HLS Muxers

```bash
# List HLS muxers
curl http://localhost:9997/v3/hlsmuxers/list

# Get specific HLS muxer
curl http://localhost:9997/v3/hlsmuxers/get/mystream
```

### Metrics

```bash
# Prometheus-compatible metrics
curl http://localhost:9997/metrics
```

## Configuration Reference

See [`streamintx.yml`](streamintx.yml) for the complete configuration reference with all available options and their defaults.

### Key Configuration Sections

- **General** - Logging, timeouts, buffer sizes
- **API** - Control API settings
- **Playback** - Playback server settings
- **RTSP/RTSPS** - RTSP server configuration
- **RTMP/RTMPS** - RTMP server configuration
- **HLS** - HLS server configuration
- **WebRTC** - WebRTC server configuration
- **SRT** - SRT server configuration
- **Recording** - Recording settings
- **Staging Database** - Stream activity logging
- **Paths** - Per-path configuration

## License

StreaminTX is licensed under the MIT License.

This project is based on [MediaMTX](https://github.com/bluenviron/mediamtx) by bluenviron, also licensed under MIT.

```
MIT License

Copyright (c) 2019 aler9 (original MediaMTX)
Copyright (c) 2025 TechSphere (StreaminTX modifications)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
