package protoplex

import (
	"context"
	"testing"
	"time"

	"github.com/darkit/protoplex/protocols"
	"github.com/darkit/slog"
)

func TestRunServer(t *testing.T) {
	pm := NewProtocolManager()
	defer pm.Close()

	pm.AddProtocol(protocols.NewHTTPWebDAVProtocol("192.168.1.5:5000"))
	pm.AddProtocol(protocols.NewMQTTProtocol("127.0.0.1:1883"))
	pm.AddProtocol(protocols.NewRedisProtocol("192.168.1.6:6379"))
	pm.AddProtocol(protocols.NewRDPProtocol("192.168.1.125:3389"))
	pm.AddProtocol(protocols.NewRTSPProtocol("192.168.1.174:554"))
	pm.AddProtocol(protocols.NewRTMPProtocol("192.168.1.252:1935"))

	go func() {
		tk := time.NewTicker(120 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				metrics := pm.GetMetrics()
				slog.Info("============================================")
				// 连接统计
				slog.Infof("活跃连接数: %v", metrics["active_connections"])

				// 全局流量统计
				slog.Infof("总入站流量: %v bytes", metrics["total_in_bytes"])
				slog.Infof("总出站流量: %v bytes", metrics["total_out_bytes"])
				slog.Infof("当前入站速率: %v bytes/s", metrics["current_in_bytes"])
				slog.Infof("当前出站速率: %v bytes/s", metrics["current_out_bytes"])

				// 按协议流量统计
				if protocolTraffic, ok := metrics["protocol_traffic"].(map[string]map[string]int64); ok {
					for protocol, stats := range protocolTraffic {
						slog.Info("----------------------------------------")
						slog.Infof("%s协议流量统计:", protocol)
						slog.Infof("  总入站流量: %v bytes", stats["total_in_bytes"])
						slog.Infof("  总出站流量: %v bytes", stats["total_out_bytes"])
						slog.Infof("  当前入站速率: %v bytes/s", stats["current_in_bytes"])
						slog.Infof("  当前出站速率: %v bytes/s", stats["current_out_bytes"])
					}
				}

				// 错误统计
				slog.Infof("协议识别错误: %v", metrics["identify_errors"])
				slog.Infof("代理错误: %v", metrics["proxy_errors"])

				// 协议命中统计
				if hits, ok := metrics["protocol_hits"].(map[string]int64); ok {
					for protocol, count := range hits {
						slog.Infof("协议 %s 命中次数: %d", protocol, count)
					}
				}
			}
		}
	}()
	_ = pm.RunServer(context.Background(), ":9090")
}
