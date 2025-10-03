import asyncio
import base64
import logging
from typing import Callable, Dict, Optional, Any

import paho.mqtt.client as mqtt

logger = logging.getLogger(__name__)

class MQTTHandler:
    """
    Класс для обработки MQTT-соединения.
    Следует принципам SOLID:单一 ответственность (MQTT I/O),
    открытость для расширения (callbacks), инверсия зависимостей (callback для сообщений).
    DRY: Методы publish/subscribe используются в app.
    KISS: Простая обертка над paho-mqtt с async интеграцией.
    """

    def __init__(
        self,
        broker: str = "localhost",
        port: int = 1883,
        data_topic: str = "elm327/outgoing/data",
        cmd_topic: str = "elm327/incoming/command",
        qos: int = 1
    ) -> None:
        """
        Инициализация MQTT-обработчика.
        
        :param broker: Адрес MQTT-брокера
        :param port: Порт брокера
        :param data_topic: Топик для публикации данных от ELM327
        :param cmd_topic: Топик для подписки на команды
        :param qos: Уровень QoS для сообщений
        """
        self.broker: str = broker
        self.port: int = port
        self.data_topic: str = data_topic
        self.cmd_topic: str = cmd_topic
        self.qos: int = qos
        self.client: Optional[mqtt.Client] = None
        self.on_message_callback: Optional[Callable[[bytes], None]] = None
        self._loop_task: Optional[asyncio.Task] = None

    def set_on_message_callback(self, callback: Callable[[bytes], None]) -> None:
        """
        Установка callback для обработки входящих сообщений.
        
        :param callback: Функция, принимающая bytes команды
        """
        self.on_message_callback = callback

    def _on_connect(self, client: mqtt.Client, userdata: Any, flags: Dict, rc: int) -> None:
        """Callback при подключении: подписка на топик."""
        if rc == 0:
            client.subscribe(self.cmd_topic, qos=self.qos)
            logger.info(f"Подключено к MQTT брокеру {self.broker}:{self.port}, подписка на {self.cmd_topic}")
        else:
            logger.error(f"Ошибка подключения к MQTT: {rc}")

    def _on_message(self, client: mqtt.Client, userdata: Any, msg: mqtt.MQTTMessage) -> None:
        """Callback для сообщений: декодирование base64 и вызов пользовательского callback."""
        if msg.topic == self.cmd_topic and self.on_message_callback:
            try:
                decoded: bytes = base64.b64decode(msg.payload.decode('utf-8'))
                self.on_message_callback(decoded)
                logger.debug(f"Получена команда из MQTT: {decoded}")
            except Exception as e:
                logger.error(f"Ошибка обработки MQTT сообщения: {e}")

    def _on_disconnect(self, client: mqtt.Client, userdata: Any, rc: int) -> None:
        """Callback при отключении."""
        if rc != 0:
            logger.warning(f"Неожиданное отключение от MQTT: {rc}")

    async def connect(self) -> bool:
        """
        Асинхронное подключение к MQTT-брокеру.
        
        :return: True если успешно, иначе False
        """
        try:
            self.client = mqtt.Client()
            self.client.on_connect = self._on_connect
            self.client.on_message = self._on_message
            self.client.on_disconnect = self._on_disconnect

            # Подключение в отдельном потоке
            loop = asyncio.get_event_loop()
            await loop.run_in_executor(None, self.client.connect, self.broker, self.port, 60)
            
            # Запуск loop в фоне
            self._loop_task = asyncio.create_task(self._mqtt_loop())
            
            logger.info(f"MQTT клиент инициализирован")
            return True
        except Exception as e:
            logger.error(f"Ошибка подключения к MQTT: {e}")
            self.client = None
            return False

    async def _mqtt_loop(self) -> None:
        """Фоновый цикл для MQTT."""
        while self.client and self.client.is_connected():
            self.client.loop(timeout=1.0)
            await asyncio.sleep(0.1)

    async def disconnect(self) -> None:
        """Асинхронное отключение."""
        if self.client:
            self.client.loop_stop()
            await asyncio.to_thread(self.client.disconnect)
            if self._loop_task:
                self._loop_task.cancel()
                try:
                    await self._loop_task
                except asyncio.CancelledError:
                    pass
            self.client = None
            logger.info("Отключено от MQTT брокера")

    async def publish(self, data: bytes) -> bool:
        """
        Асинхронная публикация данных в топик.
        
        :param data: Байты данных для публикации (будут закодированы в base64)
        :return: True если успешно
        """
        if not self.client or not self.client.is_connected():
            success = await self.connect()
            if not success:
                return False

        try:
            encoded: str = base64.b64encode(data).decode('utf-8')
            await asyncio.to_thread(self.client.publish, self.data_topic, encoded, qos=self.qos)
            logger.debug(f"Опубликованы данные в MQTT: {len(data)} байт")
            return True
        except Exception as e:
            logger.error(f"Ошибка публикации в MQTT: {e}")
            return False

    async def __aenter__(self) -> 'MQTTHandler':
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        await self.disconnect()