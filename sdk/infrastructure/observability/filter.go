// Package observability — tRPC-Go metrics filter（RPC 请求计数 + 延迟 + 错误率）。
package observability

import (
	"context"
	"time"

	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/filter"
	"trpc.group/trpc-go/trpc-go/log"
)

func init() {
	filter.Register("metrics", metricsServerFilter, nil)
}

// metricsServerFilter 拦截所有 tRPC/HTTP 请求，自动采集 RPC 级指标。
func metricsServerFilter(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
	// 提取服务名和方法名
	msg := trpc.Message(ctx)
	service := "unknown"
	method := "unknown"
	if msg != nil {
		if s := msg.CalleeServiceName(); s != "" {
			service = s
		}
		if m := msg.ServerRPCName(); m != "" {
			method = m
		}
	}

	// 注入 HTTP 状态码 holder（供 writeJSON 回写状态码）
	ctx = WithHTTPStatusTracker(ctx)

	start := time.Now()

	// 执行业务逻辑
	rsp, err := next(ctx, req)

	elapsed := time.Since(start)

	// 判断状态
	status := "success"
	if err != nil {
		status = "error"
		log.Warnf("[Metrics] RPC 调用失败: service=%s, method=%s, err=%v", service, method, err)
	} else if httpStatus := HTTPStatusFromContext(ctx); httpStatus >= 400 {
		status = httpStatusText(httpStatus)
	}

	RecordRPCRequest(ctx, service, method, status, elapsed)
	return rsp, err
}

// httpStatusText 将 HTTP 状态码转换为简短的标签字符串。
func httpStatusText(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "success"
	case code >= 400 && code < 500:
		return "client_error"
	case code >= 500:
		return "server_error"
	default:
		return "success"
	}
}
