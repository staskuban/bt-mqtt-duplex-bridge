import asyncio
import base64
import pytest
from unittest.mock import Mock, AsyncMock, patch

from src.mqtt_handler import MQTTHandler

@pytest.fixture
def mqtt_handler():
    return MQTTHandler(
        broker="localhost",
        port=1883,
        data_topic="elm327/outgoing/data",
        cmd_topic="elm327/incoming/command"
    )

@pytest.mark.asyncio
async def test_connect_success(mqtt_handler):
    with patch('paho.mqtt.client.Client') as MockClient:
        mock_client = Mock()
        mock_client.connect = AsyncMock(return_value=0)
        mock_client.is_connected = Mock(return_value=True)
        MockClient.return_value = mock_client

        # Мок для _mqtt_loop
        mock_loop_task = AsyncMock()
        with patch.object(mqtt_handler, '_mqtt_loop', return_value=mock_loop_task):
            success = await mqtt_handler.connect()

        assert success is True
        MockClient.assert_called_once()
        mock_client.connect.assert_called_once_with("localhost", 1883, 60)
        assert mqtt_handler.client == mock_client

@pytest.mark.asyncio
async def test_connect_failure(mqtt_handler):
    with patch('paho.mqtt.client.Client') as MockClient:
        mock_client = Mock()
        mock_client.connect.side_effect = Exception("Connection failed")
        MockClient.return_value = mock_client

        success = await mqtt_handler.connect()

        assert success is False
        assert mqtt_handler.client is None

@pytest.mark.asyncio
async def test_publish_success(mqtt_handler):
    with patch('paho.mqtt.client.Client') as MockClient, \
         patch.object(mqtt_handler, 'connect', new_callable=AsyncMock) as mock_connect:
        mock_client = Mock()
        mock_client.is_connected.return_value = True
        mock_client.publish = AsyncMock(return_value=(0, 0))
        MockClient.return_value = mock_client
        mock_connect.return_value = True

        await mqtt_handler.connect()
        success = await mqtt_handler.publish(b"test data")

        assert success is True
        encoded = base64.b64encode(b"test data").decode('utf-8')
        mock_client.publish.assert_called_once_with("elm327/outgoing/data", encoded, qos=1)

@pytest.mark.asyncio
async def test_publish_failure(mqtt_handler):
    with patch('paho.mqtt.client.Client') as MockClient, \
         patch.object(mqtt_handler, 'connect', new_callable=AsyncMock) as mock_connect:
        mock_client = Mock()
        mock_client.is_connected.return_value = True
        mock_client.publish.side_effect = Exception("Publish failed")
        MockClient.return_value = mock_client
        mock_connect.return_value = True

        await mqtt_handler.connect()
        success = await mqtt_handler.publish(b"test data")

        assert success is False

def test_set_on_message_callback(mqtt_handler):
    def mock_callback(data):
        pass

    mqtt_handler.set_on_message_callback(mock_callback)

    assert mqtt_handler.on_message_callback == mock_callback

@pytest.mark.asyncio
async def test_disconnect(mqtt_handler):
    with patch('paho.mqtt.client.Client') as MockClient:
        mock_client = Mock()
        mock_client.disconnect = AsyncMock()
        mock_client.loop_stop = Mock()
        MockClient.return_value = mock_client

        await mqtt_handler.connect()
        await mqtt_handler.disconnect()

        mock_client.loop_stop.assert_called_once()
        mock_client.disconnect.assert_called_once()
        assert mqtt_handler.client is None

def test_on_message_callback(mqtt_handler):
    mock_callback = Mock()
    mqtt_handler.set_on_message_callback(mock_callback)
    mqtt_handler.client = Mock()
    msg = Mock()
    msg.topic = "elm327/incoming/command"
    msg.payload = base64.b64encode(b"test command").decode('utf-8').encode('utf-8')

    # Вызов _on_message
    mqtt_handler._on_message(mqtt_handler.client, None, msg)

    mock_callback.assert_called_once_with(b"test command")