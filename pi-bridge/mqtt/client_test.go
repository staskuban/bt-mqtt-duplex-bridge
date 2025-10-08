package mqtt

import (
	"log"
	"os"
	"testing"
	"time"

	"elm327-bridge/obd"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Broker == "" {
		t.Error("Expected non-empty broker address")
	}

	if config.ClientID == "" {
		t.Error("Expected non-empty client ID")
	}

	if config.DataTopic == "" {
		t.Error("Expected non-empty data topic")
	}

	if config.CommandTopic == "" {
		t.Error("Expected non-empty command topic")
	}

	if config.QoS < 0 || config.QoS > 2 {
		t.Errorf("Expected QoS between 0 and 2, got %d", config.QoS)
	}
}

func TestGenerateClientID(t *testing.T) {
	id1 := generateClientID()
	id2 := generateClientID()

	if id1 == "" {
		t.Error("Expected non-empty client ID")
	}

	if id1 == id2 {
		t.Error("Expected unique client IDs")
	}

	if len(id1) < 15 { // "elm327-bridge-" + 4 bytes hex = 15+ chars
		t.Errorf("Expected client ID length >= 15, got %d", len(id1))
	}
}

func TestConvertToTelemetryMessage(t *testing.T) {
	logger := log.New(os.Stdout, "[Test] ", log.LstdFlags)
	client := &Client{
		vin:    "TEST123",
		logger: logger,
	}

	// Создаем тестовые данные телеметрии
	telemetry := obd.Telemetry{
		PID:       "0C",
		Metric:    "engine_rpm",
		Value:     1724.5,
		Unit:      "rpm",
		Timestamp: 1234567890,
		Raw:       "41 0C 1A F0",
	}

	msg, err := client.convertToTelemetryMessage(telemetry)
	if err != nil {
		t.Fatalf("Failed to convert telemetry: %v", err)
	}

	if msg.VIN != "TEST123" {
		t.Errorf("Expected VIN 'TEST123', got %s", msg.VIN)
	}

	if msg.PID != "0C" {
		t.Errorf("Expected PID '0C', got %s", msg.PID)
	}

	if msg.Value != 1724.5 {
		t.Errorf("Expected value 1724.5, got %.2f", msg.Value)
	}

	if msg.Unit != "rpm" {
		t.Errorf("Expected unit 'rpm', got %s", msg.Unit)
	}
}

func TestConvertToTelemetryMessageUnsupportedType(t *testing.T) {
	logger := log.New(os.Stdout, "[Test] ", log.LstdFlags)
	client := &Client{
		vin:    "TEST123",
		logger: logger,
	}

	_, err := client.convertToTelemetryMessage("unsupported string")
	if err == nil {
		t.Error("Expected error for unsupported type")
	}
}

func TestCommandMessageStructure(t *testing.T) {
	cmd := CommandMessage{
		Command:       "010C",
		CorrelationID: "test-123",
		Description:   "Запрос оборотов двигателя",
		VIN:           "TEST123",
	}

	if cmd.Command != "010C" {
		t.Errorf("Expected command '010C', got %s", cmd.Command)
	}

	if cmd.CorrelationID != "test-123" {
		t.Errorf("Expected correlation_id 'test-123', got %s", cmd.CorrelationID)
	}
}

func TestCommandResponseStructure(t *testing.T) {
	response := CommandResponse{
		CorrelationID: "test-123",
		Status:        "success",
		Result:        "OK",
		Timestamp:     time.Now(),
	}

	if response.CorrelationID != "test-123" {
		t.Errorf("Expected correlation_id 'test-123', got %s", response.CorrelationID)
	}

	if response.Status != "success" {
		t.Errorf("Expected status 'success', got %s", response.Status)
	}
}

func TestTelemetryMessageStructure(t *testing.T) {
	msg := TelemetryMessage{
		VIN:       "TEST123",
		PID:       "0C",
		Metric:    "engine_rpm",
		Value:     1724.5,
		Unit:      "rpm",
		Timestamp: time.Now(),
		Raw:       "41 0C 1A F0",
	}

	if msg.VIN != "TEST123" {
		t.Errorf("Expected VIN 'TEST123', got %s", msg.VIN)
	}

	if msg.Metric != "engine_rpm" {
		t.Errorf("Expected metric 'engine_rpm', got %s", msg.Metric)
	}

	if msg.Value != 1724.5 {
		t.Errorf("Expected value 1724.5, got %.2f", msg.Value)
	}
}

func TestClientCreation(t *testing.T) {
	config := DefaultConfig()
	telemetryChan := make(chan interface{}, 10)
	commandsChan := make(chan string, 10)
	responsesChan := make(chan CommandResponse, 10)

	logger := log.New(os.Stdout, "[Test] ", log.LstdFlags)
	client := NewClient(config, telemetryChan, commandsChan, responsesChan)

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.config.Broker != config.Broker {
		t.Errorf("Expected broker %s, got %s", config.Broker, client.config.Broker)
	}

	if cap(client.telemetryChan) != 10 {
		t.Errorf("Expected telemetry channel capacity 10, got %d", cap(client.telemetryChan))
	}
}

func TestSetVIN(t *testing.T) {
	logger := log.New(os.Stdout, "[Test] ", log.LstdFlags)
	client := &Client{
		vin:    "",
		logger: logger,
	}

	client.SetVIN("TEST123")

	if client.vin != "TEST123" {
		t.Errorf("Expected VIN 'TEST123', got %s", client.vin)
	}
}

func TestIsConnected(t *testing.T) {
	logger := log.New(os.Stdout, "[Test] ", log.LstdFlags)
	client := &Client{
		mqttClient: nil,
		logger:     logger,
	}

	// Тестируем с nil клиентом
	if client.IsConnected() {
		t.Error("Expected IsConnected to return false for nil client")
	}
}
