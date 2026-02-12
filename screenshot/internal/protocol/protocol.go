package protocol

import (
	"encoding/binary"
	"io"
	"net"
)

// Response 为客户端上报给服务器的统一消息体
type Response struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
	Data  []byte `json:"data"`
}

// AuthRequest 为客户端首次连接时的认证请求
type AuthRequest struct {
	InviteCode string `json:"invite_code"`
	DeviceID   string `json:"device_id"`
}

// AuthResponse 为服务端返回的认证结果
type AuthResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

// SendWithLengthPrefix 按 4 字节大端长度前缀发送
func SendWithLengthPrefix(conn net.Conn, data []byte) error {
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(data)))
	if _, err := conn.Write(lengthBuf[:]); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}

// ReadWithLengthPrefix 读取 4 字节大端长度前缀帧
func ReadWithLengthPrefix(conn net.Conn) ([]byte, error) {
	var lengthBuf [4]byte
	if _, err := io.ReadFull(conn, lengthBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf[:])
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}
	return data, nil
}
