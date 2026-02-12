screenshot-plat
================

一套基于 Go 的“远程截屏 + 多模态识别 + 邀请码管理”双模块项目：
- 客户端（screenshot）：截取主显示器屏幕，按长度前缀 TCP 协议上报 PNG 数据；首次连接需输入邀请码绑定设备
- 服务端（screensot-server）：TCP 收图 + HTTP 页面；支持“仅截屏刷新”和“截屏并识别”，内置模板，单个二进制即可运行

功能特性
- 截屏与识别分离：
  - 仅截屏刷新：不触发识别，保留上一次识别结果（只更新图片）
  - 截屏并识别：并行调用多模态模型，展示题目与答案
- 多模型识别：通过 SiliconFlow OpenAI 兼容接口（按配置文件选择模型）
- 模板内置：二进制内置默认页面模板；如存在外部模板则优先使用
- 简洁协议：4 字节大端长度前缀帧，JSON 传输
- 邀请码管理：内置管理后台（/admin），生成/撤销/重置绑定
- 单设备绑定：邀请码首次使用绑定设备；后续只允许该设备继续连接
- SQLite 持久化：邀请码与绑定信息存储在本地 data/invites.db

目录结构
```
.
├─ README.md                    # 本文件
├─ AGENTS.md                    # 贡献/风格与开发指南
├─ screenshot/                  # 客户端模块
│  ├─ cmd/client/main.go        # 入口，设置服务器地址后运行
│  └─ internal/{app,capture,protocol}
└─ screensot-server/            # 服务端模块
   ├─ cmd/server/main.go        # 入口
   ├─ internal/app/             # 核心：TCP/HTTP/识别/配置/内置模板
   │  ├─ http.go, tcp.go, vision.go, config.go, state.go, app.go, embed.go
   │  └─ templates/result_default.html  # 内置模板（go:embed）
   ├─ internal/protocol/        # 协议与测试
   └─ web/result.html           # 可选外部模板（优先于内置）
```

快速开始
1) 准备服务端配置（推荐）
- 拷贝示例并填写 API Key：
```
cp screensot-server/config.json.example screensot-server/config.json
# 编辑 screensot-server/config.json：
{
  "models": ["Qwen/Qwen3-VL-32B-Instruct"],
  "siliconflow_base_url": "https://api.siliconflow.cn",
  "siliconflow_api_key": "你的KEY",
  "template_path": "web/result.html",  # 可保留默认；不存在时将使用内置模板
  "sqlite_path": "data/invites.db",
  "admin_token": "你的ADMIN_TOKEN",
  "vision_timeout_seconds": 120
}
```

2) 启动服务端
- 模块根启动（建议）：
```
cd screensot-server
go run ./cmd/server
# 或构建后运行
go build -o server ./cmd/server
./server
```
- 仓库根启动（显式指定配置路径）：
```
SERVER_CONFIG=screensot-server/config.json go run ./screensot-server/cmd/server
# 或构建后二进制
SERVER_CONFIG=screensot-server/config.json ./screensot-server/server
```
- 启动日志会打印：using config: ... template=...，确认模板路径或内置模板生效。

3) 打开管理后台生成邀请码
- 管理后台地址：
  - http://localhost:8848/admin?token=你的ADMIN_TOKEN
- 在后台生成邀请码（按小时），复制给客户端使用。

4) 启动客户端
```
cd screenshot
# 按需修改服务器地址：screenshot/cmd/client/main.go 中的 address
go run ./cmd/client
# 或构建后二进制
go build -o client ./cmd/client
./client
```
- 首次在 macOS 需授予屏幕录制权限（系统设置 → 隐私与安全性 → 屏幕录制）。
- 客户端启动后会提示输入邀请码；首次绑定成功后，下次可直接回车跳过。

5) 使用（用户页面）
- 打开用户页：http://localhost:8848
- 输入邀请码后进入查看页面（/view），会自动加载“截屏并识别”
- 页面内可切换“仅截屏刷新 / 截屏并识别”

6) 直连接口（可选）
- 仅截屏刷新： http://localhost:8848/one?mode=capture&invite_code=邀请码
- 截屏并识别： http://localhost:8848/one?mode=analyze&invite_code=邀请码

配置说明（screensot-server/config.json）
- models: 模型列表（数组），默认 Qwen/Qwen3-VL-32B-Instruct
- siliconflow_base_url: SiliconFlow 网关，默认 https://api.siliconflow.cn
- siliconflow_api_key: API Key（只从配置读取，不支持环境变量覆盖）
- template_path: 外部模板路径，相对路径将按“相对于 config.json 所在目录”解析。找不到时自动使用内置模板
- sqlite_path: SQLite 数据文件路径（默认 data/invites.db）
- admin_token: 管理后台鉴权 Token（必填）
- vision_timeout_seconds: 识别请求超时（秒），默认 120

可选环境变量（覆盖非敏感项）
- SERVER_CONFIG: 指定配置文件路径
- VISION_MODELS: 覆盖 models（CSV）
- SILICONFLOW_BASEURL: 覆盖 siliconflow_base_url
- TEMPLATE_PATH: 覆盖 template_path
- SQLITE_PATH: 覆盖 sqlite_path
- ADMIN_TOKEN: 覆盖 admin_token
- VISION_TIMEOUT_SECONDS: 覆盖识别超时（秒）

端口与协议
- TCP 截屏通道：:12345（长度前缀帧，JSON 传输 PNG base64 数据）
- HTTP 页面与接口：:8848（/、/view、/one、/admin）

开发与构建
- 代码规范：go fmt ./...、go vet ./...
- 测试（已含协议帧单测）：在各模块根运行 go test ./...
- 构建：
```
# server
cd screensot-server && go build -o server ./cmd/server
# client
cd screenshot && go build -o client ./cmd/client
```
 - 不同操作系统/架构交叉编译：
```
# macOS (arm64)
cd screensot-server && GOOS=darwin GOARCH=arm64 go build -o server-darwin-arm64 ./cmd/server
cd screenshot && GOOS=darwin GOARCH=arm64 go build -o client-darwin-arm64 ./cmd/client

# macOS (amd64)
cd screensot-server && GOOS=darwin GOARCH=amd64 go build -o server-darwin-amd64 ./cmd/server
cd screenshot && GOOS=darwin GOARCH=amd64 go build -o client-darwin-amd64 ./cmd/client

# Linux (amd64)
cd screensot-server && GOOS=linux GOARCH=amd64 go build -o server-linux-amd64 ./cmd/server
cd screenshot && GOOS=linux GOARCH=amd64 go build -o client-linux-amd64 ./cmd/client

# Windows (amd64)
cd screensot-server && GOOS=windows GOARCH=amd64 go build -o server-windows-amd64.exe ./cmd/server
cd screenshot && GOOS=windows GOARCH=amd64 go build -o client-windows-amd64.exe ./cmd/client
```

常见问题
- Template not found
  - 二进制已内置默认模板；若仍看到该提示，检查是否运行旧版二进制；或确认 using config: ... template=... 日志
- No connected clients
  - 客户端未运行或未连接；先启动 server，再启动 client；确认 server 日志出现 TCP client connected
- invite_code required
  - /one 需要带 invite_code 参数；推荐通过 / 或 /view 进入
- 缺少 API Key
  - 确保 config.json 中已填写 siliconflow_api_key，并且进程读取的是你期望的配置文件（可用 SERVER_CONFIG 指定）
- 识别超时
  - 调大 vision_timeout_seconds 或 VISION_TIMEOUT_SECONDS；网络慢时建议 180-240 秒
- macOS 截屏权限
  - 需要在“隐私与安全性 → 屏幕录制”中为运行客户端的终端/IDE 授权

路线展望（未内置）
- 历史会话持久化（data 目录）与会话列表页面
- 会话内/全局去重、补识别
- 异步识别与进度展示

# screenshot-plat
