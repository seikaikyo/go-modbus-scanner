package scanner

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/goburrow/modbus"
)

// ScanRequest is the input for a scan job.
type ScanRequest struct {
	Host             string   `json:"host"`
	Port             int      `json:"port"`
	UnitID           uint8    `json:"unit_id"`
	ScanTypes        []string `json:"scan_types"`         // holding, input, coil, discrete
	AddressStart     uint16   `json:"address_start"`
	AddressEnd       uint16   `json:"address_end"`
	BatchSize        uint16   `json:"batch_size"`
	Samples          int      `json:"samples"`            // multi-sample count
	SampleIntervalMs int      `json:"sample_interval_ms"`
	TimeoutMs        int      `json:"timeout_ms"`
	DelayMs          int      `json:"delay_ms"`           // delay between batches
}

func (r *ScanRequest) applyDefaults() {
	if r.Port == 0 {
		r.Port = 502
	}
	if r.UnitID == 0 {
		r.UnitID = 1
	}
	if len(r.ScanTypes) == 0 {
		r.ScanTypes = []string{"holding"}
	}
	if r.AddressEnd == 0 {
		r.AddressEnd = 9999
	}
	if r.BatchSize == 0 {
		r.BatchSize = 125
	}
	if r.Samples == 0 {
		r.Samples = 3
	}
	if r.SampleIntervalMs == 0 {
		r.SampleIntervalMs = 1000
	}
	if r.TimeoutMs == 0 {
		r.TimeoutMs = 500
	}
	if r.DelayMs == 0 {
		r.DelayMs = 10
	}
}

// RawRegister holds the raw scan result for one address.
type RawRegister struct {
	Address   uint16   `json:"address"`
	Type      string   `json:"type"` // holding, input, coil, discrete
	RawValues []uint16 `json:"raw_values"`
}

// ScanResult is the output of a complete scan.
type ScanResult struct {
	Device       string             `json:"device"`
	UnitID       uint8              `json:"unit_id"`
	DurationMs   int64              `json:"scan_duration_ms"`
	Summary      ScanSummary        `json:"summary"`
	Registers    []AnalyzedRegister `json:"registers"`
}

type ScanSummary struct {
	TotalScanned int `json:"total_scanned"`
	Responsive   int `json:"responsive"`
	Dynamic      int `json:"dynamic"`
	Static       int `json:"static"`
}

// Scan runs a full scan against a Modbus TCP device.
func Scan(req ScanRequest) (*ScanResult, error) {
	req.applyDefaults()
	start := time.Now()

	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	handler := modbus.NewTCPClientHandler(addr)
	handler.Timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	handler.SlaveId = req.UnitID

	if err := handler.Connect(); err != nil {
		return nil, fmt.Errorf("connect %s: %w", addr, err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)

	// Phase 1+2: Scan registers (first sample)
	var allRaw []RawRegister
	totalScanned := 0

	for _, scanType := range req.ScanTypes {
		raws, scanned := scanRegisters(client, scanType, req.AddressStart, req.AddressEnd, req.BatchSize, req.DelayMs)
		allRaw = append(allRaw, raws...)
		totalScanned += scanned
		slog.Info("scan phase complete",
			"type", scanType,
			"responsive", len(raws),
			"scanned", scanned,
		)
	}

	// Phase 3: Multi-sample for dynamic detection
	if req.Samples > 1 && len(allRaw) > 0 {
		for s := 1; s < req.Samples; s++ {
			time.Sleep(time.Duration(req.SampleIntervalMs) * time.Millisecond)
			resample(client, allRaw, req.DelayMs)
			slog.Info("resample complete", "sample", s+1, "of", req.Samples)
		}
	}

	// Phase 4: Analyze
	analyzed := Analyze(allRaw)

	dynamic := 0
	static := 0
	for _, a := range analyzed {
		if a.IsDynamic {
			dynamic++
		} else {
			static++
		}
	}

	return &ScanResult{
		Device:     addr,
		UnitID:     req.UnitID,
		DurationMs: time.Since(start).Milliseconds(),
		Summary: ScanSummary{
			TotalScanned: totalScanned,
			Responsive:   len(analyzed),
			Dynamic:      dynamic,
			Static:       static,
		},
		Registers: analyzed,
	}, nil
}

// scanRegisters reads a range of registers in batches.
func scanRegisters(client modbus.Client, regType string, start, end, batchSize uint16, delayMs int) ([]RawRegister, int) {
	var results []RawRegister
	scanned := 0

	for addr := start; addr <= end; {
		count := batchSize
		if addr+count-1 > end {
			count = end - addr + 1
		}

		data, err := readBatch(client, regType, addr, count)
		scanned += int(count)

		if err == nil && len(data) > 0 {
			values := bytesToUint16(data)
			for i, v := range values {
				results = append(results, RawRegister{
					Address:   addr + uint16(i),
					Type:      regType,
					RawValues: []uint16{v},
				})
			}
		}

		addr += count
		if delayMs > 0 {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	return results, scanned
}

// resample re-reads all previously responsive registers and appends values.
func resample(client modbus.Client, registers []RawRegister, delayMs int) {
	for i := range registers {
		reg := &registers[i]
		data, err := readBatch(client, reg.Type, reg.Address, 1)
		if err == nil && len(data) >= 2 {
			val := binary.BigEndian.Uint16(data[:2])
			reg.RawValues = append(reg.RawValues, val)
		}
		if delayMs > 0 && i%50 == 49 {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}
}

func readBatch(client modbus.Client, regType string, addr, count uint16) ([]byte, error) {
	switch regType {
	case "holding":
		return client.ReadHoldingRegisters(addr, count)
	case "input":
		return client.ReadInputRegisters(addr, count)
	case "coil":
		return client.ReadCoils(addr, count)
	case "discrete":
		return client.ReadDiscreteInputs(addr, count)
	default:
		return nil, fmt.Errorf("unknown register type: %s", regType)
	}
}

func bytesToUint16(data []byte) []uint16 {
	n := len(data) / 2
	values := make([]uint16, n)
	for i := 0; i < n; i++ {
		values[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
	}
	return values
}

// Float32FromPair converts two consecutive uint16 registers to float32 (big-endian).
func Float32FromPair(hi, lo uint16) float32 {
	bits := uint32(hi)<<16 | uint32(lo)
	return math.Float32frombits(bits)
}
