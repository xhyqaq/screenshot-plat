# 邀请码平台方案（单机 + SQLite + 单设备绑定）

## 目标
- 管理员发放邀请码（带有效期）。
- 客户端必须输入邀请码才能使用。
- 一个邀请码只能绑定一个设备，避免转发给他人复用。
- 单机部署（无多服务端），但希望有可用的管理后台。
- 不引入复杂用户体系。

## 结论
- 必须有持久化状态才能做到邀请码只绑定一个设备。
- 采用 SQLite 作为最小持久化（本地文件），无需外部数据库服务。
- 邀请码校验 = 过期时间 + 绑定设备；后台提供发放、列表、撤销、重置绑定能力。

## 系统组件
- Server：screensot-server
  - TCP 服务：客户端连接、认证、指令收发。
  - HTTP 后台：管理员发放/管理邀请码。
- Client：screenshot
  - 设备身份生成与持久化（本地）。
  - 首次使用：提交邀请码绑定设备。
  - 后续使用：仅提交设备凭证即可。

## 核心流程
### 1) 设备身份（无用户体系）
- 客户端首次启动时生成随机 device_id（UUID 即可）。
- device_id 持久化到本地文件（例如 ~/.screenshot/device_id）。
- 之后每次启动都读取同一个 device_id，避免被识别为新设备。

### 2) 首次激活（绑定）
客户端发送：
- invite_code
- device_id

服务端处理：
1. 校验邀请码是否存在、是否过期、是否被撤销。
2. 若 bound_device_id 为空：写入 bound_device_id、bound_at。
3. 返回确认成功即可（后续请求只需要 device_id）。

### 3) 后续使用（免邀请码）
客户端发送：
- device_id

服务端验证：
- device_id 是否与绑定一致
- 通过后允许使用

这样即使邀请码泄露，也无法在另一台设备激活。

## 数据模型（SQLite）
表：invites
- id INTEGER PRIMARY KEY
- code_hash TEXT UNIQUE NOT NULL（存 SHA256，不存明文）
- exp_at INTEGER NOT NULL（Unix 秒）
- created_at INTEGER NOT NULL
- note TEXT
- revoked INTEGER NOT NULL DEFAULT 0
- bound_device_id TEXT
- bound_at INTEGER
- last_seen_at INTEGER

索引：
- code_hash
- exp_at

绑定设备只需放在 invites 表中，无需单独 devices 表。

## 管理后台（HTTP）
使用 ADMIN_TOKEN 简单鉴权（Header：X-Admin-Token）。

建议接口：
- POST /admin/invites
  - body: ttl_seconds, note
  - 返回：invite_code, exp_at
- GET /admin/invites
  - 返回：列表（含绑定状态、过期时间、撤销状态）
- POST /admin/invites/revoke
  - body: invite_code
- POST /admin/invites/reset-binding
  - body: invite_code
  - 清空 bound_device_id，用于人工解除绑定

仅单机部署时，这套接口足够轻量。

## TCP 认证协议（建议）
新增消息类型（示意）：
- AuthRequest:
  - invite_code（首次绑定）
  - device_id

认证流程：
1. 客户端发送 AuthRequest。
2. 服务端验证（邀请码或 device_id）。
3. 服务端返回认证成功或失败。

现有 internal/protocol 只需要增加一个认证消息结构，复用长度前缀帧即可。

## 配置项
- SQLITE_PATH（默认 data/invites.db）
- ADMIN_TOKEN（管理后台鉴权）
- DEVICE_TOKEN_SECRET（可选：如果未来需要服务端签发 token）

## 安全要点
- 邀请码只存 hash，避免泄露即被使用。
- device_id 必须本地持久化，否则每次启动会被当作新设备。
- 管理后台必须有鉴权（ADMIN_TOKEN）。

## 与现有代码结构的落点（建议）
- screensot-server/internal/app/invite.go
  - SQLite 初始化、CRUD、绑定逻辑
- screensot-server/internal/app/http.go
  - /admin/invites* 路由
- screensot-server/internal/app/tcp.go
  - 连接后先走认证流程
- screensot-server/internal/protocol
  - 新增 AuthRequest
- screenshot/cmd/client
  - 生成/保存 device_id、处理认证流程

## 关键行为约束
- 邀请码在首次绑定设备后不可更换（除非管理员手动 reset）。
- 邀请码过期后，无论是否绑定都不可再用。
- 同一邀请码只能一个设备使用（强制绑定）。

## 扩展点（以后可选）
- 统计：活跃设备数、最近使用时间。
- 后台可导出使用记录。
- 绑定设备的备注（例如部署地点）。
