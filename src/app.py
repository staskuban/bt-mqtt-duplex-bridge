import asyncio
import logging
from typing import Optional

from .bluetooth_handler import BluetoothHandler
from .mqtt_handler import MQTTHandler

logger = logging.getLogger(__name__)

class CarDiagApp:
    """
    Основной класс приложения для оркестрации Bluetooth и MQTT.
    Следует принципам SOLID:单一 ответственность (оркестрация потоков),
    инверсия зависимостей (handlers инжектируются), открытость для расширения.
    DRY: Общие reconnect логика в handlers.
    KISS: Простой asyncio event loop с задачами.
    """

    def __init__(
        self,
        bt_addr: str,
        mqtt_broker: str = "mosquitto",
        mqtt_port: int = 1883,
        data_topic: str = "elm327/outgoing/data",
        cmd_topic: str = "elm327/incoming/command",
        reconnect_interval: int = 5
    ) -> None:
        """
        Инициализация приложения.
        
        :param bt_addr: MAC-адрес ELM327
        :param mqtt_broker: Хост MQTT-брокера (в Docker - имя сервиса)
        :param mqtt_port: Порт MQTT
        :param data_topic: Топик для данных от ELM327
        :param cmd_topic: Топик для команд к ELM327
        :param reconnect_interval: Интервал переподключения в секундах
        """
        self.bt_handler: BluetoothHandler = BluetoothHandler(bt_addr)
        self.mqtt_handler: MQTTHandler = MQTTHandler(
            broker=mqtt_broker,
            port=mqtt_port,
            data_topic=data_topic,
            cmd_topic=cmd_topic
        )
        self.reconnect_interval: int = reconnect_interval
        self._running: bool = False
        self._read_task: Optional[asyncio.Task] = None

    def _on_mqtt_command(self, command: bytes) -> None:
        """Callback для команд из MQTT: отправка в Bluetooth."""
        asyncio.create_task(self.bt_handler.write(command))

    async def _handle_bluetooth_read(self) -> None:
        """Фоновая задача для чтения из Bluetooth и публикации в MQTT."""
        while self._running:
            try:
                async for data in self.bt_handler.read():
                    if self._running:
                        await self.mqtt_handler.publish(data)
            except Exception as e:
                logger.error(f"Ошибка в чтении Bluetooth: {e}")
                if self._running:
                    await self.reconnect()

    async def connect(self) -> bool:
        """
        Подключение к Bluetooth и MQTT.
        
        :return: True если все подключено
        """
        bt_success = await self.bt_handler.connect()
        if not bt_success:
            logger.error("Не удалось подключиться к Bluetooth")
            return False

        mqtt_success = await self.mqtt_handler.connect()
        if not mqtt_success:
            logger.error("Не удалось подключиться к MQTT")
            await self.bt_handler.disconnect()
            return False

        # Установка callback
        self.mqtt_handler.set_on_message_callback(self._on_mqtt_command)

        logger.info("Приложение подключено: Bluetooth + MQTT")
        return True

    async def run(self) -> None:
        """Основной цикл приложения."""
        self._running = True
        success = await self.connect()
        if not success:
            return

        # Запуск задачи чтения
        self._read_task = asyncio.create_task(self._handle_bluetooth_read())

        try:
            while self._running:
                # Проверка соединений и reconnect если нужно
                if not self.bt_handler.sock:
                    logger.warning("Bluetooth отключено, переподключение...")
                    await self.reconnect()
                if not self.mqtt_handler.client or not self.mqtt_handler.client.is_connected():
                    logger.warning("MQTT отключено, переподключение...")
                    await self.reconnect()
                await asyncio.sleep(1)
        except KeyboardInterrupt:
            logger.info("Получен сигнал остановки")
        finally:
            await self.stop()

    async def stop(self) -> None:
        """Остановка приложения."""
        self._running = False
        if self._read_task:
            self._read_task.cancel()
            try:
                await self._read_task
            except asyncio.CancelledError:
                pass

        await self.bt_handler.disconnect()
        await self.mqtt_handler.disconnect()
        logger.info("Приложение остановлено")

    async def disconnect(self) -> None:
        """Отключение от всех компонентов."""
        await self.bt_handler.disconnect()
        await self.mqtt_handler.disconnect()

    async def reconnect(self) -> None:
        """Переподключение при ошибках."""
        logger.warning("Переподключение...")
        await asyncio.sleep(self.reconnect_interval)
        await self.disconnect()  # Сначала отключить
        await self.connect()

    async def __aenter__(self) -> 'CarDiagApp':
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        await self.stop()