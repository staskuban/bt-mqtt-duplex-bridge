package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
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
var btConn net.Conn
var mqttClient mqtt.Client
var btMutex sync.Mutex
var reconnectInterval = 5 * time.Second

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

	logger.SetFlags(log.LstdFlags)
	// Уровень логирования можно настроить, но для простоты используем info

	return nil
}

func connectBluetooth(addr string) error {
	btMutex.Lock()
	defer btMutex.Unlock()

	if btConn != nil {
		btConn.Close()
	}

	// Примечание: Для реального Bluetooth RFCOMM на Linux/RPi рекомендуется использовать rfcomm bind для создания /dev/rfcomm0,
	// затем dial("unix", "/dev/rfcomm0"). Здесь используется упрощенный net.Dial для демонстрации.
	// Альтернатива: Использовать github.com/muka/go-bluetooth для прямого API.
	conn, err := net.DialTimeout("tcp", addr+":1", 10*time.Second) // RFCOMM channel 1, timeout
	if err != nil {
		logger.Printf("Bluetooth connect error: %v", err)
		return err
	}

	btConn = conn
	logger.Printf("Connected to Bluetooth device: %s", addr)
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

		encoded := base64.StdEncoding.EncodeToString(data)
		if mqttClient != nil && mqttClient.IsConnected() {
			token := mqttClient.Publish(config.MQTT.DataTopic, 1, false, encoded)
			token.Wait()
			if token.Error() != nil {
				logger.Printf("MQTT publish error: %v", token.Error())
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
	addr := config.Elm327.Mac
	for {
		if err := connectBluetooth(addr); err != nil {
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
	if config.MQTT.Username != "" && config.MQTT.Password != "" {
		opts.SetUsername(config.MQTT.Username)
		opts.SetPassword(config.MQTT.Password)
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
	decoded, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		logger.Printf("Base64 decode error: %v", err)
		return
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

	addr := config.Elm327.Mac
	if addr == "XX:XX:XX:XX:XX:XX" {
		logger.Fatal("Please set the ELM327 MAC address in config.yaml")
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
