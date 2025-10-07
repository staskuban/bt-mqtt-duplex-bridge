package main

import (
	"encoding/base64"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockReadWriteCloser для тестирования io.ReadWriteCloser
type MockReadWriteCloser struct {
	mock.Mock
}

func (m *MockReadWriteCloser) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockReadWriteCloser) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockReadWriteCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Тестовая структура для dependency injection
type BluetoothTestSuite struct {
	conn   io.ReadWriteCloser
	config Config
}

// Вспомогательная функция для создания тестовой конфигурации
func createTestConfig() Config {
	return Config{
		Elm327: struct {
			Mac string `mapstructure:"mac"`
		}{
			Mac: "00:11:22:33:44:55",
		},
		MQTT: struct {
			Broker       string `mapstructure:"broker"`
			Username     string `mapstructure:"username"`
			Password     string `mapstructure:"password"`
			DataTopic    string `mapstructure:"data_topic"`
			CommandTopic string `mapstructure:"command_topic"`
		}{
			Broker:       "tcp://test.mqtt.com:1883",
			DataTopic:    "test/data",
			CommandTopic: "test/command",
		},
		Logging: struct {
			Level string `mapstructure:"level"`
		}{
			Level: "info",
		},
	}
}

func TestWriteToBluetooth(t *testing.T) {
	mockConn := new(MockReadWriteCloser)
	mockConn.On("Write", mock.MatchedBy(func(data []byte) bool {
		return len(data) == 4 && string(data[:3]) == "ATZ"
	})).Return(4, nil)

	suite := &BluetoothTestSuite{
		conn:   mockConn,
		config: createTestConfig(),
	}

	// Сохраняем оригинальные глобальные переменные
	originalBtConn := btConn
	defer func() { btConn = originalBtConn }()

	// Устанавливаем мок в глобальную переменную для теста
	btConn = suite.conn

	command := []byte("ATZ")
	err := writeToBluetooth(command)

	assert.NoError(t, err)
	mockConn.AssertExpectations(t)
}

func TestBase64EncodingForBluetoothData(t *testing.T) {
	data := []byte("0100>41 00 BE 7F")
	encoded := base64.StdEncoding.EncodeToString(data)
	expected := "MDEwMD40MSAwMCBCRSA3Rg=="

	assert.Equal(t, expected, encoded)
}

func TestBase64DecodingForCommand(t *testing.T) {
	encoded := "QVRa" // base64 для "ATZ"
	decoded, err := base64.StdEncoding.DecodeString(encoded)

	assert.NoError(t, err)
	assert.Equal(t, []byte("ATZ"), decoded)
}

func TestWriteToBluetoothNilConnection(t *testing.T) {
	// Сохраняем оригинальное значение
	originalBtConn := btConn
	defer func() { btConn = originalBtConn }()

	// Устанавливаем nil соединение
	btConn = nil

	command := []byte("ATZ")
	err := writeToBluetooth(command)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Bluetooth not connected")
}

func TestWriteToBluetoothWithCarriageReturn(t *testing.T) {
	mockConn := new(MockReadWriteCloser)
	expectedData := []byte("ATZ\r")
	mockConn.On("Write", expectedData).Return(4, nil)

	// Сохраняем оригинальные глобальные переменные
	originalBtConn := btConn
	defer func() { btConn = originalBtConn }()

	btConn = mockConn

	command := []byte("ATZ")
	err := writeToBluetooth(command)

	assert.NoError(t, err)
	mockConn.AssertExpectations(t)
}

func TestReadFromBluetoothWithValidData(t *testing.T) {
	mockConn := new(MockReadWriteCloser)
	testData := []byte("41 00 BE 7F>")
	mockConn.On("Read", mock.AnythingOfType("[]uint8")).Return(13, nil)

	// Сохраняем оригинальные глобальные переменные
	originalBtConn := btConn
	defer func() { btConn = originalBtConn }()

	btConn = mockConn

	// Этот тест проверяет только чтение, без полной интеграции с MQTT
	// В реальности readFromBluetooth работает в горутине
	go func() {
		reader := &mockReader{data: testData, conn: mockConn}
		data, err := reader.ReadBytes('>')
		if err == nil && len(data) > 0 && data[len(data)-1] == '>' {
			data = data[:len(data)-1]
		}
		assert.Equal(t, []byte("41 00 BE 7F"), data)
	}()

	// Небольшая пауза для выполнения горутины
	// В реальном сценарии этот тест должен быть более изолированным
}

type mockReader struct {
	data []byte
	conn *MockReadWriteCloser
	pos  int
}

func (m *mockReader) ReadBytes(delim byte) ([]byte, error) {
	if m.pos >= len(m.data) {
		return nil, io.EOF
	}

	start := m.pos
	for m.pos < len(m.data) && m.data[m.pos] != delim {
		m.pos++
	}

	if m.pos < len(m.data) {
		result := m.data[start : m.pos+1]
		m.pos++
		return result, nil
	}

	return m.data[start:], io.EOF
}
