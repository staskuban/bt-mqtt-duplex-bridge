package bluetooth

import (
	"io"
	"strings"
	"testing"
	"time"
)

// MockReadWriteCloser для тестирования
type MockReadWriteCloser struct {
	readData  []byte
	readIndex int
	writeData []byte
	closed    bool
}

func (m *MockReadWriteCloser) Read(p []byte) (n int, err error) {
	if m.readIndex >= len(m.readData) {
		return 0, io.EOF
	}

	n = copy(p, m.readData[m.readIndex:])
	m.readIndex += n
	return n, nil
}

func (m *MockReadWriteCloser) Write(p []byte) (n int, err error) {
	m.writeData = append(m.writeData, p...)
	return len(p), nil
}

func (m *MockReadWriteCloser) Close() error {
	m.closed = true
	return nil
}

func TestNewAdapter(t *testing.T) {
	responsesChan := make(chan string, 10)
	commandsChan := make(chan string, 10)

	config := DefaultConfig()
	adapter := NewAdapter(config, responsesChan, commandsChan)

	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}

	if adapter.config.DevicePath != "/dev/rfcomm0" {
		t.Errorf("Expected device path /dev/rfcomm0, got %s", adapter.config.DevicePath)
	}

	if cap(adapter.responsesChan) != 10 {
		t.Errorf("Expected responses channel capacity 10, got %d", cap(adapter.responsesChan))
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DevicePath != "/dev/rfcomm0" {
		t.Errorf("Expected device path /dev/rfcomm0, got %s", config.DevicePath)
	}

	if config.ReconnectInterval != 5*time.Second {
		t.Errorf("Expected reconnect interval 5s, got %v", config.ReconnectInterval)
	}

	if len(config.InitCommands) == 0 {
		t.Error("Expected non-empty init commands")
	}

	expectedCommands := []string{"ATZ", "ATE0", "ATL0", "ATH1", "ATSP0"}
	for i, expected := range expectedCommands {
		if i >= len(config.InitCommands) || config.InitCommands[i] != expected {
			t.Errorf("Expected init command %d to be %s, got %v", i, expected, config.InitCommands)
		}
	}
}

func TestAdapterStartStop(t *testing.T) {
	responsesChan := make(chan string, 10)
	commandsChan := make(chan string, 10)

	config := DefaultConfig()
	adapter := NewAdapter(config, responsesChan, commandsChan)

	// Запускаем адаптер
	err := adapter.Start()
	if err != nil {
		t.Fatalf("Failed to start adapter: %v", err)
	}

	// Ждем немного, чтобы горутины запустились
	time.Sleep(100 * time.Millisecond)

	// Останавливаем адаптер
	err = adapter.Stop()
	if err != nil {
		t.Fatalf("Failed to stop adapter: %v", err)
	}
}

func TestMockReadWriteCloser(t *testing.T) {
	mock := &MockReadWriteCloser{
		readData: []byte("test data"),
	}

	// Тестируем чтение
	buf := make([]byte, 4)
	n, err := mock.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 4 {
		t.Errorf("Expected to read 4 bytes, got %d", n)
	}
	if string(buf) != "test" {
		t.Errorf("Expected 'test', got %s", string(buf))
	}

	// Тестируем запись
	testData := []byte("hello")
	n, err = mock.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to write 5 bytes, got %d", n)
	}
	if string(mock.writeData) != "hello" {
		t.Errorf("Expected 'hello', got %s", string(mock.writeData))
	}

	// Тестируем закрытие
	err = mock.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !mock.closed {
		t.Error("Expected mock to be closed")
	}
}

func TestAdapterWithMockConnection(t *testing.T) {
	// Создаем мок-соединение с тестовыми данными
	mockConn := &MockReadWriteCloser{
		readData: []byte("OK>"),
	}

	responsesChan := make(chan string, 10)
	commandsChan := make(chan string, 10)

	config := DefaultConfig()
	adapter := NewAdapter(config, responsesChan, commandsChan)

	// Устанавливаем мок-соединение напрямую для тестирования
	adapter.setConnection(mockConn)

	// Запускаем только writeLoop для тестирования записи
	go adapter.writeLoop()

	// Тестируем отправку команды
	commandsChan <- "ATZ"
	time.Sleep(50 * time.Millisecond)

	// Проверяем, что команда была записана
	if !strings.Contains(string(mockConn.writeData), "ATZ") {
		t.Error("Expected ATZ command to be written")
	}

	// Тестируем чтение ответа
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(commandsChan)
	}()

	// Запускаем только readLoop для тестирования чтения
	go adapter.readLoop()

	// Ждем получения данных
	select {
	case response := <-responsesChan:
		if !strings.Contains(response, "OK") {
			t.Errorf("Expected response to contain 'OK', got %s", response)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for response")
	}

	adapter.Stop()
}
