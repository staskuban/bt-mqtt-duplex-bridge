package obd

import (
	"testing"
)

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		expectedPID string
		expectedVal float64
		expectError bool
	}{
		{
			name:        "RPM parsing",
			response:    "41 0C 1A F0",
			expectedPID: "0C",
			expectedVal: 1724, // ((26 * 256) + 240) / 4 = 1724
			expectError: false,
		},
		{
			name:        "Vehicle speed parsing",
			response:    "41 0D 32",
			expectedPID: "0D",
			expectedVal: 50, // 0x32 = 50
			expectError: false,
		},
		{
			name:        "Coolant temperature parsing",
			response:    "41 05 5A",
			expectedPID: "05",
			expectedVal: 50, // 0x5A - 40 = 50
			expectError: false,
		},
		{
			name:        "Invalid response format",
			response:    "INVALID",
			expectError: true,
		},
		{
			name:        "Response too short",
			response:    "41 0C",
			expectError: true,
		},
		{
			name:        "Unsupported PID",
			response:    "41 FF 12 34",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			telemetry, err := ParseResponse(tt.response)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for response %q", tt.response)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for response %q: %v", tt.response, err)
				return
			}

			if telemetry.PID != tt.expectedPID {
				t.Errorf("Expected PID %s, got %s", tt.expectedPID, telemetry.PID)
			}

			if telemetry.Value != tt.expectedVal {
				t.Errorf("Expected value %.2f, got %.2f", tt.expectedVal, telemetry.Value)
			}

			if telemetry.Raw != tt.response {
				t.Errorf("Expected raw %q, got %q", tt.response, telemetry.Raw)
			}
		})
	}
}

func TestDecodeRPM(t *testing.T) {
	tests := []struct {
		data     []byte
		expected float64
		hasError bool
	}{
		{[]byte{0x1A, 0xF0}, 1724, false},   // ((26 * 256) + 240) / 4 = 1724
		{[]byte{0x0F, 0xA0}, 1000, false},   // ((15 * 256) + 160) / 4 = 1000
		{[]byte{0x00, 0x00}, 0, false},      // 0 RPM
		{[]byte{0x1A}, 0, true},             // Wrong length
		{[]byte{0x1A, 0xF0, 0x00}, 0, true}, // Wrong length
	}

	for _, tt := range tests {
		result, err := decodeRPM(tt.data)

		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for data %v", tt.data)
			}
			continue
		}

		if err != nil {
			t.Errorf("Unexpected error for data %v: %v", tt.data, err)
			continue
		}

		if result != tt.expected {
			t.Errorf("Expected %.2f, got %.2f for data %v", tt.expected, result, tt.data)
		}
	}
}

func TestDecodeVehicleSpeed(t *testing.T) {
	tests := []struct {
		data     []byte
		expected float64
		hasError bool
	}{
		{[]byte{0x00}, 0, false},
		{[]byte{0x32}, 50, false},
		{[]byte{0xFF}, 255, false},
		{[]byte{0x32, 0x00}, 0, true}, // Wrong length
		{[]byte{}, 0, true},           // Empty data
	}

	for _, tt := range tests {
		result, err := decodeVehicleSpeed(tt.data)

		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for data %v", tt.data)
			}
			continue
		}

		if err != nil {
			t.Errorf("Unexpected error for data %v: %v", tt.data, err)
			continue
		}

		if result != tt.expected {
			t.Errorf("Expected %.2f, got %.2f for data %v", tt.expected, result, tt.data)
		}
	}
}

func TestDecodeCoolantTemp(t *testing.T) {
	tests := []struct {
		data     []byte
		expected float64
		hasError bool
	}{
		{[]byte{0x5A}, 50, false},     // 0x5A - 40 = 50
		{[]byte{0x00}, -40, false},    // 0x00 - 40 = -40
		{[]byte{0xFF}, 215, false},    // 0xFF - 40 = 215
		{[]byte{0x32, 0x00}, 0, true}, // Wrong length
	}

	for _, tt := range tests {
		result, err := decodeCoolantTemp(tt.data)

		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for data %v", tt.data)
			}
			continue
		}

		if err != nil {
			t.Errorf("Unexpected error for data %v: %v", tt.data, err)
			continue
		}

		if result != tt.expected {
			t.Errorf("Expected %.2f, got %.2f for data %v", tt.expected, result, tt.data)
		}
	}
}

func TestDecodeThrottlePos(t *testing.T) {
	tests := []struct {
		data     []byte
		expected float64
		hasError bool
	}{
		{[]byte{0x00}, 0, false},      // 0%
		{[]byte{0xFF}, 100, false},    // 100%
		{[]byte{0x80}, 50.196, false}, // (0x80 * 100) / 255 ≈ 50.196%
		{[]byte{0x32, 0x00}, 0, true}, // Wrong length
	}

	for _, tt := range tests {
		result, err := decodeThrottlePos(tt.data)

		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for data %v", tt.data)
			}
			continue
		}

		if err != nil {
			t.Errorf("Unexpected error for data %v: %v", tt.data, err)
			continue
		}

		// Используем небольшую дельту для сравнения float
		delta := 0.01
		if result < tt.expected-delta || result > tt.expected+delta {
			t.Errorf("Expected %.3f, got %.3f for data %v", tt.expected, result, tt.data)
		}
	}
}

func TestGetSupportedPIDs(t *testing.T) {
	pids := GetSupportedPIDs()

	if len(pids) == 0 {
		t.Error("Expected non-empty list of supported PIDs")
	}

	// Проверяем, что некоторые известные PID присутствуют
	expectedPIDs := []string{"0C", "0D", "05", "0F"}
	for _, expected := range expectedPIDs {
		found := false
		for _, pid := range pids {
			if pid == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected PID %s to be in supported list", expected)
		}
	}
}

func TestGetMetricName(t *testing.T) {
	tests := []struct {
		pid      string
		expected string
	}{
		{"0C", "engine_rpm"},
		{"0D", "vehicle_speed"},
		{"05", "coolant_temperature"},
		{"FF", "unknown_FF"}, // Unknown PID
	}

	for _, tt := range tests {
		result := GetMetricName(tt.pid)
		if result != tt.expected {
			t.Errorf("Expected metric name %s for PID %s, got %s", tt.expected, tt.pid, result)
		}
	}
}

func TestGetMetricUnit(t *testing.T) {
	tests := []struct {
		pid      string
		expected string
	}{
		{"0C", "rpm"},
		{"0D", "km/h"},
		{"05", "°C"},
		{"FF", "unknown"}, // Unknown PID
	}

	for _, tt := range tests {
		result := GetMetricUnit(tt.pid)
		if result != tt.expected {
			t.Errorf("Expected unit %s for PID %s, got %s", tt.expected, tt.pid, result)
		}
	}
}

func TestTelemetryStructure(t *testing.T) {
	// Тестируем создание структуры Telemetry
	telemetry := &Telemetry{
		PID:       "0C",
		Metric:    "engine_rpm",
		Value:     1724.5,
		Unit:      "rpm",
		Timestamp: 1234567890,
		Raw:       "41 0C 1A F0",
	}

	if telemetry.PID != "0C" {
		t.Errorf("Expected PID '0C', got %s", telemetry.PID)
	}

	if telemetry.Value != 1724.5 {
		t.Errorf("Expected value 1724.5, got %.2f", telemetry.Value)
	}

	if telemetry.Unit != "rpm" {
		t.Errorf("Expected unit 'rpm', got %s", telemetry.Unit)
	}
}

// Бенчмарк для тестирования производительности парсера
func BenchmarkParseResponse(b *testing.B) {
	response := "41 0C 1A F0"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseResponse(response)
		if err != nil {
			b.Fatalf("ParseResponse failed: %v", err)
		}
	}
}

// Тест для проверки корректности всех декодеров
func TestAllDecoders(t *testing.T) {
	testCases := []struct {
		pid      string
		response string
		expected float64
	}{
		{"0C", "41 0C 1A F0", 1724},   // RPM
		{"0D", "41 0D 32", 50},        // Speed
		{"05", "41 05 5A", 50},        // Coolant temp (0x5A - 40 = 50)
		{"0F", "41 0F 00", -40},       // Intake temp (0x00 - 40 = -40)
		{"11", "41 11 80", 50.196078}, // Throttle position (0x80 * 100 / 255 ≈ 50.196078)
		{"04", "41 04 33", 20},        // Engine load (0x33 * 100 / 255 ≈ 20)
		{"2F", "41 2F 66", 40},        // Fuel level (0x66 * 100 / 255 ≈ 40)
		{"0A", "41 0A 1F", 93},        // Fuel pressure (0x1F * 3 = 93)
		{"0B", "41 0B 64", 100},       // Intake pressure
		{"33", "41 33 61", 97},        // Barometric pressure
		{"21", "41 21 00 FA", 250},    // Distance with MIL (0x00FA = 250)
	}

	for _, tc := range testCases {
		t.Run(tc.pid, func(t *testing.T) {
			telemetry, err := ParseResponse(tc.response)
			if err != nil {
				t.Fatalf("Failed to parse response %s: %v", tc.response, err)
			}

			// Используем небольшую дельту для сравнения float
			delta := 0.01
			if telemetry.Value < tc.expected-delta || telemetry.Value > tc.expected+delta {
				t.Errorf("PID %s: expected %.6f, got %.6f", tc.pid, tc.expected, telemetry.Value)
			}
		})
	}
}
