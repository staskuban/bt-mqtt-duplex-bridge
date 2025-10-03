# ELM327 Bridge для Raspberry Pi (Golang версия)

## Обзор
Этот проект реализует мост между Bluetooth-устройством ELM327 (OBD-II сканер) и удаленным MQTT-брокером. Данные от ELM327 передаются в MQTT в сыром виде (base64-encoded). Команды из MQTT отправляются в ELM327 через Bluetooth как прямые команды. Приложение написано на Go, компилируется в бинарный файл и предназначено для развертывания на Raspberry Pi 5 с Raspberry Pi OS.

Проект использует:
- Bluetooth (RFCOMM/SPP) для связи с ELM327.
- MQTT-клиент для публикации/подписки на топики.
- Конфигурацию через YAML-файл.

Подробная архитектурная документация доступна в [architecture_pi.md](../architecture_pi.md).

## Предварительные требования
- **Raspberry Pi 5** с установленной Raspberry Pi OS (64-bit).
- Go 1.21+ (для разработки и компиляции).
- Bluetooth-адаптер на Pi (встроенный).
- Установленный ELM327-сканер, сопряженный с Pi.
- Доступ к удаленному MQTT-брокеру (например, Mosquitto).
- Docker и Docker Compose (для контейнеризированного запуска и тестирования).

### Системные зависимости на Raspberry Pi
Установите Go и BlueZ:
```
sudo apt update
wget https://go.dev/dl/go1.21.5.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-arm64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

sudo apt install bluez libbluetooth-dev docker.io docker-compose
sudo systemctl start bluetooth
sudo systemctl enable bluetooth
sudo usermod -aG docker $USER  # Для запуска Docker без sudo
```

Сопрягите ELM327 с Pi:
```
bluetoothctl
scan on
pair XX:XX:XX:XX:XX:XX  # MAC-адрес ELM327
trust XX:XX:XX:XX:XX:XX
connect XX:XX:XX:XX:XX:XX
exit
```

## Установка
1. Клонируйте или скопируйте проект в директорию `pi-bridge/`.
2. Инициализируйте Go-модуль и установите зависимости:
   ```
   cd pi-bridge
   go mod tidy
   ```
   Это установит:
   - `github.com/eclipse/paho.mqtt.golang` (MQTT).
   - `github.com/spf13/viper` (конфигурация).
   - `github.com/muka/go-bluetooth` (Bluetooth, опционально).
   - `github.com/stretchr/testify` (тесты).

3. Создайте конфигурационный файл `config.yaml` (пример ниже):
   ```
   elm327:
     mac: "XX:XX:XX:XX:XX:XX"  # Замените на реальный MAC ELM327
   mqtt:
     broker: "mqtt.example.com:1883"  # Адрес вашего MQTT-брокера
     username: "user"  # Опционально
     password: "pass"  # Опционально
     data_topic: "elm327/data"  # Топик для публикации данных от ELM327
     command_topic: "elm327/command"  # Топик для команд в ELM327
   logging:
     level: "info"
   ```

## Docker-конфигурация
Для запуска в контейнере (рекомендуется для изоляции и кросс-платформенности) создайте `Dockerfile` в `pi-bridge/`:
```
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o elm327-bridge main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates bluetooth bluez
WORKDIR /root/
COPY --from=builder /app/elm327-bridge .
COPY config.yaml .
CMD ["./elm327-bridge"]
```

Создайте `docker-compose.yml` в `pi-bridge/`:
```
version: '3.8'
services:
  elm327-bridge:
    build: .
    container_name: elm327-bridge
    privileged: true  # Для доступа к Bluetooth
    devices:
      - /dev:/dev  # Доступ к Bluetooth-устройствам
    volumes:
      - ./config.yaml:/root/config.yaml
    restart: unless-stopped
    environment:
      - TZ=Europe/Moscow
```

## Запуск
### Вариант 1: Прямой запуск (без Docker)
1. Скомпилируйте бинарник:
   ```
   cd pi-bridge
   GOOS=linux GOARCH=arm64 go build -o elm327-bridge main.go
   ```
2. Запустите (требует sudo для Bluetooth):
   ```
   sudo ./elm327-bridge
   ```
   Приложение подключится к ELM327 и MQTT, начнет чтение данных и подписку на команды.

### Вариант 2: Запуск с Docker (рекомендуется)
1. Соберите и запустите:
   ```
   cd pi-bridge
   docker-compose up --build -d
   ```
2. Проверьте логи:
   ```
   docker-compose logs -f
   ```
   Контейнер будет автоматически перезапускаться при ошибках.

Приложение логирует подключения и ошибки в stdout. Остановите: `Ctrl+C` или `docker-compose down`.

## Тестирование
### Unit-тесты
Запустите тесты для Bluetooth и MQTT:
```
cd pi-bridge
go test ./... -v
```
- `bluetooth_test.go`: Тестирует запись команд, base64-кодирование/декодирование.
- `mqtt_test.go`: Тестирует обработку команд из MQTT, публикацию данных.

Пример вывода:
```
=== RUN   TestWriteToBluetooth
--- PASS: TestWriteToBluetooth (0.00s)
PASS
ok      elm327-bridge   0.002s
```

### Интеграционные тесты
1. Запустите приложение (Docker или напрямую).
2. Отправьте тестовую команду в MQTT (используйте `mosquitto_pub` или MQTT-клиент):
   ```
   mosquitto_pub -h mqtt.example.com -t elm327/command -m "$(echo -n 'ATZ' | base64)"  # Reset ELM327
   ```
   Ожидаемый ответ: В логах увидите "Sent command to ELM327: ATZ", а от ELM327 придет reset-ответ (публикуется в `elm327/data`).

3. Проверьте чтение данных: Подключите ELM327 к автомобилю, запросите PID (например, RPM: "010C").
   - Команда из MQTT: `mosquitto_pub -h mqtt.example.com -t elm327/command -m "$(echo -n '010C' | base64)"`
   - Данные вернутся в `elm327/data` как base64 (декодируйте для проверки: e.g., "41 0C ...").

4. Мониторинг:
   - Логи: `docker-compose logs` или `tail -f /var/log/elm327-bridge.log` (если настроено).
   - MQTT: Используйте `mosquitto_sub -h mqtt.example.com -t elm327/data` для прослушки данных.
   - Bluetooth: `bluetoothctl info XX:XX:XX:XX:XX:XX` для проверки соединения.

### Возможные проблемы и отладка
- **Bluetooth не подключается**: Проверьте pairing (`bluetoothctl`), убедитесь в trust. В Docker используйте `privileged: true`.
- **Partial reads**: ELM327 может слать данные по частям; приложение буферизует до '>'. Если проблемы, увеличьте таймауты в коде.
- **MQTT ошибки**: Проверьте firewall/NAT на Pi, credentials в config.yaml.
- **Docker на Pi**: Убедитесь, что Docker установлен для arm64. Если cross-compile с Mac: используйте `docker buildx` для arm64.
- **Логи**: Уровень "debug" в config.yaml для детализации.

## Архитектура
Подробный план в [architecture_pi.md](../architecture_pi.md), включая диаграммы, потоки данных и примеры кода.

## Лицензия
MIT License (или укажите вашу).

Для вопросов или улучшений: обратитесь к архитектуре или исходному коду.