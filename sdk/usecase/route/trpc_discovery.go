// Package route 提供基于 tRPC naming/discovery 的服务发现实现。
package route

import (
	"fmt"

	"github.com/polarismesh/polaris-go/pkg/model"
	trpcdiscovery "trpc.group/trpc-go/trpc-go/naming/discovery"
)

// NewTRPCDiscovery 创建基于 tRPC naming/discovery 的服务发现适配器。
// discoveryName 为 discovery 插件名（如 "polarismesh"），namespace 为 Polaris 命名空间（如 "default"）。
func NewTRPCDiscovery(discoveryName, namespace string) ServiceDiscovery {
	return &tRPCDiscovery{name: discoveryName, namespace: namespace}
}

type tRPCDiscovery struct {
	name      string
	namespace string
}

// GetInstances 从 tRPC discovery 获取服务实例列表。
// 处理 tRPC Polaris 插件的特殊格式：实例信息存储在 Node.Metadata["service_instances"] 中。
func (d *tRPCDiscovery) GetInstances(serviceName string) ([]ServiceInstance, error) {
	disc := trpcdiscovery.Get(d.name)
	if disc == nil {
		return nil, fmt.Errorf("discovery %s 未注册", d.name)
	}
	nodes, err := disc.List(serviceName, trpcdiscovery.WithNamespace(d.namespace))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", serviceName, err)
	}

	var instances []ServiceInstance
	for _, n := range nodes {
		// tRPC Polaris 插件把实例信息序列化在 Metadata 中
		if raw, ok := n.Metadata["service_instances"]; ok {
			if resp, ok := raw.(*model.InstancesResponse); ok {
				for _, inst := range resp.Instances {
					if !inst.IsHealthy() {
						continue
					}
					meta := make(map[string]string)
					for k, v := range inst.GetMetadata() {
						meta[k] = v
					}
					instances = append(instances, ServiceInstance{
						ID:       inst.GetId(),
						Address:  fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort()),
						Metadata: meta,
					})
				}
			}
		} else if n.Address != "" {
			// 兼容直接返回 Address 的 discovery
			meta := make(map[string]string, len(n.Metadata))
			for k, v := range n.Metadata {
				if s, ok := v.(string); ok {
					meta[k] = s
				}
			}
			instances = append(instances, ServiceInstance{
				ID:       n.Address,
				Address:  n.Address,
				Metadata: meta,
			})
		}
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("服务 %s 无可用实例", serviceName)
	}
	return instances, nil
}
