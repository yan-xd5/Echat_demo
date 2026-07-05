package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"trpc.group/trpc-go/trpc-go/log"
)

const kickChannel = "gateway:kick"

// kickMsg 跨网关顶号通知。
type kickMsg struct {
	UID       string `json:"uid"`
	Platform  string `json:"platform"`
	GatewayID string `json:"gateway_id"`
}

// publishKick 向 Redis Pub/Sub 发布顶号通知。
func publishKick(ctx context.Context, rdb *redis.Client, gatewayID, uid, platform string) {
	msg, _ := json.Marshal(kickMsg{UID: uid, Platform: platform, GatewayID: gatewayID})
	if err := rdb.Publish(ctx, kickChannel, msg).Err(); err != nil {
		log.Warnf("[Gateway] 发布顶号通知失败: %v", err)
	}
}

// startKickListener 订阅 Redis Pub/Sub，接收其他 Gateway 的顶号通知。
// 连接断开后自动重连。
func startKickListener(ctx context.Context, rdb *redis.Client, connMgr *ConnManager, ownGatewayID string) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			sub := rdb.Subscribe(ctx, kickChannel)
			ch := sub.Channel()

			for {
				select {
				case <-ctx.Done():
					sub.Close()
					return
				case msg, ok := <-ch:
					if !ok {
						// channel 关闭（Redis 断连），跳出内层循环重连
						sub.Close()
						log.Warn("[Gateway] 顶号 Pub/Sub 断连，5s 后重连...")
						select {
						case <-ctx.Done():
							return
						case <-time.After(5 * time.Second):
						}
						goto reconnect
					}
					var km kickMsg
					if err := json.Unmarshal([]byte(msg.Payload), &km); err != nil {
						continue
					}
					if km.GatewayID == ownGatewayID {
						continue // 忽略自己发的消息
					}
					if old := connMgr.LookupSessionByPlatform(km.UID, km.Platform); old != nil {
						log.Infof("[Gateway] 跨网关顶号: uid=%s, platform=%s", km.UID, km.Platform)
						old.Cancel()
					}
				}
			}
		reconnect:
		}
	}()
}
