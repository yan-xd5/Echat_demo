# echat API 接口文档

## 基础信息

- **基础 URL**: `http://127.0.0.1:9001`
- **认证方式**: JWT Token (Bearer Token)
- **数据格式**: JSON
- **加密方式**: 密码使用 RSA-OAEP-SHA256 加密后 Base64 传输

## 响应格式

```json
{
    "code": 0,
    "message": "success",
    "data": {}
}
```

| code | 说明 |
|------|------|
| 0 | 成功 |
| 1 | 参数错误 |
| 999 | 业务错误（message 含详情） |

---

## 认证相关 API

### 获取公钥

- **URL**: `/api/v1/auth/public-key`
- **方法**: GET
- **场景**: 前端获取 RSA 公钥，用于加密后续请求中的密码。

```
无需 Token
```

**响应体：**
```json
{
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA..."
}
```

---

### 用户注册

- **URL**: `/api/v1/user/register`
- **方法**: POST
- **场景**: 新用户注册。密码需先获取公钥，RSA-OAEP-SHA256 加密后 Base64 传参。

```
无需 Token
```

**请求体：**
```json
{
    "account": "alice",
    "password": "K0BF2m1yPYgsSKRw1ZueadhTGvlBTPkjmfMH6Y7cjl+bp9KTEH6p...",
    "username": "Alice",
    "gender": "female",
    "region": "广东省",
    "email": null,
    "avatar": null,
    "bio": null
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| account | string | 是 | 账号（唯一） |
| password | string | 是 | RSA 加密 + Base64 后的密码 |
| username | string | 是 | 昵称 |
| gender | string | 否 | male / female / other，默认 other |
| region | string | 否 | 地区 |
| email | string | 否 | 邮箱 |
| avatar | string | 否 | 头像 URL |
| bio | string | 否 | 个人简介 |

**成功响应 (200)：**
```json
{
    "uid": "2073423471030833152",
    "account": "alice",
    "username": "Alice"
}
```

**错误响应：**
- 409: 账号已存在
- 500: 服务器内部错误

---

### 用户登录

- **URL**: `/api/v1/user/login`
- **方法**: POST
- **场景**: 已注册用户登录，返回 JWT Token。密码需 RSA 加密。

```
无需 Token
```

**请求体：**
```json
{
    "account": "alice",
    "password": "gDHNGj5VJj5hQp3DcB78TThT4bomqi/UiVXLPThUe0CKCr2m..."
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| account | string | 是 | 账号 |
| password | string | 是 | RSA 加密 + Base64 后的密码 |

**成功响应 (200)：**
```json
{
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expiresAt": "1783782452",
    "user": {
        "uid": "2073423471030833152",
        "account": "alice",
        "username": "Alice",
        "gender": "other",
        "region": null,
        "email": null,
        "avatar": null,
        "bio": null
    }
}
```

**错误响应：**
- 401: 账号或密码错误
- 500: 服务器内部错误

---

## 用户管理 API

### 获取用户信息

- **URL**: `/api/v1/user/info`
- **方法**: GET
- **场景**: 获取指定用户个人信息。

```
需要 Token
```

**请求头：**
```
Authorization: Bearer {token}
```

**Query 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uid | string | 是 | 用户 ID |

**成功响应 (200)：**
```json
{
    "user": {
        "uid": "787551337918238720",
        "account": "alice",
        "username": "Alice",
        "gender": "female",
        "region": "广东省",
        "email": null,
        "create_time": 1765603671,
        "avatar": null,
        "bio": null
    }
}
```

- 401: 未登录
- 404: 用户不存在

---

### 修改个人资料

- **URL**: `/api/v1/user/profile`
- **方法**: PUT
- **场景**: 已登录用户修改本人的个人资料，只传需要修改的字段。

```
需要 Token
```

**请求体：**
```json
{
    "username": "Alice",
    "gender": "female",
    "region": "广东省",
    "email": null,
    "avatar": null,
    "bio": "Hello World!"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 否 | 昵称 |
| gender | string | 否 | male / female / other |
| region | string | 否 | 地区 |
| email | string | 否 | 邮箱 |
| avatar | string | 否 | 头像 URL |
| bio | string | 否 | 个人简介 |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "修改成功"
}
```

---

### 搜索用户

- **URL**: `/api/v1/user/search`
- **方法**: POST
- **场景**: 通过关键词搜索平台上的其他用户。

```
需要 Token
```

**请求体：**
```json
{
    "keyword": "Alice",
    "limit": 20,
    "offset": 0
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| keyword | string | 是 | 搜索关键词（模糊匹配 username） |

**成功响应 (200)：**
```json
{
    "code": 0,
    "users": [
        {
            "uid": "787574772618760192",
            "account": "alice",
            "username": "Alice",
            "avatar": null
        }
    ]
}
```

---

### 按地区搜索用户

- **URL**: `/api/v1/user/search-by-region`
- **方法**: POST

```
需要 Token
```

**请求体：**
```json
{
    "region": "广东省"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "users": [
        {
            "uid": "787574772618760192",
            "account": "alice",
            "username": "Alice",
            "avatar": null
        }
    ]
}
```

---

### 批量获取用户信息

- **URL**: `/api/v1/user/batch`
- **方法**: POST
- **场景**: 根据 UID 列表批量获取用户基本信息。

```
需要 Token
```

**请求体：**
```json
{
    "uids": ["uid1", "uid2", "uid3"]
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "users": [
        {
            "uid": "uid1",
            "account": "user1",
            "username": "User1",
            "avatar": null
        }
    ]
}
```

---

### 注销账号

- **URL**: `/api/v1/user/account`
- **方法**: DELETE
- **场景**: 已登录用户注销自己的账号。

```
需要 Token
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "账号已注销"
}
```

---

## 好友管理 API

### 获取好友列表

- **URL**: `/api/v1/friend/list`
- **方法**: GET
- **场景**: 获取当前用户的所有好友。

```
需要 Token
```

**成功响应 (200)：**
```json
{
    "friends": [
        {
            "fid": "788309484421255168",
            "uid": "787574772618760192",
            "remark": null,
            "isBlacklist": false
        }
    ]
}
```

---

### 发送好友申请

- **URL**: `/api/v1/friend/apply`
- **方法**: POST
- **场景**: 向非好友用户发送好友申请。

```
需要 Token
```

**请求体：**
```json
{
    "to_uid": "787574772618760192",
    "apply_text": "Hi! I'd like to connect with you"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| to_uid | string | 是 | 对方 UID |
| apply_text | string | 否 | 申请附言 |

**成功响应 (200)：**
```json
{
    "req_id": "787657795192229888"
}
```

---

### 接受好友申请

- **URL**: `/api/v1/friend/accept`
- **方法**: POST
- **场景**: 同意好友申请，建立好友关系并创建私聊会话。

```
需要 Token
```

**请求体：**
```json
{
    "req_id": "787661626944786432"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| req_id | string | 是 | 申请 ID |

**成功响应 (200)：**
```json
{
    "fid": "788309484421255168"
}
```

---

### 拒绝好友申请

- **URL**: `/api/v1/friend/reject`
- **方法**: POST

```
需要 Token
```

**请求体：**
```json
{
    "req_id": "787661626944786432"
}
```

**成功响应 (200)：**
```json
{}
```

---

### 取消好友申请

- **URL**: `/api/v1/friend/cancel-request`
- **方法**: POST
- **场景**: 发送者撤回待处理的好友申请。

```
需要 Token
```

**请求体：**
```json
{
    "req_id": "787661626944786432"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已取消"
}
```

---

### 获取好友申请列表

- **URL**: `/api/v1/friend/requests`
- **方法**: GET
- **场景**: 获取收到的好友申请列表（默认）或发出的申请列表。

```
需要 Token
```

**Query 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| direction | string | 否 | sent=发出的，不传=收到的 |

**成功响应 (200)：**
```json
{
    "requests": [
        {
            "req_id": "reqId123",
            "sender_uid": "uid13",
            "apply_text": "Hello",
            "status": "pending",
            "create_time": 1765604936
        }
    ]
}
```

---

### 删除好友

- **URL**: `/api/v1/friend/{fid}`
- **方法**: DELETE
- **场景**: 删除好友关系及关联的私聊会话。

```
需要 Token
```

**Path 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| fid | string | 是 | 好友关系 ID |

**成功响应 (200)：**
```json
{}
```

---

### 拉黑好友

- **URL**: `/api/v1/friend/blacklist`
- **方法**: POST

```
需要 Token
```

**请求体：**
```json
{
    "fid": "friend_001"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已拉黑"
}
```

---

### 取消拉黑

- **URL**: `/api/v1/friend/unblacklist`
- **方法**: POST

```
需要 Token
```

**请求体：**
```json
{
    "fid": "friend_001"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已取消拉黑"
}
```

---

### 黑名单列表

- **URL**: `/api/v1/friend/blacklist-list`
- **方法**: POST

```
需要 Token
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "friends": [
        {
            "fid": "friend_001",
            "uid": "user_123"
        }
    ]
}
```

---

## 会话管理 API

### 获取会话列表

- **URL**: `/api/v1/chat/conversations`
- **方法**: POST
- **场景**: 获取当前用户的所有会话（私聊 + 群聊），包含最后一条消息、未读数、置顶状态。

```
需要 Token
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "conversations": [
        {
            "chat_id": "789376146465624064",
            "chat_type": "private",
            "name": "Alice",
            "avatar": null,
            "last_msg": "你好",
            "last_time": 1766038804,
            "unread_count": 3,
            "is_pinned": false
        },
        {
            "chat_id": "789370704574287872",
            "chat_type": "group",
            "name": "Echat开发组",
            "avatar": null,
            "last_msg": "[图片]",
            "last_time": 1766038455,
            "unread_count": 5,
            "is_pinned": true
        }
    ]
}
```

---

### 会话置顶/取消置顶

- **URL**: `/api/v1/chat/pin`
- **方法**: POST

```
需要 Token
```

**请求体：**
```json
{
    "chat_id": "789376146465624064",
    "chat_type": "private",
    "is_pinned": true
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| chat_id | string | 是 | 会话 ID |
| chat_type | string | 是 | private / group |
| is_pinned | bool | 是 | 是否置顶 |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已更新"
}
```

---

## 在线状态 API

### 批量查询在线状态

- **URL**: `/api/v1/chat/online-status`
- **方法**: POST
- **场景**: 批量查询指定用户的在线状态。

```
需要 Token
```

**请求体：**
```json
{
    "uids": ["uid1", "uid2", "uid3"]
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "online": {
        "uid1": true,
        "uid2": false
    }
}
```

---

### 群在线成员

- **URL**: `/api/v1/group/online-members`
- **方法**: POST

```
需要 Token
```

**请求体：**
```json
{
    "gid": "1234"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "online_uids": ["uid1", "uid2"]
}
```

---

## 消息管理 API

### 获取历史消息

- **URL**: `/api/v1/message/history`
- **方法**: GET
- **场景**: 获取私聊或群聊的历史消息。

```
需要 Token
```

**Query 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| chat_id | string | 是 | 会话 ID |
| chat_type | string | 是 | private / group |
| before | int64 | 否 | 时间戳（ms），查此时间之前的消息 |
| limit | int32 | 否 | 每页条数，默认 50 |

**成功响应 (200)：**
```json
{
    "messages": [
        {
            "msg_id": "msg_001",
            "sender_uid": "user_12345",
            "content": "Hello!",
            "type": "text",
            "seq_id": 1,
            "send_time": 1766038804
        }
    ]
}
```

---

### 标记消息已读

- **URL**: `/api/v1/message/read`
- **方法**: POST
- **场景**: 标记会话中的消息为已读。

```
需要 Token
```

**请求体：**
```json
{
    "chat_id": "chat_67890",
    "chat_type": "private",
    "msg_id": null
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| chat_id | string | 是 | 会话 ID |
| chat_type | string | 是 | private / group |
| msg_id | string | 否 | 群聊时指定最后一条已读消息 ID |

**成功响应 (200)：**
```json
{
    "affected": 5
}
```

---

### 撤回消息

- **URL**: `/api/v1/message/revoke`
- **方法**: POST
- **场景**: 撤回自己发送的消息（仅发送者可操作）。

```
需要 Token
```

**请求体：**
```json
{
    "msg_id": "123456",
    "chat_type": "private"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| msg_id | string | 是 | 消息 ID |
| chat_type | string | 是 | private / group |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已撤回"
}
```

**错误响应：**
- 403: 只能撤回自己的消息

---

### 获取未读消息数

- **URL**: `/api/v1/message/unread-count`
- **方法**: POST
- **场景**: 获取指定会话的未读消息数量。

```
需要 Token
```

**请求体：**
```json
{
    "chat_id": "chat_67890",
    "chat_type": "private"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "unread_count": 5
}
```

---

### 获取未读消息列表

- **URL**: `/api/v1/message/unread-list`
- **方法**: POST
- **场景**: 获取指定会话的未读消息内容。

```
需要 Token
```

**请求体：**
```json
{
    "chat_id": "chat_67890",
    "chat_type": "private"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "messages": [
        {
            "msg_id": "msg_001",
            "content": "Hello!",
            "sender_uid": "user_123",
            "send_time": 1766038804
        }
    ]
}
```

---

### 获取消息已读用户列表

- **URL**: `/api/v1/message/read-users`
- **方法**: POST
- **场景**: 查看指定消息的已读用户。

```
需要 Token
```

**请求体：**
```json
{
    "msg_id": "123456"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "uids": ["uid1", "uid2"]
}
```

---

### 获取消息已读人数

- **URL**: `/api/v1/message/read-counts`
- **方法**: POST
- **场景**: 批量获取多条消息的已读人数。

```
需要 Token
```

**请求体：**
```json
{
    "msg_ids": ["msgid1", "msgid2", "msgid3"]
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "read_counts": {
        "msgid1": 44,
        "msgid2": 23
    }
}
```

---

### 获取会话消息总数

- **URL**: `/api/v1/message/chat-count`
- **方法**: POST
- **场景**: 获取指定会话的消息总数。

```
需要 Token
```

**请求体：**
```json
{
    "chat_id": "chat_67890",
    "chat_type": "private"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "count": 1100
}
```

---

## 群聊管理 API

### 创建群聊

- **URL**: `/api/v1/group/create`
- **方法**: POST
- **场景**: 创建新的群聊，创建者自动成为群主。

```
需要 Token
```

**请求体：**
```json
{
    "group_name": "Echat",
    "group_intro": null
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| group_name | string | 是 | 群名称 |
| group_intro | string | 否 | 群简介 |

**成功响应 (200)：**
```json
{
    "gid": "787350655340646400"
}
```

---

### 申请加入群聊

- **URL**: `/api/v1/group/join`
- **方法**: POST
- **场景**: 申请加入群聊，需群主或管理员审批后生效。

```
需要 Token
```

**请求体：**
```json
{
    "gid": "789370704574287872",
    "apply_text": "I want to join"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gid | string | 是 | 群 ID |
| apply_text | string | 否 | 申请附言 |

**成功响应 (200)：**
```json
{
    "req_id": "789384469353074688"
}
```

**错误响应：**
- 403: 已是群成员
- 409: 已有待审批的申请

---

### 退出群聊

- **URL**: `/api/v1/group/leave`
- **方法**: POST
- **场景**: 退出群聊。群主不能直接退出，需先转让群主。

```
需要 Token
```

**请求体：**
```json
{
    "gid": "787547607969828864"
}
```

**成功响应 (200)：**
```json
{}
```

**错误响应：**
- 403: 群主不能直接退出，请先转让群主
- 404: 不是群成员

---

### 获取群成员列表

- **URL**: `/api/v1/group/members`
- **方法**: GET
- **场景**: 查看群聊所有成员，需是群成员才能查看。

```
需要 Token
```

**Query 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gid | string | 是 | 群 ID |

**成功响应 (200)：**
```json
{
    "members": [
        {
            "uid": "787350530115506176",
            "role": "owner",
            "join_time": 1765602781
        },
        {
            "uid": "787551337918238720",
            "role": "member",
            "join_time": 1765700000
        }
    ]
}
```

---

### 获取我的群列表

- **URL**: `/api/v1/group/my`
- **方法**: GET
- **场景**: 获取当前用户加入的所有群聊。

```
需要 Token
```

**成功响应 (200)：**
```json
{
    "groups": [
        {
            "gid": "787547607969828864",
            "group_name": "Echat",
            "myRole": "member",
            "member_count": 10
        }
    ]
}
```

---

### 搜索群聊

- **URL**: `/api/v1/group/search`
- **方法**: POST
- **场景**: 通过关键词搜索群聊。

```
需要 Token
```

**请求体：**
```json
{
    "keyword": "Echat"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "groups": [
        {
            "gid": "787547607969828864",
            "group_name": "Echat",
            "group_avatar": null,
            "group_intro": null,
            "member_count": 10
        }
    ]
}
```

---

### 踢出群成员

- **URL**: `/api/v1/group/kick`
- **方法**: POST
- **场景**: 群主或管理员踢出普通成员。管理员不能踢群主，不能踢自己。

```
需要 Token（群主或管理员）
```

**请求体：**
```json
{
    "gid": "gid123",
    "target_uid": "user_123"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gid | string | 是 | 群 ID |
| target_uid | string | 是 | 被踢出的群员 UID |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已踢出"
}
```

**错误响应：**
- 403: 无权操作 / 不能踢出群主

---

### 禁言群成员

- **URL**: `/api/v1/group/mute`
- **方法**: POST
- **场景**: 群主或管理员禁言群成员。-1 为永久禁言。

```
需要 Token（群主或管理员）
```

**请求体：**
```json
{
    "gid": "787547607969828864",
    "target_uid": "787574772618760192",
    "duration": 3600
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gid | string | 是 | 群 ID |
| target_uid | string | 是 | 被禁言用户 UID |
| duration | int | 是 | 禁言秒数（-1=永久） |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已禁言",
    "ban_id": "ban_123456"
}
```

---

### 解除禁言

- **URL**: `/api/v1/group/unmute`
- **方法**: POST
- **场景**: 群主或管理员解除群成员的禁言状态。

```
需要 Token（群主或管理员）
```

**请求体：**
```json
{
    "gid": "gid123",
    "ban_id": "ban_123456"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gid | string | 是 | 群 ID |
| ban_id | string | 是 | 禁言记录 ID |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已解除禁言"
}
```

---

### 修改成员角色

- **URL**: `/api/v1/group/role`
- **方法**: PUT
- **场景**: 群主设置/取消管理员。仅群主可操作。

```
需要 Token（仅群主）
```

**请求体：**
```json
{
    "gid": "gid123",
    "target_uid": "uid23",
    "role": "admin"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gid | string | 是 | 群 ID |
| target_uid | string | 是 | 目标用户 UID |
| role | string | 是 | admin / member |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "角色已修改"
}
```

---

### 获取入群申请列表（管理员视角）

- **URL**: `/api/v1/group/requests`
- **方法**: POST
- **场景**: 群主或管理员查看待处理的入群申请。

```
需要 Token（群主或管理员）
```

**请求体：**
```json
{
    "gid": "787547607969828864"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "requests": [
        {
            "req_id": "787556646380376064",
            "gid": "787547607969828864",
            "applicant_uid": "787551337918238720",
            "apply_text": "I want to join",
            "create_time": 1765604936
        }
    ]
}
```

---

### 全部入群申请历史

- **URL**: `/api/v1/group/all-requests`
- **方法**: POST
- **场景**: 群主或管理员查看全部（含已处理）入群申请。

```
需要 Token（群主或管理员）
```

**请求体：**
```json
{
    "gid": "787547607969828864"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "requests": [
        {
            "req_id": "787556646380376064",
            "gid": "787547607969828864",
            "applicant_uid": "787551337918238720",
            "approver_uid": null,
            "status": "pending",
            "apply_text": "I want to join",
            "create_time": 1765604936,
            "handle_time": 0
        }
    ]
}
```

---

### 审批入群申请

- **URL**: `/api/v1/group/approve-request`
- **方法**: POST
- **场景**: 群主或管理员审批入群申请，同意后申请人加入群聊。

```
需要 Token（群主或管理员）
```

**请求体：**
```json
{
    "req_id": "787556646380376064",
    "approve": true
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| req_id | string | 是 | 申请 ID |
| approve | bool | 是 | true=同意, false=拒绝 |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已处理"
}
```

---

### 我的入群申请

- **URL**: `/api/v1/group/my-requests`
- **方法**: POST
- **场景**: 查看自己发出的入群申请记录。

```
需要 Token
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "requests": [
        {
            "req_id": "789384469353074688",
            "gid": "789370704574287872",
            "status": "accepted",
            "apply_text": "I want to join",
            "create_time": 1766040723
        }
    ]
}
```

---

### 解散群聊

- **URL**: `/api/v1/group/disband`
- **方法**: POST
- **场景**: 群主解散群聊。仅群主可操作。

```
需要 Token（仅群主）
```

**请求体：**
```json
{
    "gid": "787312090246287360"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "群已解散"
}
```

**错误响应：**
- 403: 只有群主能解散群

---

### 我创建的群

- **URL**: `/api/v1/group/owned`
- **方法**: POST
- **场景**: 查看当前用户创建的群聊。

```
需要 Token
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "groups": [
        {
            "gid": "787547607969828864",
            "group_name": "Echat",
            "group_avatar": null,
            "member_count": 10
        }
    ]
}
```

---

### 禁言列表

- **URL**: `/api/v1/group/mute-list`
- **方法**: POST
- **场景**: 查看群聊禁言记录。

```
需要 Token（群成员）
```

**请求体：**
```json
{
    "gid": "787547607969828864"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "mutes": [
        {
            "ban_id": "ban_001",
            "uid": "user_123",
            "duration": 3600,
            "start_time": 1765604936
        }
    ]
}
```

---

### 群公告列表

- **URL**: `/api/v1/group/announces`
- **方法**: POST
- **场景**: 查看群聊公告列表。

```
需要 Token（群成员）
```

**请求体：**
```json
{
    "gid": "787547607969828864"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "announces": [
        {
            "msg_id": "123",
            "content": "HELLO",
            "sender_uid": "user123",
            "send_time": 1766038804
        }
    ]
}
```

---

## 文件管理 API

### 上传文件

- **URL**: `/api/v1/file/upload`
- **方法**: POST
- **场景**: 上传文件，SHA256 自动去重。

```
需要 Token
Content-Type: application/json
```

**请求体：**
```json
{
    "file_name": "photo.png",
    "file_type": "image",
    "mime_type": "image/png",
    "file_data": "iVBORw0KGgoAAAANSUhEUgAA..."
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_name | string | 是 | 文件名（含扩展名） |
| file_type | string | 是 | image / video / file / audio |
| mime_type | string | 否 | MIME 类型 |
| file_data | bytes | 是 | Base64 编码的文件内容 |

**成功响应 (200)：**
```json
{
    "file_id": "file_7d8e9f0a1b2c3d4",
    "file_url": "/api/v1/file/download?file_id=file_7d8e9f0a1b2c3d4"
}
```

---

### 下载文件

- **URL**: `/api/v1/file/download`
- **方法**: GET
- **场景**: 下载文件（需有权限）。

```
需要 Token
```

**Query 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

**成功响应 (200)：**
```json
{
    "file_name": "photo.png",
    "mime_type": "image/png",
    "file_data": "iVBORw0KGgoAAAANSUhEUgAA..."
}
```

**错误响应：**
- 403: 无权下载
- 404: 文件不存在

---

### 预览文件

- **URL**: `/api/v1/file/preview`
- **方法**: GET
- **场景**: 预览文件（复用下载逻辑）。

```
需要 Token
```

**Query 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

**成功响应 (200)：**
```json
{
    "file_name": "photo.png",
    "mime_type": "image/png",
    "file_data": "iVBORw0KGgoAAAANSUhEUgAA..."
}
```

---

### 删除文件

- **URL**: `/api/v1/file/{file_id}`
- **方法**: DELETE
- **场景**: 软删除文件（仅所有者可操作）。

```
需要 Token
```

**Path 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

**成功响应 (200)：**
```json
{}
```

**错误响应：**
- 403: 无权删除

---

### 文件列表

- **URL**: `/api/v1/file/list`
- **方法**: GET
- **场景**: 获取当前用户的文件列表。

```
需要 Token
```

**Query 参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| limit | int32 | 否 | 每页条数，默认 20 |
| offset | int32 | 否 | 偏移量 |

**成功响应 (200)：**
```json
{
    "files": [
        {
            "file_id": "file_001",
            "original_name": "photo.png",
            "file_type": "image",
            "file_size": 2456789,
            "mime_type": "image/png",
            "upload_time": 1765603671,
            "download_count": 10
        }
    ],
    "total": 1
}
```

---

### 设置文件权限

- **URL**: `/api/v1/file/permission`
- **方法**: POST
- **场景**: 文件所有者对指定用户/群/公开授予文件权限。

```
需要 Token（文件所有者）
```

**请求体：**
```json
{
    "file_id": "file_001",
    "access_type": "user",
    "target_id": "uid_123",
    "permission_level": "download"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |
| access_type | string | 是 | user / friend / group / public |
| target_id | string | 否 | 授权目标 ID |
| permission_level | string | 是 | view / download / share / manage |

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "权限已设置",
    "permission_id": "perm_001"
}
```

**错误响应：**
- 403: 只有文件所有者能设置权限

---

### 撤销文件权限

- **URL**: `/api/v1/file/revoke-permission`
- **方法**: POST
- **场景**: 文件所有者撤销已授予的文件权限。

```
需要 Token（文件所有者）
```

**请求体：**
```json
{
    "file_id": "file_001",
    "access_type": "user",
    "target_id": "uid_123"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "affected": 1
}
```

---

### 文件关联列表

- **URL**: `/api/v1/file/associations`
- **方法**: POST
- **场景**: 查询文件的关联关系（按 file_id 查关联，或按关联类型+ID 查文件）。

```
需要 Token
```

**请求体：**
```json
{
    "file_id": "file_001",
    "association_type": null,
    "associated_id": null
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 二选一 | 文件 ID |
| association_type | string | 二选一 | private_message / group_message / user_avatar / group_avatar |
| associated_id | string | 二选一 | 关联对象 ID |

**成功响应 (200)：**
```json
{
    "code": 0,
    "associations": [
        {
            "association_id": "assoc_001",
            "file_id": "file_001",
            "association_type": "private_message",
            "associated_id": "msg_123",
            "creator_uid": "user_001",
            "create_time": 1765603671
        }
    ]
}
```

---

### 创建文件关联

- **URL**: `/api/v1/file/associate`
- **方法**: POST
- **场景**: 将文件关联到消息、头像等对象。

```
需要 Token
```

**请求体：**
```json
{
    "file_id": "file_001",
    "association_type": "private_message",
    "associated_id": "msg_123"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已关联",
    "association_id": "assoc_001"
}
```

---

### 删除文件关联

- **URL**: `/api/v1/file/delete-association`
- **方法**: POST
- **场景**: 删除文件的关联关系。

```
需要 Token
```

**请求体：**
```json
{
    "association_id": "assoc_001"
}
```

**成功响应 (200)：**
```json
{
    "code": 0,
    "message": "已删除"
}
```

---

## WebSocket API

### 建立连接

- **URL**: `ws://127.0.0.1:9000/ws?ticket={jwt_token}`
- **场景**: 用户登录后，客户端通过 WebSocket 与服务器建立持久连接，用于实时消息推送。

**连接参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| ticket | string | JWT Token（与 HTTP API 相同） |

### 客户端 → 服务端

**发送聊天消息：**
```json
{
    "seq": 1,
    "type": "chat",
    "to": "user_67890",
    "content": {
        "text": "Hello, how are you?"
    }
}
```

**发送群聊消息：**
```json
{
    "seq": 2,
    "type": "group_chat",
    "group_id": "787547607969828864",
    "content": {
        "text": "Hello everyone!"
    }
}
```

**心跳：**
```json
{"type": "ping"}
```

### 服务端 → 客户端

**消息 ACK：**
```json
{
    "seq": 1,
    "type": "ack",
    "msg_id": "2073405398450184192",
    "seq_id": 6,
    "server_time": 1783173343657
}
```

**新消息推送：**
```json
{
    "type": "push",
    "from": "user_12345",
    "msg_id": "2073405398450184192",
    "seq_id": 6,
    "server_time": 1783173343657,
    "content": "Hello!"
}
```

**错误响应：**
```json
{
    "seq": 1,
    "type": "error",
    "error": "不是好友"
}
```

**心跳响应：**
```json
{"type": "pong", "server_time": 1783173343657}
```

---

## 通用错误码

| HTTP 状态码 | 说明 |
|------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未登录或 Token 无效 |
| 403 | 无权操作 |
| 404 | 资源不存在 |
| 409 | 资源冲突（如重复申请） |
| 500 | 服务器内部错误 |
