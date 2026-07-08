# eChat — tRPC-Go 即时通讯后端

基于 tRPC-Go 框架的分布式 IM 系统，三服务架构，支持 WebSocket 长连接、Polaris 服务发现、OpenTelemetry 全栈观测。

## 架构

### 分层设计

```
transport ──→ usecase ──→ repository ──→ domain
    │              │
    └──────────────┴──→ infrastructure
```

| 层 | 目录 | 职责 |
|----|------|------|
| **domain** | `sdk/domain/entity/` | 数据实体，不依赖任何层 |
| **repository** | `sdk/repository/mysql/` `redis/` | 数据访问，只依赖 domain |
| **usecase** | `sdk/usecase/auth/` `message/` `route/` | 业务逻辑，依赖 domain + repository |
| **infrastructure** | `sdk/infrastructure/observability/` `idgen/` | 观测、ID生成，被所有层引用 |
| **transport** | `service/*/internal/` | 传输层，依赖 usecase（每个服务独立分层） |

### 服务拓扑

```
Client ──WebSocket──→ Gateway(:9000) ──→ Controller(:8002) ──→ MySQL
                           │                      │
                       Polaris                Redis(seq_id/路由/在线状态)
                           │
Client ──HTTP REST──→ API(:9001) ──→ MySQL + Redis
```

| 服务 | 协议 | 端口 | 职责 |
|------|------|------|------|
| **Gateway** | WebSocket + tRPC | 9000 / 8003 | 长连接接入、协议转换、连接管理、跨网关顶号 |
| **Controller** | tRPC | 8002 | 消息路由、去重、持久化、Push 推送、Token 校验 |
| **API** | tRPC + RESTful | 8001 / 9001 | 用户/好友/群组/文件 CRUD、认证、RSA 加密 |

## 技术栈

| 分类 | 技术 |
|------|------|
| 框架 | tRPC-Go v1.0.4 |
| 数据库 | MySQL 8.0 (sqlx) |
| 缓存 | Redis (goredis 插件) |
| 服务发现 | Polaris (trpc-naming-polarismesh) |
| WebSocket | gorilla/websocket |
| 认证 | JWT (HS256) + RSA-2048-OAEP 密码加密 |
| ID 生成 | Snowflake (bwmarrin) |
| 协程池 | ants |
| 序列化 | Protobuf + JSON |
| 观测 | OpenTelemetry (Tracing + Metrics) + pprof |
| 可视化 | Jaeger + Prometheus + Grafana + Pyroscope |
| 包管理 | Go Workspace (4 模块) |

## 目录结构

```
echat/
├── sdk/                             # 共享代码（分 4 层）
│   ├── domain/entity/               # 领域层：数据实体
│   ├── repository/                  # 数据访问层
│   │   ├── mysql/                   # MySQL Repository（10 文件）
│   │   └── redis/                   # Redis 操作
│   ├── usecase/                     # 业务逻辑层
│   │   ├── auth/                    # JWT + RSA
│   │   ├── message/                 # 消息结构
│   │   └── route/                   # 会话路由 + 服务发现
│   └── infrastructure/             # 基础设施层
│       ├── observability/           # OTel Metrics + pprof
│       └── idgen/                   # Snowflake ID
├── service/                         # 微服务（每个独立 go.mod，可单独部署）
│   ├── api/                         # API 服务（3 根文件 + 5 子包）
│   │   ├── internal/
│   │   │   ├── handler/             #   tRPC handler（User/Friend/Group/Message/File/Auth）
│   │   │   ├── restful/             #   RESTful handler（9 文件，58 个 API 端点）
│   │   │   ├── trpc/                #   服务注册（ServiceDesc + Register*）
│   │   │   ├── shared/              #   共享工具（GetUID/WriteJSON/MsgError/MsgSuccess）
│   │   │   └── filter/              #   auth filter
│   │   └── stub/
│   ├── controller/                  # 中控服务（2 根文件 + 3 子包）
│   │   ├── internal/
│   │   │   ├── handler/             #   消息路由/AuthCheck handler
│   │   │   ├── pipeline/            #   消息处理管线（Pipeline/Entry/Pool/IDGen）
│   │   │   └── filter/              #   Token 校验 serverFilter
│   │   └── stub/
│   └── gateway/                     # 网关服务（2 根文件 + 2 子包）
│       ├── internal/
│       │   ├── session/             #   WS 会话管理（Session/ConnManager/Gateway/Kick）
│       │   └── handler/             #   WS AuthHandler + tRPC PushService
│       └── stub/
├── cmd/                             # 测试工具（4 个）
│   ├── ws_test/                     # WS 集成测试（单条消息）
│   ├── stress_test/                 # 压力测试（并发消息）
│   ├── long_test/                   # 长时间稳定性测试（私聊+群聊+API）  
│   └── api_test/                    # 全 API 端点测试（37 RESTful）
├── deploy/                          # 部署配置
│   ├── docker-compose.observability.yaml
│   ├── otel-collector-config.yaml
│   ├── prometheus.yml
│   └── grafana-provisioning/        # Grafana 统一观测仪表板
├── proto/common/                    # 共享 Proto
└── docs/
```

## 功能

### 已实现

- [x] 用户注册/登录（RSA-OAEP 加密密码 + bcrypt 存储）
- [x] JWT Token 签发与校验（7 天有效期）
- [x] 好友管理（申请/审批/删除/拉黑）
- [x] 群组管理（创建/加入审批/踢人/禁言/解散）
- [x] 私聊 + 群聊消息收发（全链路 WS → Gateway → Controller → MySQL → Push）
- [x] 消息去重（Redis SETNX）、已读回执、撤回
- [x] 文件上传/下载/权限管理/关联
- [x] 会话列表（置顶、未读数）
- [x] 在线状态查询、跨网关顶号
- [x] 58 个 RESTful API 端点
- [x] auth filter 集中拦截（非公开路由强制认证）
- [x] 优雅关机（全部三服务）
- [x] OpenTelemetry 全栈观测（Tracing + Metrics + Profiling）
- [x] Grafana 预置仪表板（RPC 概览 + 业务指标 + 链路追踪）
- [x] Polaris 服务发现（统一走 tRPC naming/discovery）
- [x] Redis 统一走 tRPC goredis 插件

## 服务端口

| 服务 | 端口 | 协议 | 绑定 |
|------|------|------|------|
| API tRPC | 8001 | tRPC | 127.0.0.1 |
| API RESTful | 9001 | HTTP | 127.0.0.1 |
| Controller | 8002 | tRPC | 127.0.0.1 |
| Gateway tRPC | 8003 | tRPC | 127.0.0.1 |
| Gateway WebSocket | 9000 | WS | 0.0.0.0 |
| pprof (×3) | 6061-6063 | HTTP | 127.0.0.1 |

## 观测端口

| 服务 | 端口 | 用途 |
|------|------|------|
| Jaeger UI | 16686 | 链路追踪 |
| Prometheus | 9090 | 指标查询 |
| Grafana | 3000 | 统一可视化 (admin/admin) |
| Pyroscope | 4040 | 持续 Profiling |

## 快速开始

### 1. 环境要求

- Go 1.24+
- MySQL 8.0+（数据库 `chat`）
- Redis
- Docker（Polaris + 观测栈）

### 2. 启动基础设施

```bash
# Polaris
docker run -d --name polaris -p 8090:8090 -p 8091:8091 polarismesh/polaris-standalone:latest

# 观测栈（可选）
docker compose -f deploy/docker-compose.observability.yaml up -d
```

### 3. 数据库

```sql
CREATE DATABASE chat DEFAULT CHARSET utf8mb4;
-- 完整建表语句见 sdk/mysql/schema.sql
```

### 4. 配置

```bash
# API 通过以下环境变量连接 MySQL：
# DB_HOST=127.0.0.1  DB_PORT=3306  DB_USER=root  DB_PASS=your_password  DB_NAME=chat
# Controller + Gateway 使用 MYSQL_DSN
export MYSQL_DSN="root:pass@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true"
export JWT_SECRET="your-secret-key"
```

### 5. 启动

```bash
# 设置环境变量（PowerShell: $env:KEY="value" / Bash: export KEY="value"）
JWT_SECRET=your-secret
MYSQL_DSN=root:pass@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true

# 终端 1：Gateway
cd service/gateway && go run .

# 终端 2：Controller
cd service/controller && go run .

# 终端 3：API
cd service/api && go run .
```

### 6. 测试

```bash
# 环境变量
export JWT_SECRET="your-secret"
export MYSQL_DSN="root:pass@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true"

# WebSocket 全链路（单条消息，验证端到端）
go run ./cmd/ws_test/

# 压力测试（并发消息，默认 10 对 × 50 条）
PAIRS=50 MSGS=100 go run ./cmd/stress_test/

# 长时间稳定性（双 WS 在线，交替私聊+群聊+API，默认 10 轮）
ROUNDS=10 MINUTES=1 go run ./cmd/long_test/

# 全 API 端点测试（37 RESTful 端点逐个验证，自动获取 Token）
go run ./cmd/api_test/
```

### 7. 观测面板

浏览器打开：
- **Grafana**: http://localhost:3000 → 预置仪表板 `eChat - 服务概览` + `eChat - 链路追踪`
- **Jaeger**: http://localhost:16686 → 搜索 TraceID 查看调用链
- **Prometheus**: http://localhost:9090 → 查询 `echat_*` 指标
- **pprof**: http://localhost:6061/debug/pprof/ (API) / 6062 (Controller) / 6063 (Gateway)
