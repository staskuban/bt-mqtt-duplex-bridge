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
  - Для анонимного доступа: брокер должен разрешать подключения без аутентификации.
  - Для аутентифицированного доступа: создайте пользователя с соответствующими правами.
- Docker и Docker Compose (для контейнеризированного запуска и тестирования).

### Системные зависимости на Raspberry Pi (Raspberry Pi OS или Ubuntu Server 24.04)
Установите Go, BlueZ и Docker. Инструкции аналогичны для Ubuntu Server 24.04 (ARM64), используйте `apt` для установки.

Установите Go (опционально, если компилируете на Pi; для запуска бинарника не нужно):
```
sudo apt update
wget https://go.dev/dl/go1.23.0.linux-arm64.tar.gz  # Актуальная версия
sudo tar -C /usr/local -xzf go1.23.0.linux-arm64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
source /etc/profile
```

Установите BlueZ (Bluetooth stack):
```
sudo apt install -y bluez libbluetooth-dev bluetooth pi-bluetooth  # pi-bluetooth опционально для Pi OS
sudo systemctl start bluetooth
sudo systemctl enable bluetooth
``` 
 
Для Ubuntu Server 24.04:
- BlueZ уже включен, но убедитесь в обновлении: `sudo apt upgrade bluez`.
- Если проблемы с Bluetooth, установите дополнительные пакеты: `sudo apt install rfcomm bluez-tools`.

**Важно: Настройте RFCOMM перед запуском**
  

Приложение **не управляет** `rfcomm` напрямую. Вы должны настроить его самостоятельно на хост-системе (Raspberry Pi).

1.  **Сопряжение и подключение:**
    ```bash
    bluetoothctl
    scan on
    # Найдите MAC-адрес вашего ELM327
    pair XX:XX:XX:XX:XX:XX
    trust XX:XX:XX:XX:XX:XX
    connect XX:XX:XX:XX:XX:XX
    exit
    ```

2.  **Привязка к RFCOMM:**
    Для ELM327-устройств канал последовательного порта (SPP) почти всегда **1**.

    Привяжите устройство к `/dev/rfcomm0`, используя канал 1:
    ```bash
    sudo rfcomm bind 0 XX:XX:XX:XX:XX:XX 1
    ```
    Это создаст устройство `/dev/rfcomm0`.

    Если привязка не удалась или устройство не отвечает, вы можете попробовать найти правильный канал с помощью `sdptool`. Однако этот инструмент может работать нестабильно.
    ```bash
    # Эта команда может завершиться с ошибкой "Host is down", даже если устройство подключено.
    sdptool search --bdaddr XX:XX:XX:XX:XX:XX SP
    ```

3.  **Проверка:**
    ```bash
    # Убедитесь, что устройство существует
    ls -l /dev/rfcomm0

    # Попробуйте отправить команду вручную
    echo "ATZ\r" > /dev/rfcomm0 && cat /dev/rfcomm0
    ```
    Вы должны увидеть ответ от ELM327.

**Автоматизация:** Для автоматической привязки при загрузке, добавьте команду `rfcomm bind` в `/etc/rc.local` или создайте `systemd-unit`.

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
     # Для анонимного подключения закомментируйте или удалите username и password:
     # username: "user"  # Опционально: закомментируйте для анонимного подключения
     # password: "pass"  # Опционально: закомментируйте для анонимного подключения
     data_topic: "elm327/data"  # Топик для публикации данных от ELM327
     command_topic: "elm327/command"  # Топик для команд в ELM327
   logging:
     level: "info"  # Установите в "debug" для детального логирования MQTT команд и ответов Bluetooth
   ```

   **Анонимное подключение к MQTT:**
   - Если ваш MQTT-брокер позволяет анонимные подключения, просто закомментируйте или удалите строки `username` и `password` в конфигурационном файле.
   - Приложение автоматически определит режим подключения и подключится без аутентификации.
   - Убедитесь, что ваш MQTT-брокер настроен для разрешения анонимных подключений (обычно через настройки ACL или конфигурацию брокера).

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
RUN apk --no-cache add ca-certificates bluez bluez-libs rfcomm
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
      - /dev:/dev  # Доступ к Bluetooth-устройствам и rfcomm
    volumes:
      - ./config.yaml:/root/config.yaml
    restart: unless-stopped
    environment:
      - TZ=Europe/Moscow
      # - ELM327_MAC=XX:XX:XX:XX:XX:XX  # Если bind в контейнере
```

**Примечание для Docker**: Если rfcomm bind выполняется внутри контейнера, создайте entrypoint.sh:
```
#!/bin/sh
rfcomm bind 0 $ELM327_MAC 1
exec ./elm327-bridge
```
Добавьте в Dockerfile: COPY entrypoint.sh . && chmod +x entrypoint.sh && CMD ["./entrypoint.sh"]. Но рекомендуется bind на хосте.

## Запуск
### Вариант 1: Прямой запуск (без Docker)
1. **Обязательно выполните rfcomm bind** (см. выше).
2. Скомпилируйте бинарник:
   ```
   cd pi-bridge
   GOOS=linux GOARCH=arm64 go build -o elm327-bridge main.go
   ```
3. Запустите (требует sudo для Bluetooth, если не в Docker):
   ```
   sudo ./elm327-bridge
   ```
   Приложение подключится к ELM327 и MQTT, начнет чтение данных и подписку на команды.

### Вариант 2: Запуск с Docker (рекомендуется)
1. **Выполните rfcomm bind на хосте** (Raspberry Pi).
2. Соберите и запустите:
   ```
   cd pi-bridge
   docker-compose up --build -d
   ```
3. Проверьте логи:
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
1. Запустите приложение (Docker или напрямую). Убедитесь, что rfcomm bind выполнен и в логах "Connected to Bluetooth device via RFCOMM".
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
   - Bluetooth: `bluetoothctl info XX:XX:XX:XX:XX:XX` для проверки соединения; `ls /dev/rfcomm*` для портов.

### Возможные проблемы и отладка
- **Bluetooth не подключается**: Проверьте pairing (`bluetoothctl`), убедитесь в trust. Выполните rfcomm bind. В Docker используйте `privileged: true` и devices. Проверьте логи на "connect error for device '/dev/rfcomm0'".
- **No such file or directory (/dev/rfcomm0)**: `rfcomm bind` не был выполнен, устройство не сопряжено, или команда `bind` завершилась с ошибкой. Убедитесь, что вы выполнили `sudo rfcomm bind 0 XX:XX:XX:XX:XX:XX 1` и что устройство `/dev/rfcomm0` было создано.
- **Partial reads**: ELM327 может слать данные по частям; приложение буферизует до '>'. Если проблемы, увеличьте таймауты в коде.
- **MQTT ошибки**: Проверьте firewall/NAT на Pi, credentials в config.yaml.
- **Docker на Pi**: Убедитесь, что Docker установлен для arm64. Если cross-compile с Mac: используйте `docker buildx` для arm64.
- **Логи**: Уровень "debug" в config.yaml для детализации.

### Debug Logging

Для включения детального логирования установите уровень логирования в "debug" в файле `config.yaml`:

```yaml
logging:
  level: "debug"
```

При включенном debug логировании приложение будет выводить дополнительную информацию:

- **Входящие MQTT команды**: топик, сырые данные payload и декодированные команды
- **Ответы от Bluetooth**: полученные данные от ELM327 в текстовом и hex формате
- **Отправка в MQTT**: топик, закодированные данные payload и статус отправки

Пример вывода debug логов:
```
[ELM327-Bridge] 2024/01/01 12:00:00 main.go:254: [DEBUG] Received MQTT command on topic 'elm327/command', raw payload: QVRZ
[ELM327-Bridge] 2024/01/01 12:00:00 main.go:262: [DEBUG] Decoded MQTT command: ATZ (hex: 41545a)
[ELM327-Bridge] 2024/01/01 12:00:00 main.go:198: Sent command to ELM327: ATZ
[ELM327-Bridge] 2024/01/01 12:00:01 main.go:170: [DEBUG] Received data from Bluetooth: 41 00 BE 7F (hex: 34312030204245203746)
[ELM327-Bridge] 2024/01/01 12:00:01 main.go:173: [DEBUG] Publishing to MQTT topic 'elm327/data', encoded payload: NDEwMC41BCA3Rg==
[ELM327-Bridge] 2024/01/01 12:00:01 main.go:177: [DEBUG] Successfully published data to MQTT
```

## Настройка MQTT-брокера для анонимного доступа

Если вы хотите использовать анонимное подключение к MQTT, ваш брокер должен быть настроен соответствующим образом. Ниже приведен пример настройки Mosquitto:

**mosquitto.conf:**
```
# Разрешить анонимные подключения
allow_anonymous true

# ACL для анонимных пользователей (опционально)
acl_file /etc/mosquitto/acl

# Слушать на всех интерфейсах
listener 1883
```

**/etc/mosquitto/acl:**
```
# Разрешить анонимным пользователям читать и писать в топики elm327
topic readwrite elm327/#
```

После изменения конфигурации перезапустите Mosquitto:
```bash
sudo systemctl restart mosquitto
```

**Безопасность:** Анонимное подключение удобно для разработки, но для продакшена рекомендуется использовать аутентификацию с TLS.

## Архитектура
 Подробный план в [architecture_pi.md](../architecture_pi.md), включая диаграммы, потоки данных и примеры кода.

## Лицензия
MIT License (или укажите вашу).

Для вопросов или улучшений: обратитесь к архитектуре или исходному коду.