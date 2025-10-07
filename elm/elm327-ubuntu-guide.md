# Подключение к ELM327 через Bluetooth на Ubuntu Server 24

Вот пошаговая инструкция для подключения к ELM327 адаптеру через Bluetooth интерфейс и получения описания устройства.

## Шаг 1: Установка необходимых пакетов

```bash
# Обновление системы
sudo apt update && sudo apt upgrade -y

# Установка Bluetooth пакетов и утилит
sudo apt install -y bluetooth bluez bluez-tools rfkill python3-serial python3-pip minicom picocom screen

# Установка утилит для работы с последовательным портом
sudo apt install -y minicom picocom screen

# Установка Python библиотек (если планируете использовать скрипт)
sudo apt install -y python3-serial python3-pip
```

## Шаг 2: Проверка и настройка Bluetooth адаптера

```bash
# Проверка статуса Bluetooth адаптера
sudo systemctl status bluetooth

# Проверка блокировок RF
sudo rfkill list bluetooth

# Если Bluetooth заблокирован - разблокировать
sudo rfkill unblock bluetooth

# Включение Bluetooth адаптера
sudo hciconfig hci0 up

# Проверка активности адаптера
hciconfig -a
```

## Шаг 3: Поиск и сопряжение с ELM327

```bash
# Запуск bluetoothctl для управления Bluetooth
sudo bluetoothctl
```

В интерфейсе `bluetoothctl` выполните следующие команды:

```bash
# Включение адаптера и поиска
power on
agent on
default-agent
discoverable on

# Сканирование устройств (ELM327 должен быть включен!)
scan on

# Ожидайте появления устройства типа "OBDII" или "ELM327"
# Примерный вывод: [NEW] Device 00:1D:A5:68:98:8A OBDII

# Сопряжение с устройством (замените MAC-адрес на ваш)
pair 00:1D:A5:68:98:8A

# Введите PIN-код (обычно 1234 или 0000)
# PIN code: 1234

# Доверие устройству
trust 00:1D:A5:68:98:8A

# Выход из bluetoothctl
exit
```

## Шаг 4: Определение канала RFCOMM

```bash
# Проверка доступных служб Bluetooth устройства
sudo sdptool records 00:10:CC:4F:36:03

# Найдите строку "Channel: X" для Serial Port Profile
# Обычно это канал 1
```

## Шаг 5: Создание RFCOMM соединения

```bash
# Привязка RFCOMM устройства (замените MAC-адрес и канал)
sudo rfcomm bind rfcomm0 00:1D:A5:68:98:8A 1

# Проверка создания устройства
ls -l /dev/rfcomm0

# Альтернативный способ подключения без bind
sudo rfcomm connect rfcomm0 00:1D:A5:68:98:8A 1 &
```

## Шаг 6: Настройка прав доступа

```bash
# Добавление пользователя в группу dialout
sudo usermod -a -G dialout $USER

# Применение изменений (потребуется перелогиниться)
newgrp dialout

# Установка прав на устройство
sudo chmod 666 /dev/rfcomm0
```

## Шаг 7: Тестирование подключения через minicom

```bash
# Подключение через minicom
minicom -D /dev/rfcomm0 -b 38400

# Настройки minicom:
# - Скорость: 38400 бод
# - 8 бит данных
# - Без четности
# - 1 стоп-бит
# - Без управления потоком
```

В minicom введите команды для тестирования:

```
ATZ          (сброс устройства)
ATI          (версия устройства)
AT@1         (описание устройства)
AT@2         (идентификатор устройства)
ATRV         (входное напряжение)
```

**Для выхода из minicom**: `Ctrl+A`, затем `X`

## Шаг 8: Использование Python скрипта

Я создал Python скрипт для автоматизации работы с ELM327. Скопируйте его и запустите:

```bash
# Копирование скрипта
sudo cp /tmp/elm327_connect.py /usr/local/bin/
sudo chmod +x /usr/local/bin/elm327_connect.py

# Запуск скрипта
python3 /usr/local/bin/elm327_connect.py
```

## Шаг 9: Альтернативные способы подключения

### Через screen

```bash
screen /dev/rfcomm0 38400
```

### Через picocom

```bash
picocom -b 38400 /dev/rfcomm0
```

### Прямое чтение/запись

```bash
# Отправка команды
echo -e "ATI\r" > /dev/rfcomm0

# Чтение ответа
cat /dev/rfcomm0
```

## Команды для получения описания устройства

После успешного подключения используйте эти AT-команды:

| Команда | Описание |
|---------|----------|
| **`ATZ`** | Полный сброс устройства |
| **`ATI`** | Версия чипа ELM327 |
| **`AT@1`** | Описание устройства |
| **`AT@2`** | Идентификатор устройства |
| **`ATRV`** | Входное напряжение |
| **`ATDP`** | Текущий протокол |
| **`ATH1`** | Включить заголовки |

## Устранение проблем

### Если устройство не найдено:
```bash
# Перезапуск Bluetooth службы
sudo systemctl restart bluetooth

# Сброс Bluetooth адаптера
sudo hciconfig hci0 reset
```

### Если /dev/rfcomm0 не создается:
```bash
# Ручное создание RFCOMM устройства
sudo mknod /dev/rfcomm0 c 216 0
sudo chmod 666 /dev/rfcomm0
```

### Если получаете только символ "?":
- Проверьте скорость соединения (должна быть 38400)
- Убедитесь, что команды заканчиваются символом `\r` (возврат каретки)
- Попробуйте другой канал RFCOMM

### Автоматический запуск при загрузке

Создайте systemd сервис для автоматического подключения:

```bash
sudo tee /etc/systemd/system/elm327.service > /dev/null << 'EOF'
[Unit]
Description=ELM327 Bluetooth Connection
After=bluetooth.service
Wants=bluetooth.service

[Service]
Type=forking
ExecStart=/usr/bin/rfcomm bind rfcomm0 00:1D:A5:68:98:8A 1
ExecStop=/usr/bin/rfcomm release rfcomm0
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

# Активация сервиса
sudo systemctl enable elm327.service
sudo systemctl start elm327.service
```

После выполнения этих шагов вы должны получить доступ к ELM327 устройству и сможете выполнять AT-команды для получения информации об адаптере и диагностики автомобиля.