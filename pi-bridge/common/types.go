package common

import "time"

// Telemetry представляет декодированные данные телеметрии
type Telemetry struct {
	PID       string  `json:"pid"`       // PID код (например, "0C")
	Metric    string  `json:"metric"`    // Название метрики (например, "rpm")
	Value     float64 `json:"value"`     // Декодированное значение
	Unit      string  `json:"unit"`      // Единица измерения (например, "rpm")
	Timestamp int64   `json:"timestamp"` // Unix timestamp
	Raw       string  `json:"raw"`       // Сырые данные для отладки
}

// CommandMessage представляет входящую команду
type CommandMessage struct {
	Command       string `json:"command"`        // AT команда для отправки в ELM327
	CorrelationID string `json:"correlation_id"` // ID для сопоставления запроса и ответа
	Description   string `json:"description"`    // Описание команды
	VIN           string `json:"vin"`            // VIN автомобиля
}

// CommandResponse представляет ответ на команду
type CommandResponse struct {
	CorrelationID string      `json:"correlation_id"`
	Status        string      `json:"status"`          // "success", "error"
	Result        interface{} `json:"result"`          // Результат выполнения команды
	Error         string      `json:"error,omitempty"` // Описание ошибки если статус "error"
	Timestamp     time.Time   `json:"timestamp"`
}
