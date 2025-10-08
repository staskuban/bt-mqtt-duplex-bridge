# ELM327 Bridge

Мощный и надежный шлюз для двусторонней связи между ELM327 OBD-II адаптером и MQTT брокером. Разработан специально для Raspberry Pi с использованием Go.

## Архитектура

Приложение состоит из модульной архитектуры с разделением ответственности:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Bluetooth      │    │  OBD Parser     │    │  MQTT Client    │
│  Adapter        │◄──►│                 │◄──►│                 │
│                 │    │  - PID decoding │    │  - Data pub     │
│  - Connection   │    │  - Telemetry    │    │  - Command sub  │
│  - Auto-recon   │    │  - Error handle │    │  - Auto-recon   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Возможности

### ✅ Надежное подключение к Bluetooth
- Автоматическое переподключение при потере связи
- Правильная инициализация ELM327
- Настраиваемые таймауты и интервалы

### ✅ Декодирование OBD-II PID
- Поддержка популярных PID (RPM, скорость, температура, давление и др.)
- Правильные формулы преобразования из Википедии
- Расширяемая архитектура для добавления новых PID

### ✅ MQTT интеграция
- Публикация данных телеметрии в реальном времени
- Подписка на команды для отправки в автомобиль
- Стандартизированная структура топиков

### ✅ Двусторонняя связь
- Получение команд через MQTT и отправка в ELM327
- Ответы на команды с correlation_id для сопоставления
- Обработка ошибок и таймаутов

## Установка и настройка

### 1. Подготовка оборудования

Убедитесь, что ваш ELM327 адаптер подключен через Bluetooth:

```bash
# Проверка Bluetooth адаптера
sudo systemctl status bluetooth

# Поиск и сопряжение с ELM327
sudo bluetoothctl
# В интерфейсе bluetoothctl:
power on
agent on
scan on
# Найдите ваш ELM327 (обычно называется "OBDII")
pair <MAC_ADDRESS>
trust <MAC_ADDRESS>
exit

# Создание RFCOMM устройства
sudo rfcomm bind rfcomm0 <MAC_ADDRESS> 1

# Проверка устройства
ls -l /dev/rfcomm0
```

### 2. Конфигурация

Скопируйте пример конфигурации и настройте параметры:

```bash
cp config.example.yaml config.yaml
# Отредактируйте config.yaml в соответствии с вашими настройками
```

Минимальная конфигурация:

```yaml
mqtt:
  broker: "tcp://your-mqtt-broker:1883"
  data_topic: "car/telemetry"
  command_topic: "car/command"
```

### 3. Сборка и запуск

```bash
# Сборка приложения
make build

# Проверка готовности к развертыванию
make ready

# Деплой на Raspberry Pi (требуется SSH ключ)
make deploy-pi

# Полная пересборка и деплой
make deploy-pi-full

# Альтернативно: ручная сборка и запуск
go build -o elm327-bridge .
sudo ./elm327-bridge
```

## Структура топиков MQTT

### Данные телеметрии
```
car/telemetry/{VIN}/engine_rpm
car/telemetry/{VIN}/vehicle_speed
car/telemetry/{VIN}/coolant_temperature
car/telemetry/{VIN}/fuel_level
...
```

**Формат сообщения:**
```json
{
  "vin": "ABC123XYZ",
  "pid": "0C",
  "metric": "engine_rpm",
  "value": 1724,
  "unit": "rpm",
  "timestamp": "2025-10-08T00:28:56Z",
  "raw": "41 0C 1A F0"
}
```

### Команды
```
car/command/{VIN}/request      # Входящие команды
car/command/{VIN}/response     # Ответы на команды
```

**Формат команды:**
```json
{
  "command": "010C",
  "correlation_id": "cmd-123",
  "description": "Запрос оборотов двигателя",
  "vin": "ABC123XYZ"
}
```

**Формат ответа:**
```json
{
  "correlation_id": "cmd-123",
  "status": "success",
  "result": "41 0C 1A F0",
  "timestamp": "2025-10-08T00:28:56Z"
}
```

## Поддерживаемые PID

| PID | Описание | Единица |
|-----|----------|---------|
| 0C  | Обороты двигателя | rpm |
| 0D  | Скорость автомобиля | km/h |
| 05  | Температура охлаждающей жидкости | °C |
| 0F  | Температура всасываемого воздуха | °C |
| 11  | Положение дроссельной заслонки | % |
| 04  | Нагрузка двигателя | % |
| 2F  | Уровень топлива | % |
| 0A  | Давление топлива | kPa |
| 0B  | Давление во впускном коллекторе | kPa |
| 33  | Барометрическое давление | kPa |
| 01  | Статус мониторинга DTC | status |
| 21  | Расстояние с включенным MIL | km |

## Развертывание

### Автоматическое развертывание на Raspberry Pi

```bash
# Проверка готовности
make ready

# Деплой бинарного файла
make deploy-pi

# Полная пересборка и деплой
make deploy-pi-full

# Синхронизация всего проекта
make sync-pi
```

### Настройка на Raspberry Pi

После деплоя на Raspberry Pi:

```bash
# Настройка Bluetooth соединения
./setup-bluetooth.sh

# Настройка автозапуска
sudo systemctl enable elm327-bridge
sudo systemctl start elm327-bridge

# Проверка статуса
sudo systemctl status elm327-bridge
```

### Docker развертывание

```bash
# Сборка образов
make docker-build

# Запуск с MQTT брокером
make deploy

# Мониторинг
make logs
```

## Разработка и тестирование

### Запуск тестов

```bash
# Тесты всех модулей
make test

# Тесты конкретного модуля
go test ./bluetooth -v
go test ./obd -v
go test ./mqtt -v
```

### Доступные команды

```bash
make help        # Показать все команды
make build       # Сборка приложения
make clean       # Очистка артефактов
make fmt         # Форматирование кода
make vet         # Статический анализ
make status      # Статус проекта
```

### Архитектура модулей

- **`bluetooth/`** - Работа с последовательным портом и ELM327
- **`obd/`** - Парсинг ответов и декодирование PID
- **`mqtt/`** - MQTT клиент для публикации/подписки
- **`common/`** - Общие типы данных

### Добавление нового PID

1. Добавьте декодер в `obd/parser.go`:
```go
func decodeNewPID(data []byte) (float64, error) {
    // Ваша формула декодирования
    return float64(data[0]), nil
}
```

2. Зарегистрируйте в `pidDecoders`:
```go
"XX": decodeNewPID,
```

3. Добавьте метаданные:
```go
metricNames["XX"] = "new_metric"
metricUnits["XX"] = "unit"
```

## Производительность

- **Потребление памяти:** ~10-20 MB
- **CPU:** Минимальное (асинхронная обработка)
- **Задержка:** < 100ms от ELM327 до MQTT
- **Надежность:** Автоматическое переподключение при сбоях

## Лицензия

MIT License - см. файл LICENSE для подробностей.