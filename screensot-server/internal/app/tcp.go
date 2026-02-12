package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"screensot-server/internal/protocol"
)

func (a *App) startTCPServer() {
	a.clients = make(map[net.Conn]string)
	a.responseCollector = make(chan string, 1000)

	listener, err := net.Listen("tcp", ":12345")
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		return
	}
	defer listener.Close()
	fmt.Println("TCP Server listening on :12345")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err.Error())
			continue
		}
		fmt.Println("TCP client connected:", conn.RemoteAddr().String())
		go a.handleTCPClient(conn)
	}
}

func (a *App) handleTCPClient(conn net.Conn) {
	defer func() {
		conn.Close()
		a.clientsMutex.Lock()
		delete(a.clients, conn)
		a.clientsMutex.Unlock()
	}()

	deviceID, err := a.authenticateClient(conn)
	if err != nil {
		fmt.Println("TCP client auth failed:", err)
		return
	}
	a.clientsMutex.Lock()
	a.clients[conn] = deviceID
	a.clientsMutex.Unlock()
	fmt.Println("TCP client authenticated:", conn.RemoteAddr().String(), "device_id="+deviceID)

	for {
		dataBytes, err := protocol.ReadWithLengthPrefix(conn)
		if err != nil {
			if err == io.EOF {
				fmt.Println("TCP client disconnected:", conn.RemoteAddr().String())
			} else {
				fmt.Println("Error reading data from client:", err)
			}
			return
		}

		var responseObj protocol.Response
		if err := json.Unmarshal(dataBytes, &responseObj); err != nil {
			fmt.Println("Error unmarshalling JSON from client:", err)
			continue
		}

		// 统一在 TCP 层转成 base64，HTTP 层只负责聚合
		base64Str := base64.StdEncoding.EncodeToString(responseObj.Data)
		fmt.Printf("Received image from %s, Base64 size: %d\n", conn.RemoteAddr().String(), len(base64Str))
		if len(base64Str) > 100 {
			fmt.Println("Base64 Image Data (truncated):", base64Str[:100]+"...")
		}
		a.responseCollector <- base64Str
	}
}

func (a *App) sendCaptureCommandToClients() {
	a.clientsMutex.Lock()
	defer a.clientsMutex.Unlock()
	msg := []byte("1")
	for c := range a.clients {
		go func(conn net.Conn) {
			if err := protocol.SendWithLengthPrefix(conn, msg); err != nil {
				fmt.Printf("Failed to send command to client %s: %v\n", conn.RemoteAddr().String(), err)
			} else {
				fmt.Printf("Sent command to client %s\n", conn.RemoteAddr().String())
			}
		}(c)
	}
}

func (a *App) authenticateClient(conn net.Conn) (string, error) {
	dataBytes, err := protocol.ReadWithLengthPrefix(conn)
	if err != nil {
		return "", err
	}
	var req protocol.AuthRequest
	if err := json.Unmarshal(dataBytes, &req); err != nil {
		sendAuthResponse(conn, 400, "invalid auth request")
		return "", err
	}
	if err := a.invites.Authenticate(req.InviteCode, req.DeviceID); err != nil {
		sendAuthResponse(conn, 401, err.Error())
		return "", err
	}
	sendAuthResponse(conn, 200, "")
	return req.DeviceID, nil
}

func sendAuthResponse(conn net.Conn, code int, msg string) {
	resp := protocol.AuthResponse{Code: code, Error: msg}
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = protocol.SendWithLengthPrefix(conn, b)
}
