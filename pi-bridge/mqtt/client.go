package mqtt

import (
	"crypto/rand"
	"elm327-bridge/common"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	mqttLib "github.com/eclipse/paho.mqtt.golang"
)

// Config представляет конфигурацию MQTT клиента
type Config struct {
	Broker         string        `yaml:"broker"`          // Адрес брокера, например "tcp://localhost:1883"
	Username       string        `yaml:"username"`        // Имя пользователя (опционально)
	Password       string        `yaml:"password"`        // Пароль (опционально)
	ClientID       string        `yaml:"client_id"`       // ID клиента (опционально, генерируется если пустой)
	DataTopic      string        `yaml:"data_topic"`      // Базовый топик для данных телеметрии
	CommandTopic   string        `yaml:"command_topic"`   // Базовый топик для команд
	QoS            byte          `yaml:"qos"`             // Quality of Service (0, 1, 2)
	KeepAlive      int           `yaml:"keep_alive"`      // Интервал keep alive в секундах
	ConnectTimeout time.Duration `yaml:"connect_timeout"` // Таймаут подключения
	AutoReconnect  bool          `yaml:"auto_reconnect"`  // Автоматическое переподключение
}

// generateClientID генерирует случайный ID клиента
func generateClientID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return "elm327-bridge-" + hex.EncodeToString(bytes)
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		Broker:         "tcp://localhost:1883",
		ClientID:       generateClientID(),
		DataTopic:      "car/telemetry",
		CommandTopic:   "car/command",
		QoS:            1,
		KeepAlive:      60,
		ConnectTimeout: 10 * time.Second,
		AutoReconnect:  true,
	}
}

// TelemetryMessage представляет сообщение с данными телеметрии для MQTT
type TelemetryMessage struct {
	VIN       string    `json:"vin"`
	PID       string    `json:"pid"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
	Raw       string    `json:"raw,omitempty"`
}

// CommandMessage представляет входящую команду (используем общий тип)
type CommandMessage = common.CommandMessage

// CommandResponse представляет ответ на команду (используем общий тип)
type CommandResponse = common.CommandResponse

// Client представляет MQTT клиента
type Client struct {
	config           Config
	mqttClient       mqttLib.Client
	telemetryChan    <-chan interface{}          // Канал для получения данных телеметрии
	commandsChan     chan<- string               // Канал для отправки команд в Bluetooth
	commandResponses chan common.CommandResponse // Канал для ответов на команды (двунаправленный)
	stopChan         chan struct{}
	wg               sync.WaitGroup
	logger           *log.Logger
	vin              string // VIN автомобиля (определяется динамически)
}

// NewClient создает нового MQTT клиента
func NewClient(config Config, telemetryChan <-chan interface{}, commandsChan chan<- string, commandResponses chan common.CommandResponse) *Client {
	return &Client{
		config:           config,
		telemetryChan:    telemetryChan,
		commandsChan:     commandsChan,
		commandResponses: commandResponses,
		stopChan:         make(chan struct{}),
		logger:           log.New(os.Stdout, "[MQTT-Client] ", log.LstdFlags|log.Lshortfile),
	}
}

// Start запускает MQTT клиента
func (c *Client) Start() error {
	c.logger.Printf("Starting MQTT client, broker: %s", c.config.Broker)

	// Создаем опции подключения
	opts := mqttLib.NewClientOptions()
	opts.AddBroker(c.config.Broker)
	opts.SetClientID(c.config.ClientID)
	opts.SetKeepAlive(time.Duration(c.config.KeepAlive) * time.Second)
	opts.SetConnectTimeout(c.config.ConnectTimeout)
	opts.SetAutoReconnect(c.config.AutoReconnect)

	// Устанавливаем аутентификацию если задана
	if c.config.Username != "" && c.config.Password != "" {
		opts.SetUsername(c.config.Username)
		opts.SetPassword(c.config.Password)
		c.logger.Println("MQTT authentication: ENABLED")
	} else {
		c.logger.Println("MQTT authentication: DISABLED (anonymous mode)")
	}

	// Обработчики событий
	opts.SetOnConnectHandler(c.onConnectHandler)
	opts.SetConnectionLostHandler(c.onConnectionLostHandler)
	opts.SetReconnectingHandler(c.onReconnectingHandler)

	// Создаем клиента
	c.mqttClient = mqttLib.NewClient(opts)

	// Подключаемся
	if token := c.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}

	c.logger.Println("MQTT client started successfully")
	return nil
}

// Stop останавливает MQTT клиента
func (c *Client) Stop() error {
	c.logger.Println("Stopping MQTT client...")

	close(c.stopChan)
	c.wg.Wait()

	if c.mqttClient != nil && c.mqttClient.IsConnected() {
		c.mqttClient.Disconnect(1000)
		c.logger.Println("MQTT client disconnected")
	}

	return nil
}

// onConnectHandler вызывается при успешном подключении к брокеру
func (c *Client) onConnectHandler(client mqttLib.Client) {
	c.logger.Println("Connected to MQTT broker")

	// Подписываемся на топики команд
	commandTopic := fmt.Sprintf("%s/+/request", c.config.CommandTopic)
	if token := client.Subscribe(commandTopic, c.config.QoS, c.onCommandReceived); token.Wait() && token.Error() != nil {
		c.logger.Printf("Failed to subscribe to command topic %s: %v", commandTopic, token.Error())
		return
	}
	c.logger.Printf("Subscribed to command topic: %s", commandTopic)

	// Запускаем горутину для публикации телеметрии
	c.wg.Add(1)
	go c.publishTelemetryLoop()

	// Запускаем горутину для публикации ответов на команды
	c.wg.Add(1)
	go c.publishResponsesLoop()
}

// onConnectionLostHandler вызывается при потере соединения
func (c *Client) onConnectionLostHandler(client mqttLib.Client, err error) {
	c.logger.Printf("Connection lost: %v", err)
}

// onReconnectingHandler вызывается при попытке переподключения
func (c *Client) onReconnectingHandler(client mqttLib.Client, opts *mqttLib.ClientOptions) {
	c.logger.Println("Attempting to reconnect to MQTT broker...")
}

// onCommandReceived обрабатывает входящие команды
func (c *Client) onCommandReceived(client mqttLib.Client, msg mqttLib.Message) {
	c.logger.Printf("Received command on topic: %s", msg.Topic())

	var cmd CommandMessage
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		c.logger.Printf("Failed to unmarshal command: %v", err)
		return
	}

	c.logger.Printf("Processing command: %s (correlation_id: %s)", cmd.Command, cmd.CorrelationID)

	// Отправляем команду в канал для Bluetooth модуля
	select {
	case c.commandsChan <- cmd.Command:
		c.logger.Printf("Command sent to Bluetooth: %s", cmd.Command)
	case <-time.After(5 * time.Second):
		c.logger.Printf("Timeout sending command to Bluetooth: %s", cmd.Command)
	}
}

// publishTelemetryLoop публикует данные телеметрии
func (c *Client) publishTelemetryLoop() {
	defer c.wg.Done()
	c.logger.Println("Starting telemetry publish loop")

	for {
		select {
		case <-c.stopChan:
			c.logger.Println("Telemetry publish loop stopped")
			return
		case telemetryData, ok := <-c.telemetryChan:
			if !ok {
				c.logger.Println("Telemetry channel closed")
				return
			}

			// Конвертируем данные в TelemetryMessage
			msg, err := c.convertToTelemetryMessage(telemetryData)
			if err != nil {
				c.logger.Printf("Failed to convert telemetry data: %v", err)
				continue
			}

			// Публикуем в MQTT
			if err := c.publishTelemetry(msg); err != nil {
				c.logger.Printf("Failed to publish telemetry: %v", err)
			}
		}
	}
}

// publishResponsesLoop публикует ответы на команды
func (c *Client) publishResponsesLoop() {
	defer c.wg.Done()
	c.logger.Println("Starting responses publish loop")

	for {
		select {
		case <-c.stopChan:
			c.logger.Println("Responses publish loop stopped")
			return
		case response, ok := <-c.commandResponses:
			if !ok {
				c.logger.Println("Command responses channel closed")
				return
			}

			// Публикуем ответ в MQTT
			if err := c.publishCommandResponse(response); err != nil {
				c.logger.Printf("Failed to publish command response: %v", err)
			}
		}
	}
}

// convertToTelemetryMessage конвертирует данные телеметрии в MQTT сообщение
func (c *Client) convertToTelemetryMessage(data interface{}) (*TelemetryMessage, error) {
	// Пытаемся привести к типу common.Telemetry
	if telemetry, ok := data.(common.Telemetry); ok {
		return &TelemetryMessage{
			VIN:       c.vin, // TODO: Получить реальный VIN
			PID:       telemetry.PID,
			Metric:    telemetry.Metric,
			Value:     telemetry.Value,
			Unit:      telemetry.Unit,
			Timestamp: time.Now(),
			Raw:       telemetry.Raw,
		}, nil
	}

	return nil, fmt.Errorf("unsupported telemetry data type: %T", data)
}

// publishTelemetry публикует данные телеметрии в MQTT
func (c *Client) publishTelemetry(msg *TelemetryMessage) error {
	if c.mqttClient == nil || !c.mqttClient.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	// Создаем JSON payload
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry message: %v", err)
	}

	// Создаем топик
	topic := fmt.Sprintf("%s/%s/%s", c.config.DataTopic, c.vin, msg.Metric)

	// Публикуем
	token := c.mqttClient.Publish(topic, c.config.QoS, false, payload)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("failed to publish to topic %s: %v", topic, token.Error())
	}

	c.logger.Printf("Published telemetry to %s: %.2f %s", topic, msg.Value, msg.Unit)
	return nil
}

// publishCommandResponse публикует ответ на команду в MQTT
func (c *Client) publishCommandResponse(response CommandResponse) error {
	if c.mqttClient == nil || !c.mqttClient.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	// Создаем JSON payload
	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal command response: %v", err)
	}

	// Создаем топик для ответа
	topic := fmt.Sprintf("%s/%s/response", c.config.CommandTopic, c.vin)

	// Публикуем
	token := c.mqttClient.Publish(topic, c.config.QoS, false, payload)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("failed to publish response to topic %s: %v", topic, token.Error())
	}

	c.logger.Printf("Published command response to %s: %s", topic, response.Status)
	return nil
}

// SetVIN устанавливает VIN автомобиля
func (c *Client) SetVIN(vin string) {
	c.vin = vin
	c.logger.Printf("VIN set to: %s", vin)
}

// IsConnected возвращает true если клиент подключен к брокеру
func (c *Client) IsConnected() bool {
	return c.mqttClient != nil && c.mqttClient.IsConnected()
}

// PublishCommandResponse публикует ответ на команду (может быть вызван извне)
func (c *Client) PublishCommandResponse(correlationID, status string, result interface{}, err error) {
	response := CommandResponse{
		CorrelationID: correlationID,
		Status:        status,
		Result:        result,
		Timestamp:     time.Now(),
	}

	if err != nil {
		response.Status = "error"
		response.Error = err.Error()
	}

	// Отправляем в канал для публикации
	select {
	case c.commandResponses <- response:
	case <-time.After(1 * time.Second):
		c.logger.Printf("Timeout publishing command response for correlation_id: %s", correlationID)
	}
}
