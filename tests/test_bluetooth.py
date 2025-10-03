import asyncio
import pytest
from unittest.mock import Mock, AsyncMock, patch

from src.bluetooth_handler import BluetoothHandler

@pytest.fixture
def bt_handler():
    return BluetoothHandler(addr="00:11:22:33:44:55")

@pytest.mark.asyncio
async def test_connect_success(bt_handler):
    with patch('bluetooth.BluetoothSocket') as MockSocket:
        mock_sock = Mock()
        MockSocket.return_value = mock_sock
        mock_sock.connect = AsyncMock()

        success = await bt_handler.connect()

        assert success is True
        MockSocket.assert_called_once_with(bluetooth.RFCOMM)
        mock_sock.connect.assert_called_once_with(("00:11:22:33:44:55", 1))
        assert bt_handler.sock == mock_sock

@pytest.mark.asyncio
async def test_connect_failure(bt_handler):
    with patch('bluetooth.BluetoothSocket') as MockSocket:
        mock_sock = Mock()
        MockSocket.return_value = mock_sock
        mock_sock.connect.side_effect = Exception("Connection failed")

        success = await bt_handler.connect()

        assert success is False
        assert bt_handler.sock is None

@pytest.mark.asyncio
async def test_read_success(bt_handler):
    with patch('bluetooth.BluetoothSocket') as MockSocket, \
         patch.object(bt_handler, 'connect', new_callable=AsyncMock) as mock_connect:
        mock_sock = Mock()
        MockSocket.return_value = mock_sock
        mock_connect.return_value = True
        mock_sock.recv.side_effect = [b"test data", b"", None]

        data_iter = bt_handler.read()
        data1 = await anext(data_iter)
        data2 = await anext(data_iter)

        assert data1 == b"test data"
        assert data2 == b""
        mock_sock.recv.assert_any_call(1024)

@pytest.mark.asyncio
async def test_write_success(bt_handler):
    with patch('bluetooth.BluetoothSocket') as MockSocket, \
         patch.object(bt_handler, 'connect', new_callable=AsyncMock) as mock_connect:
        mock_sock = Mock()
        MockSocket.return_value = mock_sock
        mock_connect.return_value = True
        mock_sock.send = AsyncMock()

        success = await bt_handler.write(b"ATZ")

        assert success is True
        mock_sock.send.assert_called_once_with(b"ATZ")

@pytest.mark.asyncio
async def test_write_failure(bt_handler):
    with patch('bluetooth.BluetoothSocket') as MockSocket, \
         patch.object(bt_handler, 'connect', new_callable=AsyncMock) as mock_connect:
        mock_sock = Mock()
        MockSocket.return_value = mock_sock
        mock_connect.return_value = True
        mock_sock.send.side_effect = Exception("Send failed")

        success = await bt_handler.write(b"ATZ")

        assert success is False
        assert bt_handler.sock is None

@pytest.mark.asyncio
async def test_disconnect(bt_handler):
    bt_handler.sock = Mock()
    bt_handler.sock.close = AsyncMock()

    await bt_handler.disconnect()

    bt_handler.sock.close.assert_called_once()
    assert bt_handler.sock is None