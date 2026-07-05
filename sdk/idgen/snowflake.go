package idgen

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

// Snowflake 雪花算法 ID 生成器。
type Snowflake struct {
	node *snowflake.Node
}

// NewSnowflake 创建 Snowflake（workerID 1~1023，多实例需不同值）。
func NewSnowflake(workerID int64) (*Snowflake, error) {
	node, err := snowflake.NewNode(workerID)
	if err != nil {
		return nil, fmt.Errorf("snowflake init: %w", err)
	}
	return &Snowflake{node: node}, nil
}

// Generate 生成全局唯一 ID。
func (s *Snowflake) Generate() string {
	return s.node.Generate().String()
}
