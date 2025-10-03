import asyncio
import logging
from typing import Any

from src.app import CarDiagApp

# Настройка логирования
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

async def main() -> None:
    # Конфигурация (в будущем из env или config)
    BT_ADDR: str = "XX:XX:XX:XX:XX:XX"  # Заменить на реальный MAC ELM327
    MQTT_BROKER: str = "mosquitto"  # Имя сервиса в Docker
    MQTT_PORT: int = 1883
    DATA_TOPIC: str = "elm327/outgoing/data"
    CMD_TOPIC: str = "elm327/incoming/command"

    app = CarDiagApp(
        bt_addr=BT_ADDR,
        mqtt_broker=MQTT_BROKER,
        mqtt_port=MQTT_PORT,
        data_topic=DATA_TOPIC,
        cmd_topic=CMD_TOPIC
    )

    try:
        await app.run()
    except KeyboardInterrupt:
        logger.info("Приложение остановлено пользователем")
    except Exception as e:
        logger.error(f"Критическая ошибка: {e}")

if __name__ == "__main__":
    asyncio.run(main())