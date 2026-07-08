package session

import (
	"github.com/redis/go-redis/v9"
	"echat/sdk/usecase/route"
)

type Gateway struct {
	GatewayID     string
	ConnMgr       *ConnManager
	SessionRedis  *SessionRedis
	Redis         *redis.Client
	SessionRouter *route.SessionRouter
}
