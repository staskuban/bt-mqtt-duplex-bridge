package obd

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"elm327-bridge/common"
)

var logger = log.New(os.Stdout, "[OBD-Parser] ", log.LstdFlags|log.Lshortfile)

// Telemetry представляет декодированные данные телеметрии (используем общий тип)
type Telemetry = common.Telemetry

// CommandResponse представляет ответ на команду (используем общий тип)
type CommandResponse = common.CommandResponse

// PIDDecoder представляет функцию для декодирования конкретного PID
type PIDDecoder func(data []byte) (float64, error)

// pidDecoders содержит декодеры для различных PID
var pidDecoders = map[string]PIDDecoder{
	// Двигатель и производительность
	"0C": decodeRPM,          // Обороты двигателя (Engine RPM)
	"0D": decodeVehicleSpeed, // Скорость автомобиля (Vehicle Speed)
	"05": decodeCoolantTemp,  // Температура охлаждающей жидкости (Engine Coolant Temperature)
	"0F": decodeIntakeTemp,   // Температура всасываемого воздуха (Intake Air Temperature)
	"11": decodeThrottlePos,  // Положение дроссельной заслонки (Throttle Position)
	"04": decodeEngineLoad,   // Нагрузка двигателя (Calculated Engine Load)

	// Топливо и эффективность
	"2F": decodeFuelLevel,          // Уровень топлива (Fuel Level Input)
	"0A": decodeFuelPressure,       // Давление топлива (Fuel Pressure)
	"06": decodeShortTermFuelTrim1, // Короткий срок корректировки топлива Bank 1
	"07": decodeLongTermFuelTrim1,  // Длинный срок корректировки топлива Bank 1

	// Давление и температура
	"0B": decodeIntakePressure,     // Давление во впускном коллекторе (Intake Manifold Pressure)
	"33": decodeBarometricPressure, // Барометрическое давление (Barometric Pressure)

	// Диагностика
	"01": decodeMonitorStatus,   // Статус мониторинга DTC
	"21": decodeDistanceWithMIL, // Расстояние с включенным MIL
}

// metricNames содержит человеко-читаемые названия метрик
var metricNames = map[string]string{
	"0C": "engine_rpm",
	"0D": "vehicle_speed",
	"05": "coolant_temperature",
	"0F": "intake_air_temperature",
	"11": "throttle_position",
	"04": "engine_load",
	"2F": "fuel_level",
	"0A": "fuel_pressure",
	"06": "short_term_fuel_trim_1",
	"07": "long_term_fuel_trim_1",
	"0B": "intake_manifold_pressure",
	"33": "barometric_pressure",
	"01": "monitor_status",
	"21": "distance_with_mil",
}

// metricUnits содержит единицы измерения
var metricUnits = map[string]string{
	"0C": "rpm",
	"0D": "km/h",
	"05": "°C",
	"0F": "°C",
	"11": "%",
	"04": "%",
	"2F": "%",
	"0A": "kPa",
	"06": "%",
	"07": "%",
	"0B": "kPa",
	"33": "kPa",
	"01": "status",
	"21": "km",
}

// Декодеры для конкретных PID

// decodeRPM декодирует обороты двигателя (PID 0C)
// Формула: ((A * 256) + B) / 4
func decodeRPM(data []byte) (float64, error) {
	if len(data) != 2 {
		return 0, fmt.Errorf("PID 0C: ожидалось 2 байта, получено %d", len(data))
	}
	A := float64(data[0])
	B := float64(data[1])
	return ((A * 256) + B) / 4, nil
}

// decodeVehicleSpeed декодирует скорость автомобиля (PID 0D)
// Формула: A
func decodeVehicleSpeed(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 0D: ожидался 1 байт, получено %d", len(data))
	}
	return float64(data[0]), nil
}

// decodeCoolantTemp декодирует температуру охлаждающей жидкости (PID 05)
// Формула: A - 40
func decodeCoolantTemp(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 05: ожидался 1 байт, получено %d", len(data))
	}
	return float64(data[0]) - 40, nil
}

// decodeIntakeTemp декодирует температуру всасываемого воздуха (PID 0F)
// Формула: A - 40
func decodeIntakeTemp(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 0F: ожидался 1 байт, получено %d", len(data))
	}
	return float64(data[0]) - 40, nil
}

// decodeThrottlePos декодирует положение дроссельной заслонки (PID 11)
// Формула: (A * 100) / 255
func decodeThrottlePos(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 11: ожидался 1 байт, получено %d", len(data))
	}
	return (float64(data[0]) * 100) / 255, nil
}

// decodeEngineLoad декодирует нагрузку двигателя (PID 04)
// Формула: (A * 100) / 255
func decodeEngineLoad(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 04: ожидался 1 байт, получено %d", len(data))
	}
	return (float64(data[0]) * 100) / 255, nil
}

// decodeFuelLevel декодирует уровень топлива (PID 2F)
// Формула: (A * 100) / 255
func decodeFuelLevel(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 2F: ожидался 1 байт, получено %d", len(data))
	}
	return (float64(data[0]) * 100) / 255, nil
}

// decodeFuelPressure декодирует давление топлива (PID 0A)
// Формула: A * 3
func decodeFuelPressure(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 0A: ожидался 1 байт, получено %d", len(data))
	}
	return float64(data[0]) * 3, nil
}

// decodeShortTermFuelTrim1 декодирует короткий срок корректировки топлива Bank 1 (PID 06)
// Формула: (A - 128) * 100 / 128
func decodeShortTermFuelTrim1(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 06: ожидался 1 байт, получено %d", len(data))
	}
	return (float64(data[0]) - 128) * 100 / 128, nil
}

// decodeLongTermFuelTrim1 декодирует длинный срок корректировки топлива Bank 1 (PID 07)
// Формула: (A - 128) * 100 / 128
func decodeLongTermFuelTrim1(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 07: ожидался 1 байт, получено %d", len(data))
	}
	return (float64(data[0]) - 128) * 100 / 128, nil
}

// decodeIntakePressure декодирует давление во впускном коллекторе (PID 0B)
// Формула: A
func decodeIntakePressure(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 0B: ожидался 1 байт, получено %d", len(data))
	}
	return float64(data[0]), nil
}

// decodeBarometricPressure декодирует барометрическое давление (PID 33)
// Формула: A
func decodeBarometricPressure(data []byte) (float64, error) {
	if len(data) != 1 {
		return 0, fmt.Errorf("PID 33: ожидался 1 байт, получено %d", len(data))
	}
	return float64(data[0]), nil
}

// decodeMonitorStatus декодирует статус мониторинга (PID 01)
// Это битовая карта, возвращаем как сырое значение
func decodeMonitorStatus(data []byte) (float64, error) {
	if len(data) != 4 {
		return 0, fmt.Errorf("PID 01: ожидалось 4 байта, получено %d", len(data))
	}
	// Конвертируем 4 байта в одно число для простоты
	return float64(data[0])*256*256*256 + float64(data[1])*256*256 + float64(data[2])*256 + float64(data[3]), nil
}

// decodeDistanceWithMIL декодирует расстояние с включенным MIL (PID 21)
// Формула: (A * 256) + B
func decodeDistanceWithMIL(data []byte) (float64, error) {
	if len(data) != 2 {
		return 0, fmt.Errorf("PID 21: ожидалось 2 байта, получено %d", len(data))
	}
	return float64(data[0])*256 + float64(data[1]), nil
}

// ParseResponse разбирает сырой ответ от ELM327
func ParseResponse(response string) (*Telemetry, error) {
	// Очищаем ответ от лишних символов
	response = strings.TrimSpace(response)

	// Проверяем формат ответа ELM327 (должен начинаться с 4x)
	if len(response) < 5 || !strings.HasPrefix(response, "4") {
		return nil, fmt.Errorf("invalid response format: %s", response)
	}

	// Разбираем ответ: формат "41 0C 1A F0"
	parts := strings.Fields(response)
	if len(parts) < 3 {
		return nil, fmt.Errorf("response too short: %s", response)
	}

	// Извлекаем компоненты
	echo := parts[0]       // "41" (эхо сервиса)
	pid := parts[1]        // "0C" (PID)
	dataParts := parts[2:] // Данные

	// Проверяем эхо (должен быть "4x" где x - сервис)
	if len(echo) != 2 || echo[0] != '4' {
		return nil, fmt.Errorf("invalid echo format: %s", echo)
	}

	// Проверяем PID
	if len(pid) != 2 {
		return nil, fmt.Errorf("invalid PID format: %s", pid)
	}

	// Конвертируем данные из hex в байты
	data := make([]byte, len(dataParts))
	for i, part := range dataParts {
		val, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hex data %s: %v", part, err)
		}
		data[i] = byte(val)
	}

	// Декодируем данные
	decoder, exists := pidDecoders[pid]
	if !exists {
		return nil, fmt.Errorf("unsupported PID: %s", pid)
	}

	value, err := decoder(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PID %s: %v", pid, err)
	}

	// Получаем метаданные
	metric, exists := metricNames[pid]
	if !exists {
		metric = "unknown_" + pid
	}

	unit, exists := metricUnits[pid]
	if !exists {
		unit = "unknown"
	}

	// Создаем структуру телеметрии
	telemetry := &Telemetry{
		PID:       pid,
		Metric:    metric,
		Value:     value,
		Unit:      unit,
		Timestamp: getCurrentTimestamp(),
		Raw:       response,
	}

	logger.Printf("Parsed telemetry: %s = %.2f %s", metric, value, unit)
	return telemetry, nil
}

// getCurrentTimestamp возвращает текущий Unix timestamp
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// GetSupportedPIDs возвращает список поддерживаемых PID
func GetSupportedPIDs() []string {
	pids := make([]string, 0, len(pidDecoders))
	for pid := range pidDecoders {
		pids = append(pids, pid)
	}
	return pids
}

// GetMetricName возвращает название метрики для PID
func GetMetricName(pid string) string {
	if name, exists := metricNames[pid]; exists {
		return name
	}
	return "unknown_" + pid
}

// GetMetricUnit возвращает единицу измерения для PID
func GetMetricUnit(pid string) string {
	if unit, exists := metricUnits[pid]; exists {
		return unit
	}
	return "unknown"
}

// StartParser запускает горутину для парсинга ответов от ELM327
func StartParser(responsesChan <-chan string, telemetryChan chan<- interface{}, commandResponsesChan chan CommandResponse) {
	logger := log.New(os.Stdout, "[OBD-Parser] ", log.LstdFlags|log.Lshortfile)
	logger.Println("Starting OBD parser")

	for {
		select {
		case response, ok := <-responsesChan:
			if !ok {
				logger.Println("Responses channel closed")
				return
			}

			// Парсим ответ
			telemetry, err := ParseResponse(response)
			if err != nil {
				logger.Printf("Failed to parse response %q: %v", response, err)
				continue
			}

			// Отправляем в канал телеметрии
			select {
			case telemetryChan <- telemetry:
				logger.Printf("Telemetry sent: %s = %.2f %s", telemetry.Metric, telemetry.Value, telemetry.Unit)
			default:
				logger.Printf("Warning: telemetry channel is full, dropping: %s", telemetry.Metric)
			}
		}
	}
}

// StartCommandManager запускает менеджер команд для периодического опроса PID
func StartCommandManager(commandsChan chan<- string) {
	logger := log.New(os.Stdout, "[OBD-CommandManager] ", log.LstdFlags|log.Lshortfile)
	logger.Println("Starting command manager")

	// Список PID для периодического опроса
	pids := []string{"0C", "0D", "05", "0F", "11", "04", "2F", "0A", "0B", "33"}

	ticker := time.NewTicker(5 * time.Second) // Опрос каждые 5 секунд
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Отправляем команды для опроса PID
			for _, pid := range pids {
				command := fmt.Sprintf("01%s", pid) // Сервис 01 + PID

				select {
				case commandsChan <- command:
					logger.Printf("Sent command: %s", command)
				default:
					logger.Printf("Warning: commands channel is full, skipping: %s", command)
				}

				time.Sleep(100 * time.Millisecond) // Пауза между командами
			}
		}
	}
}
