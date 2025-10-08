#!/bin/bash

# Скрипт настройки ELM327 Bridge на Raspberry Pi
# Требуются права root для выполнения

set -e

echo "🚗 Настройка ELM327 Bridge на Raspberry Pi"
echo "=========================================="

# Проверка прав root
if [[ $EUID -ne 0 ]]; then
   echo "❌ Этот скрипт должен выполняться с правами root (sudo)" >&2
   exit 1
fi

# Функция логирования
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

# 1. Обновление системы
log "📦 Обновление системы..."
apt update && apt upgrade -y

# 2. Установка необходимых пакетов
log "🔧 Установка зависимостей..."
apt install -y bluetooth bluez bluez-tools rfkill

# 3. Включение Bluetooth
log "🔵 Настройка Bluetooth..."
systemctl enable bluetooth
systemctl start bluetooth

# Проверка Bluetooth адаптера
if ! hciconfig hci0 | grep -q "UP"; then
    log "🔵 Включаем Bluetooth адаптер..."
    hciconfig hci0 up
fi

# 4. Установка Go (если не установлен)
if ! command -v go &> /dev/null; then
    log "🐹 Установка Go..."
    wget https://go.dev/dl/go1.21.0.linux-arm64.tar.gz
    tar -C /usr/local -xzf go1.21.0.linux-arm64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
    export PATH=$PATH:/usr/local/go/bin
fi

# 5. Сборка приложения
log "🔨 Сборка приложения..."
cd /home/pi/elm327-bridge
go mod tidy
go build -o elm327-bridge .

# 6. Создание сервиса systemd
log "⚙️ Создание сервиса systemd..."
cat > /etc/systemd/system/elm327-bridge.service << 'EOF'
[Unit]
Description=ELM327 Bridge Service
After=bluetooth.service
Wants=bluetooth.service
Requires=/dev/rfcomm0

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/home/pi/elm327-bridge
ExecStart=/home/pi/elm327-bridge/elm327-bridge
Restart=always
RestartSec=5

# Безопасность
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/home/pi/elm327-bridge /dev/rfcomm0

[Install]
WantedBy=multi-user.target
EOF

# 7. Создание скрипта для настройки Bluetooth устройства
log "📱 Создание скрипта настройки Bluetooth..."
cat > /home/pi/elm327-bridge/setup-bluetooth.sh << 'EOF'
#!/bin/bash
# Скрипт настройки Bluetooth соединения с ELM327

echo "🔍 Поиск ELM327 устройств..."
bluetoothctl << BT_EOF
power on
agent on
default-agent
scan on
BT_EOF

echo "📋 Найденные устройства:"
bluetoothctl devices

echo ""
echo "Введите MAC адрес вашего ELM327 устройства:"
read mac_address

if [[ -z "$mac_address" ]]; then
    echo "❌ MAC адрес не введен"
    exit 1
fi

echo "🔗 Сопряжение с устройством $mac_address..."
bluetoothctl << BT_EOF
pair $mac_address
trust $mac_address
exit
BT_EOF

echo "🔌 Создание RFCOMM соединения..."
rfcomm bind rfcomm0 $mac_address 1

echo "✅ Настройка завершена!"
echo "Теперь можно запустить сервис: sudo systemctl start elm327-bridge"
EOF

chmod +x /home/pi/elm327-bridge/setup-bluetooth.sh

# 8. Включение сервиса
log "🚀 Включение сервиса..."
systemctl daemon-reload
systemctl enable elm327-bridge

# 9. Создание конфигурации по умолчанию
if [[ ! -f /home/pi/elm327-bridge/config.yaml ]]; then
    log "📄 Создание конфигурации по умолчанию..."
    cp config.example.yaml config.yaml
    log "⚠️  Не забудьте настроить config.yaml!"
fi

log "✅ Настройка завершена!"
echo ""
echo "Следующие шаги:"
echo "1. Запустите настройку Bluetooth: ./setup-bluetooth.sh"
echo "2. Настройте config.yaml"
echo "3. Запустите сервис: sudo systemctl start elm327-bridge"
echo "4. Проверьте статус: sudo systemctl status elm327-bridge"
echo ""
echo "Для просмотра логов: sudo journalctl -u elm327-bridge -f"