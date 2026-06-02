# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

eChat — 基于 tRPC-Go 的即时通讯（IM）后端，三服务分布式架构。Go 1.24.2，使用 Go workspace 管理多模块。

## 常用命令

```bash
# Proto stub 生成（proto 修改后执行）
cd service/xxx
trpc create -p proto/xxx.proto -d ../../proto -o stub --rpconly --nogomod

# pb.go 生成了错误嵌套路径时的修复
mv service/xxx/stub/service/*/proto/*.pb.go service/xxx/stub/ && rm -rf service/xxx/stub/service service/xxx/stub/tmp-*

# 运行各服务（必须在 server/ 目录下，因为 trpc_go.yaml 在那里）
cd service/api/server && go run .
cd service/controller/server && go run .
cd service/gateway/server && go run .

# 依赖管理（在各 service 目录下）
cd service/xxx && go mod tidy

# 测试链路
# 1. 启动 API（端口 8001 tRPC + 9001 HTTP）
# 2. 启动 Controller（端口 8002）
# 3. 运行 Gateway（一次性模拟客户端，调用 Controller 的四个场景）
```

## 架构：三服务分布式

```
客户端 (WebSocket) → Gateway (代理) → Controller (中控) → API (数据)
                  端口 9000/8003      端口 8002           端口 8001/9001
                                                              ↓
                                                           MySQL
```

| 服务 | 职责 | protocol | 对外端口 |
|------|------|----------|---------|
| **Gateway** | 长连接接入、协议转换（WebSocket↔tRPC） | tRPC client | 9000 (ws) |
| **Controller** | 消息路由、Token 校验、在线状态 | tRPC server + client | 8002 |
| **API** | 用户/好友/群组 CRUD、MySQL 读写 | tRPC + HTTP | 8001 (trpc) / 9001 (http) |

## 服务间调用链

```
Gateway ──tRPC──► Controller.AuthCheck / RouteMessage / UpdateStatus
                       │
                       └── tRPC ──► API.GetUserInfo / Login / ...
                                       │
                                       └── sqlx ──► MySQL
```

Controller 的 `trpc_go.yaml` 中配置了 `client` 段指向 API 地址。Gateway 通过代码 `client.WithTarget("ip://127.0.0.1:8002")` 调用 Controller。

## Module 与 import 路径

Go workspace (`go.work`) 包含四个模块：根 `echat` + 三个 service。proto 的 `go_package` 需与 import 路径一致：

```
echat (root)                    → import "echat/proto/common"
echat/service/api               → import "echat/service/api/stub"
echat/service/controller        → import "echat/service/controller/stub"
echat/service/gateway           → import "echat/service/gateway/stub"
```

## Filter 机制

Controller 通过 tRPC Filter 实现请求拦截。`filter.go` 中的 `serverFilter` 在每个 RPC 前按请求类型做 `switch` 断言：
- `AuthCheckRequest` → 解析 Token，将 `user_id`/`nickname` 写入 `context.Context`
- `RouteMessageRequest` → 校验 `FromUserId` 与 Token 身份一致性
- `UpdateStatusRequest` → 写入状态变更信息到 ctx

Filter 通过 `filter.Register("myServerFilter", serverFilter, nil)` 注册，在 `trpc_go.yaml` 的 `server.filter` 中启用。业务方法通过 `GetUserID(ctx)` / `GetNickname(ctx)` 从 ctx 读取 Filter 注入的信息。

## API 服务数据层

API 服务通过 `sqlx` + `go-sql-driver/mysql` 连接 MySQL。`db.go` 管理连接池，`repository/user_repo.go` 封装数据访问。数据库配置通过 `.env` 文件（`MYSQL_DSN` 或其拆分字段）管理，`config.go` 使用 `godotenv` 自动加载。

数据库表 `user` 主要字段：`uid`(PK), `username`, `account`(UNIQUE, 登录用), `password`, `gender`, `avatar`, `region`, `email`, `bio`。

## trpc create 已知问题

`trpc create --rpconly` 会把 `.pb.go` 生成到嵌套目录 `stub/service/xxx/proto/`，需要手动移动到 `stub/` 根下，mock 文件删除。每次重新生成 stub 前应 `rm -rf stub/` 清理旧文件。