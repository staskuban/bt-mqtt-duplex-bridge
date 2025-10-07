#!/usr/bin/env python3
"""
Скрипт для подключения к ELM327 через Bluetooth на Ubuntu Server 24
Позволяет отправлять AT команды и получать ответы
"""

import serial
import time
import sys

def connect_elm327(device_path="/dev/rfcomm0", baudrate=38400):
    """Подключение к ELM327 через serial порт"""
    try:
        # Настройка параметров соединения
        ser = serial.Serial(
            port=device_path,
            baudrate=baudrate,
            bytesize=serial.EIGHTBITS,
            parity=serial.PARITY_NONE,
            stopbits=serial.STOPBITS_ONE,
            timeout=3
        )
        
        print(f"Подключено к {device_path} на скорости {baudrate} бод")
        return ser
    except Exception as e:
        print(f"Ошибка подключения: {e}")
        return None

def send_command(ser, command):
    """Отправка команды в ELM327 и получение ответа"""
    try:
        # Очистка буфера
        ser.flushInput()
        ser.flushOutput()
        
        # Отправка команды с символом возврата каретки
        cmd = command + "\r"
        ser.write(cmd.encode('ascii'))
        
        print(f"Отправлена команда: {command}")
        
        # Ожидание и чтение ответа
        time.sleep(0.1)
        response = ""
        start_time = time.time()
        
        while time.time() - start_time < 3:  # тайм-аут 3 секунды
            if ser.in_waiting > 0:
                data = ser.read(ser.in_waiting).decode('ascii', errors='ignore')
                response += data
                if '>' in data:  # Промпт ELM327
                    break
            time.sleep(0.01)
        
        print(f"Ответ: {response.strip()}")
        return response.strip()
        
    except Exception as e:
        print(f"Ошибка при отправке команды: {e}")
        return None

def main():
    # Подключение к устройству
    elm = connect_elm327()
    if elm is None:
        print("Не удалось подключиться к ELM327")
        sys.exit(1)
    
    try:
        # Основные команды для получения информации об устройстве
        commands = [
            "ATI",      # Версия устройства
            "AT@1",     # Описание устройства  
            "AT@2",     # Идентификатор устройства
            "ATRV",     # Входное напряжение
            "ATDP",     # Описать протокол
        ]
        
        print("\n=== Получение информации об ELM327 ===")
        
        for cmd in commands:
            print(f"\n--- Команда: {cmd} ---")
            response = send_command(elm, cmd)
            time.sleep(0.2)  # Небольшая пауза между командами
            
    except KeyboardInterrupt:
        print("\nПрерывание работы пользователем")
    except Exception as e:
        print(f"Ошибка: {e}")
    finally:
        elm.close()
        print("\nСоединение закрыто")

if __name__ == "__main__":
    main()