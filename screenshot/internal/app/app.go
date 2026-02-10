package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"screenshot/internal/capture"
	"screenshot/internal/protocol"
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
