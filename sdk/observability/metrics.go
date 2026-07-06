// Package observability 预定义业务指标（Counter / Histogram / Gauge）。
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ─── 业务指标定义 ───

var (
	// RPCRequestCounter RPC 请求计数器（Filter 自动采集），标签: service, method, status
	RPCRequestCounter metric.Int64Counter

	// RPCLatency RPC 请求延迟直方图（Filter 自动采集），标签: service, method
	RPCLatency metric.Int64Histogram

	// MessageCounter 消息处理计数器（业务代码采集），标签: chat_type, result
	MessageCounter metric.Int64Counter

	// MessageLatency 消息处理延迟直方图（业务代码采集），标签: chat_type
	MessageLatency metric.Int64Histogram

	// ActiveConnections 活跃 WebSocket 连接数
	ActiveConnections metric.Int64UpDownCounter

	// PushToUserCounter Push 推送计数器
	PushToUserCounter metric.Int64Counter

	// DBQueryLatency 数据库查询延迟直方图
	DBQueryLatency metric.Int64Histogram
)

// InitBusinessMetrics 在 MeterProvider 就绪后调用，创建业务指标。
func InitBusinessMetrics() error {
	m := Meter()

	var err error

	// ─── RPC 指标（Filter 层自动采集） ───

	RPCRequestCounter, err = m.Int64Counter(
		"echat.rpc.requests",
		metric.WithDescription("RPC 请求总数（Filter 自动采集）"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	RPCLatency, err = m.Int64Histogram(
		"echat.rpc.latency",
		metric.WithDescription("RPC 请求延迟（Filter 自动采集）"),
		metric.WithUnit("us"),
	)
	if err != nil {
		return err
	}

	// ─── 业务指标（代码手动采集） ───

	MessageCounter, err = m.Int64Counter(
		"echat.messages.processed",
		metric.WithDescription("已处理的消息总数"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	MessageLatency, err = m.Int64Histogram(
		"echat.messages.latency",
		metric.WithDescription("消息处理延迟（微秒）"),
		metric.WithUnit("us"),
	)
	if err != nil {
		return err
	}

	ActiveConnections, err = m.Int64UpDownCounter(
		"echat.connections.active",
		metric.WithDescription("活跃 WebSocket 连接数"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}

	PushToUserCounter, err = m.Int64Counter(
		"echat.push.sent",
		metric.WithDescription("Push 推送消息总数"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	DBQueryLatency, err = m.Int64Histogram(
		"echat.db.query_latency",
		metric.WithDescription("数据库查询延迟（微秒）"),
		metric.WithUnit("us"),
	)
	if err != nil {
		return err
	}

	return nil
}

// ─── RPC 指标（Filter 调用） ───

// RecordRPCRequest 记录一次 RPC/HTTP 请求（由 metrics filter 调用）。
func RecordRPCRequest(ctx context.Context, service, method, status string, d time.Duration) {
	if RPCRequestCounter != nil {
		RPCRequestCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("service", service),
				attribute.String("method", method),
				attribute.String("status", status),
			),
		)
	}
	if RPCLatency != nil {
		RPCLatency.Record(ctx, d.Microseconds(),
			metric.WithAttributes(
				attribute.String("service", service),
				attribute.String("method", method),
			),
		)
	}
}

// ─── 业务指标（业务代码调用） ───

// RecordMessage 记录一条消息处理。
func RecordMessage(ctx context.Context, chatType, result string) {
	if MessageCounter == nil {
		return
	}
	MessageCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("chat_type", chatType),
			attribute.String("result", result),
		),
	)
}

// RecordMessageLatency 记录消息处理延迟。
func RecordMessageLatency(ctx context.Context, chatType string, d time.Duration) {
	if MessageLatency == nil {
		return
	}
	MessageLatency.Record(ctx, d.Microseconds(),
		metric.WithAttributes(attribute.String("chat_type", chatType)),
	)
}

// RecordConnection 记录连接数变化（+1 连接，-1 断开）。
func RecordConnection(ctx context.Context, delta int64) {
	if ActiveConnections == nil {
		return
	}
	ActiveConnections.Add(ctx, delta)
}

// RecordPush 记录一次 Push 推送。
func RecordPush(ctx context.Context, targetCount int64) {
	if PushToUserCounter == nil {
		return
	}
	PushToUserCounter.Add(ctx, targetCount)
}

// RecordDBQuery 记录数据库查询延迟。
func RecordDBQuery(ctx context.Context, operation string, d time.Duration) {
	if DBQueryLatency == nil {
		return
	}
	DBQueryLatency.Record(ctx, d.Microseconds(),
		metric.WithAttributes(attribute.String("operation", operation)),
	)
}
