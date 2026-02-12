package app

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"screenshot/internal/capture"
	"screenshot/internal/protocol"
	"strings"
)

// Run 启动客户端：连接服务器，循环接收命令并发送截图结果
func Run(address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Error connecting:", err.Error())
		return
	}
	defer conn.Close()
	fmt.Println("已连接到服务器")

	deviceID, err := loadOrCreateDeviceID()
	if err != nil {
		fmt.Println("设备ID初始化失败:", err.Error())
		return
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("请输入邀请码（首次绑定需要，已绑定可直接回车）：")
	inviteCode, _ := reader.ReadString('\n')
	inviteCode = strings.TrimSpace(inviteCode)

	if err := authenticate(conn, inviteCode, deviceID); err != nil {
		fmt.Println("认证失败:", err.Error())
		return
	}
	fmt.Println("认证成功，等待指令...")

	for {
		// 读取命令（长度前缀帧）
		commandBytes, err := protocol.ReadWithLengthPrefix(conn)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Connection closed by server")
			} else {
				fmt.Println("Error reading command:", err.Error())
			}
			return
		}
		command := string(commandBytes)
		fmt.Printf("Received command: %s\n", command)

		resp := protocol.Response{Code: 200}
		switch command {
		case "1": // 截图
			pngBytes, err := capture.PrimaryPNG()
			if err != nil {
				resp.Code = 500
				resp.Error = err.Error()
			} else {
				resp.Data = pngBytes
			}
		default:
			resp.Code = 400
			resp.Error = "Unknown command"
		}

		// 编码为 JSON 发送（仍然套长度前缀帧）
		b, err := json.Marshal(resp)
		if err != nil {
			fmt.Println("Error marshalling JSON:", err)
			continue
		}
		if err := protocol.SendWithLengthPrefix(conn, b); err != nil {
			fmt.Println("Error sending data:", err)
			return
		}
		fmt.Println("Sent response, size:", len(b))
	}
}

func authenticate(conn net.Conn, inviteCode, deviceID string) error {
	req := protocol.AuthRequest{
		InviteCode: inviteCode,
		DeviceID:   deviceID,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := protocol.SendWithLengthPrefix(conn, b); err != nil {
		return err
	}
	respBytes, err := protocol.ReadWithLengthPrefix(conn)
	if err != nil {
		return err
	}
	var resp protocol.AuthResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return err
	}
	if resp.Code != 200 {
		if resp.Error != "" {
			return fmt.Errorf("%s", resp.Error)
		}
		return fmt.Errorf("auth failed: code=%d", resp.Code)
	}
	return nil
}

func loadOrCreateDeviceID() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".screenshot")
	path := filepath.Join(dir, "device_id")
	if b, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(b))
		if id != "" {
			return id, nil
		}
	}
	id, err := newRandomID()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(id), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

func newRandomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
