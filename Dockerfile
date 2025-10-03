FROM python:3.11-slim

# Установка системных зависимостей для Bluetooth
RUN apt-get update && \
    apt-get install -y bluez libbluetooth-dev && \
    rm -rf /var/lib/apt/lists/*

# Установка Python зависимостей
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Копирование кода
COPY . /app
WORKDIR /app

# Запуск приложения
CMD ["python", "main.py"]