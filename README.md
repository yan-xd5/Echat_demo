# eChat — tRPC-Go 即时通讯后端

基于 tRPC-Go 框架的分布式 IM 系统，采用三服务架构，支持 WebSocket 长连接、Polaris 服务发现、OpenTelemetry 链路追踪。

## 架构

```
客户端 (WebSocket) ──► Gateway (代理) ──► Controller (中控) ──► API (数据)
   端口:9000             端口:8003           端口:8002             端口:8001/9001
                                                                     │
                                                                   MySQL
```

**三服务职责：**

| 服务 | 协议 | 职责 |
|------|------|------|
| **Gateway** | WebSocket + tRPC | 长连接接入、协议转换、连接管理、消息收发 |
| **Controller** | tRPC | 消息路由决策、Token 校验、在线状态管理 |
| **API** | tRPC + HTTP | 用户/好友/群组 CRUD、数据库操作 |

**调用链路：**

```
Gateway ──tRPC──► Controller.AuthCheck / RouteMessage / UpdateStatus
                       │
                       └── tRPC ──► API.GetUserInfo / Login
                                       │
                                       └── sqlx ──► MySQL
```

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.24 |
| 微服务框架 | tRPC-Go |
| 序列化 | Protobuf |
| WebSocket | gorilla/websocket |
| 数据库 | MySQL（sqlx） |
| 服务发现 | Polaris（北极星） |
| 链路追踪 | OpenTelemetry + Jaeger |
| 包管理 | Go Workspace |

## 目录结构

```
echat/
├── go.work                          # Go Workspace（多模块管理）
├── go.mod                           # 根模块
├── proto/common/                    # 共享 Proto 定义
│   └── common.proto
├── service/
│   ├── api/                         # API 服务
│   │   ├── proto/user.proto         # RPC + HTTP 路由定义
│   │   ├── stub/                    # 生成的桩代码（.pb.go + .trpc.go）
│   │   │   ├── user.pb.go
│   │   │   └── user.trpc.go
│   │   ├── server/
│   │   │   ├── main.go              # 启动入口
│   │   │   ├── db.go                # MySQL 连接池
│   │   │   ├── config.go            # 环境变量加载
│   │   │   ├── trpc_go.yaml         # tRPC 配置
│   │   │   ├── .env                 # 本地数据库密码（不提交）
│   │   │   ├── .env.example         # 环境变量模板
│   │   │   └── repository/          # 数据访问层
│   │   │       └── user_repo.go
│   │   └── go.mod
│   │
│   ├── controller/                  # 中控服务
│   │   ├── proto/controller.proto
│   │   ├── stub/
│   │   ├── server/
│   │   │   ├── main.go
│   │   │   ├── filter.go            # 请求拦截 Filter
│   │   │   └── trpc_go.yaml
│   │   └── go.mod
│   │
│   └── gateway/                     # 代理服务
│       ├── proto/gateway.proto
│       ├── stub/
│       ├── server/
│       │   ├── main.go
│       │   ├── ws_handler.go        # WebSocket 处理器
│       │   ├── conn_manager.go      # 连接管理器
│       │   ├── test_client.go       # WebSocket 测试客户端
│       │   └── trpc_go.yaml
│       └── go.mod
│
├── CLAUDE.md                        # Claude Code 项目文档
└── README.md
```

## 功能

### 已实现

- [x] 用户登录认证（account + password）
- [x] WebSocket 长连接（认证、收发消息、心跳）
- [x] 消息路由（单聊，含接收者校验）
- [x] 在线状态管理（上线/下线通知）
- [x] Token 校验 Filter（解析请求，注入用户信息到 ctx）
- [x] 优雅关机（全部三服务）
- [x] OpenTelemetry 链路追踪（Jaeger 可视化）
- [x] 双协议支持（API 同时支持 tRPC 和 HTTP）
- [x] 数据库环境变量管理（godotenv + .env）

### 待实现

- [ ] 好友关系管理
- [ ] 群聊功能
- [ ] 消息持久化存储与离线消息
- [ ] JWT 真实 Token 签发与校验
- [ ] 协程池 + 限流 + 熔断降级
- [ ] 服务发现（北极星/Consul）

## 插件与中间件

| 插件 | 服务 | 用途 |
|------|------|------|
| **opentelemetry** | 全部 | 链路追踪，TraceID 注入日志 + Span 上报 Jaeger |
| **polarismesh** | 全部 | 北极星服务发现，替代硬编码 IP |
| **myServerFilter** | Controller | 自定义 Filter：按请求类型解析 Token、校验身份、注入 ctx |
| **godotenv** | API | 自动加载 .env 环境变量 |
| **sqlx** | API | MySQL 连接池与查询 |

## 服务端口

| 服务 | 端口 | 协议 |
|------|------|------|
| API tRPC | 8001 | tRPC |
| API HTTP | 9001 | HTTP |
| Controller | 8002 | tRPC |
| Gateway tRPC | 8003 | tRPC |
| Gateway WebSocket | 9000 | WebSocket |
| Jaeger UI | 16686 | HTTP |

## 快速开始

### 1. 环境要求

- Go 1.24+
- MySQL 8.0+
- Docker（运行 Polaris）
- Jaeger（可选，链路追踪可视化）

### 1.1 启动 Polaris

```bash
docker run -d --name polaris -p 8091:8091 polarismesh/polaris-standalone:latest
```

### 2. 数据库

```sql
CREATE DATABASE chat DEFAULT CHARSET utf8mb4;

CREATE TABLE user (
    uid VARCHAR(20) PRIMARY KEY COMMENT '用户编号',
    username VARCHAR(50) NOT NULL COMMENT '用户名',
    account VARCHAR(100) NOT NULL UNIQUE COMMENT '账号',
    password VARCHAR(100) NOT NULL COMMENT '密码',
    gender ENUM('male','female','other') NOT NULL DEFAULT 'other',
    region VARCHAR(100) DEFAULT NULL,
    email VARCHAR(100) DEFAULT NULL,
    avatar VARCHAR(500) DEFAULT NULL,
    bio VARCHAR(90) DEFAULT NULL,
    create_time TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 3. 配置

```bash
# 复制环境变量模板，修改密码
cd service/api/server
cp .env.example .env
# 编辑 .env 中的 DB_PASS
```

### 4. 启动

```bash
# 终端 1：API
cd service/api/server && go run .

# 终端 2：Controller
cd service/controller/server && go run .

# 终端 3：Gateway
cd service/gateway/server && go run .
```

### 5. 测试

```bash
# WebSocket 客户端测试
cd service/gateway/server && go run test_client.go

# 或 HTTP 测试
curl -X POST http://localhost:9001/api/v1/user/login \
  -H "Content-Type: application/json" \
  -d '{"account":"admin","password":"123456"}'
```

### 6. Proto 代码生成

```bash
# 修改 .proto 后重新生成桩代码
cd service/xxx
trpc create -p proto/xxx.proto -d ../../proto -o stub --rpconly --nogomod

# 修复生成路径（trpc create 已知问题）
mv stub/service/*/proto/*.pb.go stub/ && rm -rf stub/service stub/tmp-*
```

### 7. 链路追踪（可选）

```bash
# 下载 Jaeger Windows 版：https://github.com/jaegertracing/jaeger/releases
.\jaeger-all-in-one.exe

# 浏览器打开
http://localhost:16686
```
