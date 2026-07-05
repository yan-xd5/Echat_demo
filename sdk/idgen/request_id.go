// Package idgen 提供分布式 ID 生成工具。
package idgen

import (
	"fmt"
	"sync/atomic"
	"time"
)

var requestSeq atomic.Int64

// GenerateRequestID 生成 tRPC 去重用的 request_id。
// 格式: {gatewayID}_{unix_timestamp}_{sequence}
func GenerateRequestID(gatewayID string) string {
	seq := requestSeq.Add(1)
	if seq < 0 {
		requestSeq.Store(1) // 溢出后重置
		seq = 1
	}
	return fmt.Sprintf("%s_%d_%04d", gatewayID, time.Now().Unix(), seq%10000)
}
