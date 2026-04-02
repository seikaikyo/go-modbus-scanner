# Go Modbus Scanner

Modbus TCP / RTU register scanner with embedded web UI. Discovers register maps when PLC vendors don't provide point tables.

Single binary, no dependencies. Plug into factory network or connect a USB-to-RS485 adapter and scan.

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

## Connection Modes

### Modbus TCP (Ethernet)

For PLCs, VFDs, power meters and other devices with RJ45 Ethernet ports.

```
Your laptop ── RJ45 ── Factory switch ── PLC (192.168.x.x:502)
```

Connect your computer to the same network as the device, enter the IP in the web UI, and scan.

### Modbus RTU (Serial / RS-485)

For devices with RS-485 terminal blocks (A+/B-/GND), such as Delta Unolite, Schneider PM series, etc.

```
Your laptop ── USB-to-RS485 adapter ── A+/B-/GND terminal
```

1. Plug in the USB-to-RS485 adapter (CH340, FTDI, etc.)
2. Switch to **Modbus RTU** mode in the web UI
3. Select the serial port (auto-detected) or type the path manually
4. Set baud rate (default 9600) and parity (default None)
5. Scan

The scanner auto-detects serial ports:
- macOS: `/dev/cu.usbserial-*`, `/dev/cu.wchusbserial-*`
- Linux: `/dev/ttyUSB*`, `/dev/ttyACM*`

## Raspberry Pi Deployment

```bash
# Cross-compile on Mac/Linux
GOOS=linux GOARCH=arm64 go build -o modbus-scanner-arm64 ./cmd/scanner/

# Copy to Pi
scp modbus-scanner-arm64 pi@192.168.1.xxx:~/

# Run on Pi, open browser from laptop
http://pi-ip:8080
```

Works for both TCP and RTU on Pi. For RTU, plug the USB-to-RS485 adapter into the Pi's USB port.

## Features

- **Dual mode**: Modbus TCP (Ethernet) and Modbus RTU (RS-485 serial)
- Batch register scanning (holding / input / coil / discrete)
- Multi-sample dynamic value detection
- Float32 pair detection (consecutive registers)
- Type inference (uint16 / int16 / float32 / bool)
- Category guessing (temperature / pressure / counter / config)
- Serial port auto-detection
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
| `/api/serial/ports` | GET | List available serial ports |

## Scan Request

### TCP mode

```json
{
  "mode": "tcp",
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

### RTU mode

```json
{
  "mode": "rtu",
  "serial_port": "/dev/cu.usbserial-110",
  "baud_rate": 9600,
  "data_bits": 8,
  "stop_bits": 1,
  "parity": "N",
  "unit_id": 1,
  "scan_types": ["holding"],
  "address_start": 0,
  "address_end": 999
}
```

## Architecture

```
go-modbus-scanner/
├── cmd/scanner/main.go         # Entry point (chi router)
├── internal/scanner/
│   ├── scanner.go              # Core scan logic (TCP + RTU)
│   ├── analyzer.go             # Type inference + guessing
│   ├── router.go               # REST API + serial port detection
│   ├── embed.go                # //go:embed React build
│   ├── response.go             # JSON response helpers
│   └── web/                    # Embedded React build
└── web/scanner-ui/             # React source (dev only)
```

## Tech Stack

| Component | Choice |
|-----------|--------|
| Backend | Go + chi |
| Modbus | goburrow/modbus (TCP + RTU) |
| Serial | goburrow/serial |
| Frontend | React + Tailwind + shadcn/ui |
| Embedding | Go `embed` package |

## License

MIT
