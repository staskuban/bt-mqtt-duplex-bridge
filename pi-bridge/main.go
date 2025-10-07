package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
)

var logger = log.New(os.Stdout, "[ELM327-Bridge] ", log.LstdFlags|log.Lshortfile)

type Config struct {
	Elm327 struct {
		Mac string `mapstructure:"mac"`
	} `mapstructure:"elm327"`
	MQTT struct {
		Broker       string `mapstructure:"broker"`
		Username     string `mapstructure:"username"`
		Password     string `mapstructure:"password"`
		DataTopic    string `mapstructure:"data_topic"`
		CommandTopic string `mapstructure:"command_topic"`
	} `mapstructure:"mqtt"`
	Logging struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"logging"`
}

var config Config
var btConn io.ReadWriteCloser
var mqttClient mqtt.Client
var btMutex sync.Mutex
var reconnectInterval = 5 * time.Second

// isDebug возвращает true если уровень логирования установлен в debug
func isDebug() bool {
	return config.Logging.Level == "debug"
}

func loadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("error unmarshaling config: %v", err)
	}

	// Валидация конфигурации
	if err := validateConfig(); err != nil {
		return err
	}

	logger.SetFlags(log.LstdFlags)
	return nil
}

func validateConfig() error {
	// Проверка обязательных полей
	if config.Elm327.Mac == "" || config.Elm327.Mac == "XX:XX:XX:XX:XX:XX" {
		return fmt.Errorf("ELM327 MAC address must be set in config.yaml")
	}

	if config.MQTT.Broker == "" {
		return fmt.Errorf("MQTT broker address must be set in config.yaml")
	}

	if config.MQTT.DataTopic == "" {
		return fmt.Errorf("MQTT data topic must be set in config.yaml")
	}

	if config.MQTT.CommandTopic == "" {
		return fmt.Errorf("MQTT command topic must be set in config.yaml")
	}

	// Проверка учетных данных MQTT
	if (config.MQTT.Username == "" && config.MQTT.Password != "") ||
		(config.MQTT.Username != "" && config.MQTT.Password == "") {
		return fmt.Errorf("both MQTT username and password must be provided together, or both omitted for anonymous connection")
	}

	// Логирование режима подключения
	if config.MQTT.Username != "" && config.MQTT.Password != "" {
		logger.Printf("MQTT authentication: ENABLED")
	} else {
		logger.Printf("MQTT authentication: DISABLED (anonymous mode)")
	}

	return nil
}

func connectBluetooth() error {
	btMutex.Lock()
	defer btMutex.Unlock()

	if btConn != nil {
		btConn.Close()
	}

	logger.Printf("Attempting to open RFCOMM device /dev/rfcomm0")

	// Проверяем, существует ли /dev/rfcomm0
	if _, err := os.Stat("/dev/rfcomm0"); os.IsNotExist(err) {
		return fmt.Errorf("/dev/rfcomm0 does not exist. Please run 'sudo rfcomm bind 0 <MAC> <channel>' first")
	}

	// Подключаемся к /dev/rfcomm0 как к файлу последовательного порта
	f, err := os.OpenFile("/dev/rfcomm0", os.O_RDWR|unix.O_NOCTTY|os.O_SYNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open /dev/rfcomm0: %v. Check permissions and if device is connected", err)
	}

	btConn = f
	logger.Printf("Successfully opened /dev/rfcomm0")

	// Инициализация ELM327
	time.Sleep(500 * time.Millisecond)
	initCmd := []byte("ATZ\r")
	if _, err := f.Write(initCmd); err != nil {
		return fmt.Errorf("failed to send init command ATZ: %v", err)
	}

	logger.Printf("Sent init command ATZ to ELM327, waiting for response...")
	response := make([]byte, 128)
	n, readErr := f.Read(response)
	if readErr != nil {
		logger.Printf("Warning: No response from ELM327 after ATZ (err: %v). Device may not be ready.", readErr)
	} else if n > 0 {
		respStr := strings.TrimSpace(string(response[:n]))
		logger.Printf("ELM327 response to ATZ: %q", respStr)
		if strings.Contains(respStr, "ELM327") {
			logger.Printf("ELM327 initialized successfully")
		}
	} else {
		logger.Printf("Warning: Empty response from ELM327 after ATZ.")
	}

	return nil
}

func readFromBluetooth() {
	for {
		if btConn == nil {
			time.Sleep(reconnectInterval)
			continue
		}

		reader := bufio.NewReader(btConn)
		data, err := reader.ReadBytes('>') // ELM327 часто заканчивает ответ '>'
		if err != nil {
			logger.Printf("Bluetooth read error: %v", err)
			btConn.Close()
			btConn = nil
			time.Sleep(reconnectInterval)
			go attemptReconnectBluetooth()
			continue
		}

		// Удаляем trailing '>' если есть
		if len(data) > 0 && data[len(data)-1] == '>' {
			data = data[:len(data)-1]
		}

		if isDebug() {
			logger.Printf("[DEBUG] Received data from Bluetooth: %s (hex: %x)", string(data), data)
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		if isDebug() {
			logger.Printf("[DEBUG] Publishing to MQTT topic '%s', encoded payload: %s", config.MQTT.DataTopic, encoded)
		}

		if mqttClient != nil && mqttClient.IsConnected() {
			token := mqttClient.Publish(config.MQTT.DataTopic, 1, false, encoded)
			token.Wait()
			if token.Error() != nil {
				logger.Printf("MQTT publish error: %v", token.Error())
			} else if isDebug() {
				logger.Printf("[DEBUG] Successfully published data to MQTT")
			}
		}
	}
}

func writeToBluetooth(command []byte) error {
	btMutex.Lock()
	defer btMutex.Unlock()

	if btConn == nil {
		return fmt.Errorf("Bluetooth not connected")
	}

	_, err := btConn.Write(append(command, '\r'))
	if err != nil {
		logger.Printf("Bluetooth write error: %v", err)
		btConn.Close()
		btConn = nil
		go attemptReconnectBluetooth()
		return err
	}

	logger.Printf("Sent command to ELM327: %s", string(command))
	return nil
}

func attemptReconnectBluetooth() {
	for {
		if err := connectBluetooth(); err != nil {
			logger.Printf("Reconnecting Bluetooth in %v...", reconnectInterval)
			time.Sleep(reconnectInterval)
			continue
		}
		break
	}
}

func connectMQTT() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://" + config.MQTT.Broker)
	// Проверяем, что username и password заданы в конфигурации
	if config.MQTT.Username != "" && config.MQTT.Password != "" {
		opts.SetUsername(config.MQTT.Username)
		opts.SetPassword(config.MQTT.Password)
		logger.Printf("Connecting to MQTT with authentication")
	} else {
		logger.Printf("Connecting to MQTT anonymously (no credentials provided)")
	}
	opts.SetClientID("elm327-bridge-" + fmt.Sprintf("%d", time.Now().Unix()))
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		logger.Printf("Received message on topic: %s\nPayload: %s", msg.Topic(), msg.Payload())
	})
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		logger.Println("MQTT connected")
		token := client.Subscribe(config.MQTT.CommandTopic, 1, onCommandReceived)
		token.Wait()
		if token.Error() != nil {
			logger.Printf("MQTT subscribe error: %v", token.Error())
		}
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		logger.Printf("MQTT connection lost: %v", err)
		go attemptReconnectMQTT()
	})
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)

	mqttClient = mqtt.NewClient(opts)
	token := mqttClient.Connect()
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("MQTT connect error: %v", token.Error())
	}

	return nil
}

func onCommandReceived(client mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	if isDebug() {
		logger.Printf("[DEBUG] Received MQTT command on topic '%s', raw payload: %s", msg.Topic(), string(payload))
	}

	decoded, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		logger.Printf("Base64 decode error: %v", err)
		return
	}

	if isDebug() {
		logger.Printf("[DEBUG] Decoded MQTT command: %s (hex: %x)", string(decoded), decoded)
	}

	if err := writeToBluetooth(decoded); err != nil {
		logger.Printf("Failed to send command: %v", err)
	}
}

func attemptReconnectMQTT() {
	for {
		if err := connectMQTT(); err != nil {
			logger.Printf("Reconnecting MQTT in %v...", reconnectInterval)
			time.Sleep(reconnectInterval)
			continue
		}
		break
	}
}

func main() {
	if err := loadConfig(); err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Initial connections
	go attemptReconnectBluetooth()
	time.Sleep(2 * time.Second) // Wait for BT connect

	if err := connectMQTT(); err != nil {
		logger.Fatalf("Failed to connect MQTT: %v", err)
	}

	// Start reading from Bluetooth
	go readFromBluetooth()

	// Keep alive
	logger.Println("ELM327 Bridge started. Press Ctrl+C to stop.")
	select {}
}
