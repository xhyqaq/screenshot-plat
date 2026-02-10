screenshot-plat
================

一套基于 Go 的“远程截屏 + 多模态识别”双模块项目：
- 客户端（screenshot）：截取主显示器屏幕，按长度前缀 TCP 协议上报 PNG 数据
- 服务端（screensot-server）：TCP 收图 + HTTP 页面；支持“仅截屏刷新”和“截屏并识别”，并内置模板，单个二进制即可运行

功能特性
- 截屏与识别分离：
  - 仅截屏刷新：不触发识别，保留上一次识别结果（只更新图片）
  - 截屏并识别：并行调用多模态模型，展示题目与答案
- 多模型识别：通过 SiliconFlow OpenAI 兼容接口（按配置文件选择模型）
- 模板内置：二进制内置默认页面模板；如存在外部模板则优先使用
- 简洁协议：4 字节大端长度前缀帧，JSON 传输

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
  "template_path": "web/result.html"  # 可保留默认；不存在时将使用内置模板
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

3) 启动客户端
```
cd screenshot
# 按需修改服务器地址：screenshot/cmd/client/main.go 中的 address
go run ./cmd/client
# 或构建后二进制
go build -o client ./cmd/client
./client
```
- 首次在 macOS 需授予屏幕录制权限（系统设置 → 隐私与安全性 → 屏幕录制）。

4) 使用
- 仅截屏刷新： http://localhost:8848/one?mode=capture
- 截屏并识别： http://localhost:8848/one?mode=analyze 或 http://localhost:8848/one

配置说明（screensot-server/config.json）
- models: 模型列表（数组），默认 Qwen/Qwen3-VL-32B-Instruct
- siliconflow_base_url: SiliconFlow 网关，默认 https://api.siliconflow.cn
- siliconflow_api_key: API Key（只从配置读取，不支持环境变量覆盖）
- template_path: 外部模板路径，相对路径将按“相对于 config.json 所在目录”解析。找不到时自动使用内置模板

可选环境变量（覆盖非敏感项）
- SERVER_CONFIG: 指定配置文件路径
- VISION_MODELS: 覆盖 models（CSV）
- SILICONFLOW_BASEURL: 覆盖 siliconflow_base_url
- TEMPLATE_PATH: 覆盖 template_path

端口与协议
- TCP 截屏通道：:12345（长度前缀帧，JSON 传输 PNG base64 数据）
- HTTP 页面与接口：:8848（/one?mode=capture|analyze）

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

常见问题
- Template not found
  - 二进制已内置默认模板；若仍看到该提示，检查是否运行旧版二进制；或确认 using config: ... template=... 日志
- No connected clients
  - 客户端未运行或未连接；先启动 server，再启动 client；确认 server 日志出现 TCP client connected
- 缺少 API Key
  - 确保 config.json 中已填写 siliconflow_api_key，并且进程读取的是你期望的配置文件（可用 SERVER_CONFIG 指定）
- macOS 截屏权限
  - 需要在“隐私与安全性 → 屏幕录制”中为运行客户端的终端/IDE 授权

路线展望（未内置）
- 历史会话持久化（data 目录）与会话列表页面
- 会话内/全局去重、补识别
- 异步识别与进度展示

# screenshot-plat
