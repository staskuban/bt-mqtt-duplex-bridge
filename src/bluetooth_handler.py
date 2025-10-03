import asyncio
import bluetooth
import logging
from typing import Optional, AsyncIterator

logger = logging.getLogger(__name__)

class BluetoothHandler:
    """
    Класс для обработки Bluetooth-соединения с ELM327.
    Следует принципам SOLID:单一 ответственность (соединение и I/O),
    открытость для расширения (можно добавить reconnect).
    DRY: Методы read/write используются в app.
    KISS: Простая async обертка над pybluez.
    """

    def __init__(self, addr: str, channel: int = 1) -> None:
        """
        Инициализация обработчика Bluetooth.
        
        :param addr: MAC-адрес ELM327 устройства
        :param channel: RFCOMM канал (по умолчанию 1 для SPP)
        """
        self.addr: str = addr
        self.channel: int = channel
        self.sock: Optional[bluetooth.BluetoothSocket] = None
        self._lock = asyncio.Lock()

    async def connect(self) -> bool:
        """
        Асинхронное соединение с ELM327.
        
        :return: True если успешно, иначе False
        """
        async with self._lock:
            try:
                self.sock = bluetooth.BluetoothSocket(bluetooth.RFCOMM)
                await asyncio.to_thread(self.sock.connect, (self.addr, self.channel))
                logger.info(f"Подключено к Bluetooth устройству {self.addr}")
                return True
            except bluetooth.BluetoothError as e:
                logger.error(f"Ошибка подключения к Bluetooth: {e}")
                self.sock = None
                return False

    async def disconnect(self) -> None:
        """Асинхронное отключение."""
        async with self._lock:
            if self.sock:
                await asyncio.to_thread(self.sock.close)
                self.sock = None
                logger.info(f"Отключено от Bluetooth устройства {self.addr}")

    async def read(self, buffer_size: int = 1024) -> AsyncIterator[bytes]:
        """
        Асинхронное чтение данных из Bluetooth.
        Генератор для потокового чтения.
        
        :param buffer_size: Размер буфера для чтения
        :yield: Чанки байтов от ELM327
        """
        if not self.sock:
            await self.connect()
            if not self.sock:
                return

        while True:
            try:
                data: bytes = await asyncio.to_thread(self.sock.recv, buffer_size)
                if data:
                    yield data
                else:
                    logger.warning("Bluetooth соединение закрыто")
                    await self.disconnect()
                    break
            except bluetooth.BluetoothError as e:
                logger.error(f"Ошибка чтения из Bluetooth: {e}")
                await self.disconnect()
                break

    async def write(self, command: bytes) -> bool:
        """
        Асинхронная запись команды в Bluetooth.
        
        :param command: Байты команды для ELM327 (с \r в конце если нужно)
        :return: True если успешно отправлено
        """
        if not self.sock:
            success = await self.connect()
            if not success:
                return False

        async with self._lock:
            try:
                await asyncio.to_thread(self.sock.send, command)
                logger.debug(f"Отправлена команда в Bluetooth: {command}")
                return True
            except bluetooth.BluetoothError as e:
                logger.error(f"Ошибка записи в Bluetooth: {e}")
                await self.disconnect()
                return False

    async def __aenter__(self) -> 'BluetoothHandler':
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        await self.disconnect()