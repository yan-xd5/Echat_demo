# eChat — tRPC-Go 即时通讯后端

基于 tRPC-Go 框架的分布式 IM 系统，三服务架构，支持 WebSocket 长连接、Polaris 服务发现、OpenTelemetry 全栈观测。

## 架构

### SDK 四层架构

| 层 | 职责 | 子包 | 依赖方向 |
|----|------|------|----------|
| 用例层 | 核心业务逻辑 | auth / route / message | → 仓储层 + 领域层 |
| 仓储层 | 数据访问封装 | mysql (10 Repo) / redis | → 领域层 |
| 领域层 | 纯数据结构 | entity | 无外部依赖 |
| 基础设施层 | 横切能力 | observability / idgen | 被所有层引用 |

### 服务分层

每个服务内部按 Level 分层，依赖 SDK 四层：

| 服务 | Level 3（入口） | Level 2（传输） | Level 1（业务） | 跨切 |
|------|----------------|----------------|----------------|------|
| API | main + config | trpc + restful | handler (用户/好友/群组/消息/文件) | filter (JWT 拦截) |
| Controller | main + config | — | handler + pipeline | filter (Token 校验) |
| Gateway | main + config | — | handler + session | — |

### 服务职责

| 服务 | 协议 | 端口 | 职责 |
| **Gateway** | WebSocket + tRPC | 9000 / 8003 | 长连接管理、协议转换、序列号生成、消息路由分发、跨网关顶号 |
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

## 端口规划

| 服务 | 端口 | 协议 | 用途 |
|------|------|------|------|
| Gateway WebSocket | 9000 | WS | 客户端长连接 |
| Gateway tRPC | 8003 | tRPC | Controller 回调推送 |
| Controller | 8002 | tRPC | 消息路由处理 |
| API tRPC | 8001 | tRPC | 内部服务调用 |
| API RESTful | 9001 | HTTP | 客户端 RESTful 接口 |
| pprof | 6061-6063 | HTTP | Go 性能剖析 |
| Grafana | 3000 | HTTP | 统一可视化 |
| Prometheus | 9090 | HTTP | 指标查询 |
| Jaeger | 16686 | HTTP | 链路追踪 |
| Pyroscope | 4040 | HTTP | 持续 Profiling |

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

**Bash:**
```bash
export MYSQL_DSN="root:pass@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true"
export JWT_SECRET="your-secret-key"
```

**PowerShell:**
```powershell
$env:MYSQL_DSN="root:pass@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true"
$env:JWT_SECRET="your-secret-key"
```

### 5. 启动

**Bash:**
```bash
cd service/gateway && go run .       # 终端 1：Gateway
cd service/controller && go run .    # 终端 2：Controller
cd service/api && go run .           # 终端 3：API
```

**PowerShell:**
```powershell
cd service/gateway; go run .         # 终端 1：Gateway
cd service/controller; go run .      # 终端 2：Controller
cd service/api; go run .             # 终端 3：API
```

### 6. 测试

**Bash:**
```bash
go run ./cmd/ws_test/                        # WS 全链路冒烟测试
PAIRS=50 MSGS=100 go run ./cmd/stress_test/  # 并发压力测试
ROUNDS=10 go run ./cmd/long_test/            # 长时间稳定性测试
go run ./cmd/api_test/                       # 37 RESTful 端点回归测试
```

**PowerShell:**
```powershell
go run ./cmd/ws_test/
$env:PAIRS="50"; $env:MSGS="100"; go run ./cmd/stress_test/
$env:ROUNDS="10"; go run ./cmd/long_test/
go run ./cmd/api_test/
```

### 7. 观测面板

浏览器打开：
- **Grafana**: http://localhost:3000 → 预置仪表板 `eChat - 服务概览` + `eChat - 链路追踪`
- **Jaeger**: http://localhost:16686 → 搜索 TraceID 查看调用链
- **Prometheus**: http://localhost:9090 → 查询 `echat_*` 指标
- **pprof**: http://localhost:6061/debug/pprof/ (API) / 6062 (Controller) / 6063 (Gateway)
