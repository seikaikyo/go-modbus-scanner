# Go Modbus Scanner

Modbus TCP register scanner with embedded web UI. Discovers register maps
when PLC vendors don't provide point tables.

AI-assisted development with Claude Code.

## Usage

```bash
# Build
go build -o modbus-scanner ./cmd/scanner/

# Run (default port 8080)
./modbus-scanner

# Custom port
PORT=9090 ./modbus-scanner

# Open browser
http://localhost:8080
```

## Raspberry Pi Deployment

```bash
# Cross-compile on Mac/Linux
GOOS=linux GOARCH=arm64 go build -o modbus-scanner-arm64 ./cmd/scanner/

# Copy to Pi
scp modbus-scanner-arm64 pi@192.168.1.xxx:~/

# Run on Pi, open browser from laptop
http://pi-ip:8080
```

Single binary, no dependencies. Plug into factory network and scan.

## Features

- Batch register scanning (holding / input / coil / discrete)
- Multi-sample dynamic value detection
- Float32 pair detection (consecutive registers)
- Type inference (uint16 / int16 / float32 / bool)
- Category guessing (temperature / pressure / counter / config)
- CSV and JSON export
- Dark-themed technical dashboard (React + Tailwind + shadcn/ui)
- Read-only operations (FC01-04), safe for production PLCs

## API Endpoints

| Path | Method | Description |
|------|--------|-------------|
| `/` | GET | Web UI |
| `/health` | GET | Health check |
| `/api/scan` | POST | Full scan (async) |
| `/api/scan/quick` | POST | Quick scan (holding 0-999) |
| `/api/read` | POST | Read specific registers |
| `/api/jobs` | GET | List scan jobs |
| `/api/jobs/{id}` | GET | Get scan result |

## Scan Request

```json
{
  "host": "192.168.1.200",
  "port": 502,
  "unit_id": 1,
  "scan_types": ["holding", "input"],
  "address_start": 0,
  "address_end": 9999,
  "samples": 5,
  "sample_interval_ms": 1000
}
```

## Architecture

```
go-modbus-scanner/
├── cmd/scanner/main.go         # Entry point (chi router)
├── internal/scanner/
│   ├── scanner.go              # Core scan logic
│   ├── analyzer.go             # Type inference + guessing
│   ├── router.go               # REST API
│   ├── embed.go                # //go:embed React build
│   ├── response.go             # JSON response helpers
│   └── web/                    # Embedded React build
└── web/scanner-ui/             # React source (dev only)
```

## Tech Stack

| Component | Choice |
|-----------|--------|
| Backend | Go + chi |
| Modbus | goburrow/modbus |
| Frontend | React + Tailwind + shadcn/ui |
| Embedding | Go `embed` package |
