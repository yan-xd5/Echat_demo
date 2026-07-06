// Package route 会话路由与服务发现。
package route

// ServiceInstance 服务实例信息。
type ServiceInstance struct {
	ID       string            // 实例唯一标识
	Address  string            // ip:port
	Metadata map[string]string // 元数据（gateway_id 等）
}

// ServiceDiscovery 服务发现接口（由 tRPC naming/discovery 适配）。
type ServiceDiscovery interface {
	GetInstances(serviceName string) ([]ServiceInstance, error)
}
