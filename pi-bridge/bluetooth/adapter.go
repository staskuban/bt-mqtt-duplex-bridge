package bluetooth

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

var logger = log.New(os.Stdout, "[Bluetooth-Adapter] ", log.LstdFlags|log.Lshortfile)

// Config представляет конфигурацию для Bluetooth адаптера
type Config struct {
	DevicePath        string        `yaml:"device_path"`        // Путь к устройству, например "/dev/rfcomm0"
	ReconnectInterval time.Duration `yaml:"reconnect_interval"` // Интервал переподключения при ошибках
	ConnectTimeout    time.Duration `yaml:"connect_timeout"`    // Таймаут на подключение
	ReadTimeout       time.Duration `yaml:"read_timeout"`       // Таймаут на чтение
	WriteTimeout      time.Duration `yaml:"write_timeout"`      // Таймаут на запись
	InitCommands      []string      `yaml:"init_commands"`      // Команды для инициализации ELM327
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		DevicePath:        "/dev/rfcomm0",
		ReconnectInterval: 5 * time.Second,
		ConnectTimeout:    10 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      1 * time.Second,
		InitCommands: []string{
			"ATZ",   // Полный сброс
			"ATE0",  // Отключить эхо
			"ATL0",  // Отключить перевод строки
			"ATH1",  // Включить заголовки
			"ATSP0", // Автоматический выбор протокола
		},
	}
}

// Adapter представляет Bluetooth адаптер для работы с ELM327
type Adapter struct {
	config        Config
	conn          io.ReadWriteCloser
	connMutex     sync.RWMutex
	responsesChan chan<- string  // Канал для отправки ответов (только для записи)
	commandsChan  <-chan string  // Канал для получения команд (только для чтения)
	stopChan      chan struct{}  // Канал для graceful shutdown
	wg            sync.WaitGroup // WaitGroup для синхронизации горутин
}

// NewAdapter создает новый Bluetooth адаптер
func NewAdapter(config Config, responsesChan chan<- string, commandsChan <-chan string) *Adapter {
	return &Adapter{
		config:        config,
		responsesChan: responsesChan,
		commandsChan:  commandsChan,
		stopChan:      make(chan struct{}),
	}
}

// Start запускает работу адаптера
func (a *Adapter) Start() error {
	logger.Printf("Starting Bluetooth adapter with device: %s", a.config.DevicePath)

	// Запускаем горутину для чтения данных
	a.wg.Add(1)
	go a.readLoop()

	// Запускаем горутину для записи команд
	a.wg.Add(1)
	go a.writeLoop()

	// Запускаем горутину для переподключения
	a.wg.Add(1)
	go a.reconnectLoop()

	return nil
}

// Stop останавливает работу адаптера
func (a *Adapter) Stop() error {
	logger.Println("Stopping Bluetooth adapter...")
	close(a.stopChan)
	a.wg.Wait()

	a.connMutex.Lock()
	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
	a.connMutex.Unlock()

	logger.Println("Bluetooth adapter stopped")
	return nil
}

// isConnected проверяет, подключен ли адаптер
func (a *Adapter) isConnected() bool {
	a.connMutex.RLock()
	defer a.connMutex.RUnlock()
	return a.conn != nil
}

// setConnection устанавливает соединение
func (a *Adapter) setConnection(conn io.ReadWriteCloser) {
	a.connMutex.Lock()
	a.conn = conn
	a.connMutex.Unlock()
	logger.Println("Bluetooth connection established")
}

// getConnection получает соединение
func (a *Adapter) getConnection() io.ReadWriteCloser {
	a.connMutex.RLock()
	defer a.connMutex.RUnlock()
	return a.conn
}

// closeConnection закрывает текущее соединение
func (a *Adapter) closeConnection() {
	a.connMutex.Lock()
	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
	a.connMutex.Unlock()
	logger.Println("Bluetooth connection closed")
}

// connect устанавливает соединение с устройством
func (a *Adapter) connect() error {
	logger.Printf("Attempting to connect to %s", a.config.DevicePath)

	// Проверяем, существует ли устройство
	if _, err := os.Stat(a.config.DevicePath); os.IsNotExist(err) {
		return fmt.Errorf("device %s does not exist. Please run 'sudo rfcomm bind' first", a.config.DevicePath)
	}

	// Открываем устройство
	file, err := os.OpenFile(a.config.DevicePath, os.O_RDWR|unix.O_NOCTTY|os.O_SYNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", a.config.DevicePath, err)
	}

	// Устанавливаем соединение
	a.setConnection(file)

	// Выполняем инициализацию ELM327
	if err := a.initializeELM327(); err != nil {
		a.closeConnection()
		return fmt.Errorf("failed to initialize ELM327: %v", err)
	}

	return nil
}

// initializeELM327 выполняет инициализацию ELM327 после подключения
func (a *Adapter) initializeELM327() error {
	conn := a.getConnection()
	if conn == nil {
		return fmt.Errorf("no connection available for initialization")
	}

	logger.Println("Initializing ELM327...")

	// Небольшая пауза после подключения
	time.Sleep(500 * time.Millisecond)

	// Отправляем команды инициализации последовательно
	for i, cmd := range a.config.InitCommands {
		logger.Printf("Sending init command %d/%d: %s", i+1, len(a.config.InitCommands), cmd)

		cmdBytes := []byte(cmd + "\r")
		if _, err := conn.Write(cmdBytes); err != nil {
			return fmt.Errorf("failed to send command %s: %v", cmd, err)
		}

		// Ждем ответ на команду инициализации
		time.Sleep(200 * time.Millisecond)

		// Читаем ответ
		response := make([]byte, 128)
		n, err := conn.Read(response)
		if err != nil {
			logger.Printf("Warning: No response to %s (err: %v). Continuing...", cmd, err)
		} else if n > 0 {
			respStr := string(response[:n])
			logger.Printf("Response to %s: %q", cmd, respStr)
		}
	}

	logger.Println("ELM327 initialization completed")
	return nil
}

// readLoop читает данные из Bluetooth соединения
func (a *Adapter) readLoop() {
	defer a.wg.Done()
	logger.Println("Starting Bluetooth read loop")

	for {
		select {
		case <-a.stopChan:
			logger.Println("Read loop stopped")
			return
		default:
		}

		conn := a.getConnection()
		if conn == nil {
			time.Sleep(a.config.ReconnectInterval)
			continue
		}

		// Создаем reader с таймаутом
		reader := bufio.NewReader(conn)

		// Читаем до символа '>' (конец ответа ELM327)
		data, err := reader.ReadBytes('>')
		if err != nil {
			logger.Printf("Read error: %v", err)
			a.closeConnection()
			time.Sleep(a.config.ReconnectInterval)
			continue
		}

		// Удаляем trailing '>' если есть
		response := string(data)
		if len(response) > 0 && response[len(response)-1] == '>' {
			response = response[:len(response)-1]
		}

		// Убираем лишние пробелы
		response = fmt.Sprintf("%s", response)

		logger.Printf("Received from ELM327: %q", response)

		// Отправляем ответ в канал (неблокирующе)
		select {
		case a.responsesChan <- response:
			// Ответ отправлен успешно
		default:
			logger.Printf("Warning: responses channel is full, dropping response: %q", response)
		}
	}
}

// writeLoop отправляет команды в Bluetooth соединение
func (a *Adapter) writeLoop() {
	defer a.wg.Done()
	logger.Println("Starting Bluetooth write loop")

	for {
		select {
		case <-a.stopChan:
			logger.Println("Write loop stopped")
			return
		case command, ok := <-a.commandsChan:
			if !ok {
				logger.Println("Commands channel closed")
				return
			}

			conn := a.getConnection()
			if conn == nil {
				logger.Printf("Cannot send command %q: no connection", command)
				continue
			}

			logger.Printf("Sending command to ELM327: %q", command)

			// Добавляем символ возврата каретки
			cmdBytes := []byte(command + "\r")

			// TODO: Установить таймаут на запись при использовании net.Conn вместо io.ReadWriteCloser
			_, err := conn.Write(cmdBytes)
			if err != nil {
				logger.Printf("Write error: %v", err)
				a.closeConnection()
				continue
			}

			logger.Printf("Command sent successfully: %q", command)
		}
	}
}

// reconnectLoop управляет переподключением при ошибках
func (a *Adapter) reconnectLoop() {
	defer a.wg.Done()
	logger.Println("Starting Bluetooth reconnect loop")

	// Первая попытка подключения
	if err := a.connect(); err != nil {
		logger.Printf("Initial connection failed: %v", err)
	}

	ticker := time.NewTicker(a.config.ReconnectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopChan:
			logger.Println("Reconnect loop stopped")
			return
		case <-ticker.C:
			if !a.isConnected() {
				logger.Println("Attempting to reconnect...")
				if err := a.connect(); err != nil {
					logger.Printf("Reconnection failed: %v", err)
				}
			}
		}
	}
}
