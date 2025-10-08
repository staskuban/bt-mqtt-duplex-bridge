package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"elm327-bridge/bluetooth"
	"elm327-bridge/mqtt"
	"elm327-bridge/obd"

	"github.com/spf13/viper"
)

var logger = log.New(os.Stdout, "[ELM327-Bridge] ", log.LstdFlags|log.Lshortfile)

type Config struct {
	Bluetooth bluetooth.Config `mapstructure:"bluetooth"`
	MQTT      mqtt.Config      `mapstructure:"mqtt"`
	Logging   struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"logging"`
}

var config Config

// loadConfig загружает конфигурацию из файла config.yaml
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

	return nil
}

// validateConfig проверяет корректность конфигурации
func validateConfig() error {
	if config.Bluetooth.DevicePath == "" {
		config.Bluetooth = bluetooth.DefaultConfig()
		logger.Println("Using default Bluetooth configuration")
	}

	if config.MQTT.Broker == "" {
		return fmt.Errorf("MQTT broker address must be set in config.yaml")
	}

	return nil
}

// main функция приложения
func main() {
	logger.Println("Starting ELM327 Bridge...")

	// Загружаем конфигурацию
	if err := loadConfig(); err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Создаем каналы для связи между модулями
	responsesChan := make(chan string, 50)                     // Сырые ответы от ELM327
	commandsChan := make(chan string, 20)                      // Команды для отправки в ELM327
	telemetryChan := make(chan interface{}, 100)               // Декодированные данные телеметрии
	commandResponsesChan := make(chan obd.CommandResponse, 50) // Ответы на команды

	// Создаем и запускаем Bluetooth адаптер
	btAdapter := bluetooth.NewAdapter(config.Bluetooth, responsesChan, commandsChan)
	if err := btAdapter.Start(); err != nil {
		logger.Fatalf("Failed to start Bluetooth adapter: %v", err)
	}

	// Создаем и запускаем парсер OBD
	go obd.StartParser(responsesChan, telemetryChan, commandResponsesChan)

	// Создаем и запускаем MQTT клиента
	mqttClient := mqtt.NewClient(config.MQTT, telemetryChan, commandsChan, commandResponsesChan)
	if err := mqttClient.Start(); err != nil {
		logger.Fatalf("Failed to start MQTT client: %v", err)
	}

	// Запускаем менеджер команд для периодического опроса PID
	go obd.StartCommandManager(commandsChan)

	logger.Println("ELM327 Bridge started successfully")
	logger.Println("Press Ctrl+C to stop")

	// Ожидаем сигнал завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	logger.Println("Shutting down...")

	// Останавливаем все модули
	btAdapter.Stop()
	mqttClient.Stop()

	logger.Println("ELM327 Bridge stopped")
}
