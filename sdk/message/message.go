// Package message 定义 Controller 内部流转的 Message 结构体及相关枚举。
// 与 entity 包不同，Message 是运行时消息对象，非持久化模型。
package message

import "context"

// ============================================================
// 路由枚举
// ============================================================

// ChatType 会话类型
type ChatType string

const (
	ChatTypeSingle ChatType = "single" // 私聊
	ChatTypeGroup  ChatType = "group"  // 群聊
	ChatTypeAck    ChatType = "ack"    // 回执（已读/送达确认）
)

// ContentType 消息内容类型
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
	ContentTypeFile  ContentType = "file"
	ContentTypeAudio ContentType = "audio"
	ContentTypeVideo ContentType = "video"
)

// AckType 回执类型（存入 Ext["ack_type"]）
type AckType string

const (
	AckTypeDelivered AckType = "delivered" // 送达回执
	AckTypeRead      AckType = "read"      // 已读回执
)

// ============================================================
// Message 结构体
// ============================================================

// Message Controller 各模块间流转的统一消息体。
// 字段归属见 design docs: 公共SDK/消息模型.md
type Message struct {
	MsgType    string            // 路由类型：chat / group_chat / typing / read_receipt / delivery_ack / ping
	FromUID    string            // 发送者
	ToUID      string            // 目标用户（ack 类消息为空，由 Router 查原消息确定）
	GroupID    string            // 群聊 ID
	ChatType   string            // "single" / "group" / "ack"
	SessionID  string            // seq_id 的 Redis key 后缀
	MsgID      string            // 雪花算法生成
	SeqID      int64             // 会话内递增序号（Gateway 侧 Redis INCR 分配）
	ServerTime int64             // 服务端时间戳（ms）
	Content    []byte            // 消息内容（二进制兼容）
	Ext        map[string]string // key-value 扩展字段
	RequestID  string            // Gateway 生成的去重 key
	RawBody    []byte            // 完整 JSON 帧（Gateway 透传）
	Ctx        context.Context   // 请求上下文
	RespCh     chan *ACKResult   // 协程池 → Entry 回传 ACK
}

// ACKResult 协程池处理完成后回传给 Entry 的 ACK 结果
type ACKResult struct {
	MsgID      string
	SeqID      int64
	ServerTime int64
	Err        error
}
