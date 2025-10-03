package main

import (
	"encoding/base64"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockConn для тестирования net.Conn
type MockConn struct {
	mock.Mock
	closed bool
}

func (m *MockConn) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockConn) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockConn) Close() error {
	m.closed = true
	args := m.Called()
	return args.Error(0)
}

func (m *MockConn) LocalAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *MockConn) RemoteAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *MockConn) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockConn) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func TestWriteToBluetooth(t *testing.T) {
	// Mock config
	config.Elm327.Mac = "00:11:22:33:44:55"

	mockConn := new(MockConn)
	mockConn.On("Write", mock.Anything).Return(5, nil)
	btConn = mockConn

	command := []byte("ATZ")
	err := writeToBluetooth(command)

	assert.NoError(t, err)
	mockConn.AssertExpectations(t)
}

func TestBase64EncodingForBluetoothData(t *testing.T) {
	data := []byte("0100>41 00 BE 7F")
	encoded := base64.StdEncoding.EncodeToString(data)
	expected := "MDEwMD40MSAwMCBCRSA3Rg=="

	assert.Equal(t, expected, encoded)
}

func TestBase64DecodingForCommand(t *testing.T) {
	encoded := "QVRa" // base64 для "ATZ"
	decoded, err := base64.StdEncoding.DecodeString(encoded)

	assert.NoError(t, err)
	assert.Equal(t, []byte("ATZ"), decoded)
}
