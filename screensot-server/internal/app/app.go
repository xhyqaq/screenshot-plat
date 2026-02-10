package app

import (
	"fmt"
)

// App 持有服务器运行期状态（TCP 客户端集合、响应收集通道等）
type App struct {
	*state
	cfg Config
}

// New 创建应用实例
func New() *App { return &App{state: &state{}, cfg: loadConfig()} }

// Run 并行启动 TCP 与 HTTP 服务
func (a *App) Run() {
	go a.startTCPServer() // 监听 :12345 并接收客户端
	fmt.Println("HTTP Server listening on :8848")
	a.startHTTPServer() // 阻塞在 HTTP 服务
}
