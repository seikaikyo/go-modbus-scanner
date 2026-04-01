package scanner

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

)

// --- Analyzer tests ---

func TestIsDynamic(t *testing.T) {
	tests := []struct {
		name   string
		values []uint16
		want   bool
	}{
		{"single value", []uint16{100}, false},
		{"all same", []uint16{100, 100, 100}, false},
		{"changing", []uint16{100, 101, 100}, true},
		{"empty", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDynamic(tt.values); got != tt.want {
				t.Errorf("isDynamic(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestCalcRange(t *testing.T) {
	r := calcRange([]uint16{50, 200, 100, 10, 150})
	if r.Min != 10 {
		t.Errorf("Min = %d, want 10", r.Min)
	}
	if r.Max != 200 {
		t.Errorf("Max = %d, want 200", r.Max)
	}
}

func TestIsMonotonic(t *testing.T) {
	tests := []struct {
		name   string
		values []uint16
		want   bool
	}{
		{"increasing", []uint16{1, 2, 3, 4, 5}, true},
		{"same", []uint16{5, 5, 5}, false},
		{"decreasing", []uint16{5, 4, 3}, false},
		{"too short", []uint16{1, 2}, false},
		{"mixed", []uint16{1, 3, 2, 4}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMonotonic(tt.values); got != tt.want {
				t.Errorf("isMonotonic(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestFloat32FromPair(t *testing.T) {
	// 25.5 in IEEE 754: 0x41CC0000 → hi=0x41CC lo=0x0000
	f := Float32FromPair(0x41CC, 0x0000)
	if math.Abs(float64(f)-25.5) > 0.01 {
		t.Errorf("Float32FromPair(0x41CC, 0x0000) = %f, want ~25.5", f)
	}

	// 100.0 in IEEE 754: 0x42C80000 → hi=0x42C8 lo=0x0000
	f = Float32FromPair(0x42C8, 0x0000)
	if math.Abs(float64(f)-100.0) > 0.01 {
		t.Errorf("Float32FromPair(0x42C8, 0x0000) = %f, want ~100.0", f)
	}
}

func TestInferType(t *testing.T) {
	tests := []struct {
		name string
		raw  RawRegister
		want string
	}{
		{
			"coil is bool",
			RawRegister{Type: "coil", RawValues: []uint16{1}},
			"bool",
		},
		{
			"small uint16",
			RawRegister{Type: "holding", RawValues: []uint16{100}},
			"uint16",
		},
		{
			"signed int16",
			RawRegister{Type: "holding", RawValues: []uint16{65000}},
			"int16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferType(tt.raw); got != tt.want {
				t.Errorf("inferType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnalyze(t *testing.T) {
	raws := []RawRegister{
		{Address: 0, Type: "holding", RawValues: []uint16{250, 253, 251}},
		{Address: 1, Type: "holding", RawValues: []uint16{100, 100, 100}},
		{Address: 10, Type: "coil", RawValues: []uint16{1, 0, 1}},
	}

	result := Analyze(raws)

	if len(result) != 3 {
		t.Fatalf("Analyze returned %d registers, want 3", len(result))
	}

	// Register 0: dynamic
	if !result[0].IsDynamic {
		t.Error("Register 0 should be dynamic")
	}

	// Register 1: static
	if result[1].IsDynamic {
		t.Error("Register 1 should be static")
	}

	// Register 10: coil = bool
	if result[2].InferredType != "bool" {
		t.Errorf("Register 10 type = %q, want bool", result[2].InferredType)
	}
}

func TestAnalyzeFloat32Pair(t *testing.T) {
	// 25.5 = 0x41CC0000
	raws := []RawRegister{
		{Address: 100, Type: "holding", RawValues: []uint16{0x41CC}},
		{Address: 101, Type: "holding", RawValues: []uint16{0x0000}},
	}

	result := Analyze(raws)

	if len(result) != 1 {
		t.Fatalf("Analyze returned %d registers, want 1 (pair merged)", len(result))
	}

	if result[0].InferredType != "float32_hi" {
		t.Errorf("type = %q, want float32_hi", result[0].InferredType)
	}
	if result[0].Float32Value == nil {
		t.Fatal("Float32Value is nil")
	}
	if math.Abs(*result[0].Float32Value-25.5) > 0.01 {
		t.Errorf("Float32Value = %f, want ~25.5", *result[0].Float32Value)
	}
	if result[0].PairedAddr == nil || *result[0].PairedAddr != 101 {
		t.Error("PairedAddr should be 101")
	}
}

func TestGuessCategoryCounter(t *testing.T) {
	raws := []RawRegister{
		{Address: 50, Type: "holding", RawValues: []uint16{100, 105, 110, 115}},
	}

	result := Analyze(raws)
	if len(result) != 1 {
		t.Fatal("expected 1 register")
	}
	if result[0].Guess == nil || result[0].Guess.Category != "counter" {
		cat := ""
		if result[0].Guess != nil {
			cat = result[0].Guess.Category
		}
		t.Errorf("category = %q, want counter", cat)
	}
}

// --- Router tests ---

func TestScanEndpointValidation(t *testing.T) {
	r := Router()

	// Missing host
	req := httptest.NewRequest("POST", "/api/scan", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}

	var body apiResponse
	json.NewDecoder(w.Body).Decode(&body)
	if body.Success {
		t.Error("should fail without host")
	}
}

func TestListJobsEmpty(t *testing.T) {
	r := Router()

	req := httptest.NewRequest("GET", "/api/jobs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestGetJobNotFound(t *testing.T) {
	r := Router()

	req := httptest.NewRequest("GET", "/api/jobs/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestReadEndpointValidation(t *testing.T) {
	r := Router()

	req := httptest.NewRequest("POST", "/api/read", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestScanRequestDefaults(t *testing.T) {
	req := ScanRequest{Host: "10.0.0.1"}
	req.applyDefaults()

	if req.Port != 502 {
		t.Errorf("Port = %d, want 502", req.Port)
	}
	if req.UnitID != 1 {
		t.Errorf("UnitID = %d, want 1", req.UnitID)
	}
	if req.BatchSize != 125 {
		t.Errorf("BatchSize = %d, want 125", req.BatchSize)
	}
	if req.Samples != 3 {
		t.Errorf("Samples = %d, want 3", req.Samples)
	}
	if req.AddressEnd != 9999 {
		t.Errorf("AddressEnd = %d, want 9999", req.AddressEnd)
	}
}
