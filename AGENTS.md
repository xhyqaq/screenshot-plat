# Repository Guidelines

必须用中文回答我

## Project Structure & Module Organization
- `screenshot/` (Go 1.22)
  - `cmd/client`: 入口二进制，最小 main，仅调用 `internal/app.Run`。
  - `internal/capture`: 屏幕捕获与 PNG 编码。
  - `internal/protocol`: 长度前缀帧与通用 `Response` 定义。
- `screensot-server/` (Go 1.22)
  - `cmd/server`: 入口二进制，最小 main，启动 `internal/app`。
  - `internal/app`: 拆分为 `tcp.go`（客户端连接与帧收发）、`http.go`（路由与模板渲染）、`vision.go`（多模态识别聚合）。
  - `internal/protocol`: 与客户端一致的帧编解码与 `Response`。

## Build, Test, and Development Commands
- Server (dev): `cd screensot-server && go run ./cmd/server`（日志含 “HTTP Server listening on :8848”）。触发一次：`curl -s http://localhost:8848/one`。
- Client (dev): `cd screenshot && go run ./cmd/client`（打印 “已连接到服务器”，等待命令）。
- Binaries: 在仓库根目录构建：
  - `go build -o bin/server ./screensot-server/cmd/server`
  - `go build -o bin/client ./screenshot/cmd/client`
- Tests (none yet): 从各模块根运行 `go test ./...`。

## Coding Style & Naming Conventions
- Format/lint: `go fmt ./...` and `go vet ./...` before committing.
- Naming: exported identifiers CamelCase; unexported lowerCamelCase; keep packages small and cohesive; prefer explicit error handling.
- Files: Go defaults via `gofmt`; avoid long functions; keep I/O and protocol code unit‑testable.

## Testing Guidelines
- 使用 `testing` + 表格驱动；文件命名 `*_test.go`。
- 建议单测：长度前缀帧（`protocol.SendWithLengthPrefix`/`ReadWithLengthPrefix` 使用 `net.Pipe`）、JSON 载荷、HTTP `/one` handler（用 `httptest`）。
- 覆盖成功路径与超时分支。

## Commit & Pull Request Guidelines
- Commits: short, imperative subjects; English or 中文 ok (history shows “init”, “first commit”, “添加ai识别”). Reference issues when relevant.
- PRs: concise description, what changed, how to run (server + client), and a screenshot or HTML snippet of `/one`. Note any port or config changes.

## Security & Configuration Tips
- 不要提交密钥。服务端支持环境变量：`VISION_MODELS`（CSV）、`SILICONFLOW_BASEURL`（默认 `https://api.siliconflow.cn`）、`SILICONFLOW_API_KEY`（必需）。
- 远程部署时，在 `screenshot/cmd/client/main.go` 修改 `address` 为服务器地址。
