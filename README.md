# 基于Go实现的协议分流器

[![Go Reference](https://pkg.go.dev/badge/github.com/darkit/protoplex.svg)](https://pkg.go.dev/github.com/darkit/protoplex)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkit/protoplex)](https://goreportcard.com/report/github.com/darkit/protoplex)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/darkit/protoplex/blob/master/LICENSE)

一个基于Go实现的高性能协议分流器，能够自动识别并转发不同类型的网络协议到相应的目标服务器。

## 主要特性

- 自动识别多种网络协议
- 高性能的数据转发和内存管理
- 灵活的协议规则配置
- 并发连接控制
- 完整的指标收集
- 内置缓存管理
- TCP 连接优化
- 协议优先级支持
- 内存池优化

## 支持的协议

目前支持以下协议的自动识别和转发：

- HTTP/WebDAV
- MQTT
- Redis
- RDP (Remote Desktop Protocol)
- RTSP (Real Time Streaming Protocol)
- RTMP (Real Time Messaging Protocol)
- SSH
- SOCKS4/5
- TLS
- TURN/STUN
- OpenVPN

## 安装

```bash
go get github.com/darkit/protoplex
```

## 快速开始

### 基础用法

```go
package main

import (
    "context"
    "github.com/darkit/protoplex"
    "github.com/darkit/protoplex/protocols"
)

func main() {
    // 创建协议管理器（使用默认配置）
    pm := protoplex.NewProtocolManager()
    defer pm.Close() // 确保资源被正确清理

    // 添加需要支持的协议
    pm.AddProtocol(protocols.NewHTTPWebDAVProtocol("192.168.1.5:80"))
    pm.AddProtocol(protocols.NewMQTTProtocol("127.0.0.1:1883"))
    pm.AddProtocol(protocols.NewRedisProtocol("192.168.1.6:6379"))
    
    // 启动服务器
    if err := pm.RunServer(context.Background(), ":9090"); err != nil {
        panic(err)
    }
}
```

### 高级配置

```go
// 创建带自定义配置的管理器
pm := protoplex.NewProtocolManager(
    protoplex.WithMaxConnections(2000),      // 最大并发连接数
    protoplex.WithBufferSize(64 * 1024),     // 缓冲区大小
    protoplex.WithIdentifyTimeout(10 * time.Second), // 协议识别超时
    protoplex.WithCacheTTL(10 * time.Minute),       // 缓存过期时间
    protoplex.WithDialTimeout(3 * time.Second),      // 连接目标超时
)
defer pm.Close()

// 配置多个协议并设置优先级
httpProtocol := protocols.NewHTTPWebDAVProtocol("192.168.1.5:80")
httpProtocol.Priority = 1
pm.AddProtocol(httpProtocol)

// 获取运行时指标
metrics := pm.GetMetrics()
log.Printf("活跃连接数: %d", metrics["active_connections"])
log.Printf("入站流量: %d bytes", metrics["total_in_bytes"])
log.Printf("出站流量: %d bytes", metrics["total_out_bytes"])
```

### 指标监控示例

```go
// 创建协议管理器
pm := protoplex.NewProtocolManager()
defer pm.Close()

// 定期获取指标数据
go func() {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            metrics := pm.GetMetrics()
            
            // 连接统计
            log.Printf("活跃连接数: %v", metrics["active_connections"])
            
            // 全局流量统计
            log.Printf("总入站流量: %v bytes", metrics["total_in_bytes"])
            log.Printf("总出站流量: %v bytes", metrics["total_out_bytes"])
            log.Printf("当前入站速率: %v bytes/s", metrics["current_in_bytes"])
            log.Printf("当前出站速率: %v bytes/s", metrics["current_out_bytes"])
            
            // 按协议流量统计
            if protocolTraffic, ok := metrics["protocol_traffic"].(map[string]map[string]int64); ok {
                for protocol, stats := range protocolTraffic {
                    log.Printf("协议 %s:", protocol)
                    log.Printf("  - 总入站流量: %v bytes", stats["total_in_bytes"])
                    log.Printf("  - 总出站流量: %v bytes", stats["total_out_bytes"])
                    log.Printf("  - 当前入站速率: %v bytes/s", stats["current_in_bytes"])
                    log.Printf("  - 当前出站速率: %v bytes/s", stats["current_out_bytes"])
                }
            }
            
            // 错误统计
            log.Printf("协议识别错误: %v", metrics["identify_errors"])
            log.Printf("代理错误: %v", metrics["proxy_errors"])
            
            // 协议命中统计
            if hits, ok := metrics["protocol_hits"].(map[string]int64); ok {
                for protocol, count := range hits {
                    log.Printf("协议 %s 命中次数: %d", protocol, count)
                }
            }
        }
    }
}()
```

## 性能优化

Protoplex 采用了多种性能优化策略：

1. 内存池优化
    - 使用 `sync.Pool` 复用缓冲区
    - 减少内存分配和 GC 压力

2. TCP 连接优化
    - 禁用 Nagle 算法
    - 启用 TCP keepalive
    - 优化读写缓冲区大小

3. 并发控制
    - 使用 errgroup 管理并发连接
    - 连接数限制保护

4. 协议识别优化
    - 多级匹配策略（字节匹配、正则匹配）
    - 协议优先级支持
    - 缓存识别结果

## 配置参数说明

```go
type Config struct {
    MaxConnections  int           // 最大并发连接数
    BufferSize      int           // 缓冲区大小
    IdentifyTimeout time.Duration // 协议识别超时
    CacheTTL        time.Duration // 缓存过期时间
    DialTimeout     time.Duration // 连接目标超时
}

// 默认配置
var DefaultConfig = Config{
    MaxConnections:  1024,
    BufferSize:      32 * 1024,    // 32KB
    IdentifyTimeout: 15 * time.Second,
    CacheTTL:        5 * time.Minute,
    DialTimeout:     5 * time.Second,
}
```

## 监控指标

可通过 `GetMetrics()` 获取的指标包括：

### 全局指标
- `active_connections`: 当前活跃连接数
- `total_in_bytes`: 总入站流量（字节）
- `total_out_bytes`: 总出站流量（字节）
- `current_in_bytes`: 当前入站速率（字节/秒）
- `current_out_bytes`: 当前出站速率（字节/秒）
- `identify_errors`: 协议识别错误次数
- `proxy_errors`: 代理错误次数
- `protocol_hits`: 各协议命中次数统计

### 协议级别指标
通过 `protocol_traffic` 字段可获取每个协议的详细流量统计：
```json
{
    "protocol_traffic": {
        "HTTP": {
            "total_in_bytes": 1000000,
            "total_out_bytes": 2000000,
            "current_in_bytes": 1000,
            "current_out_bytes": 2000
        },
        "MQTT": {
            "total_in_bytes": 500000,
            "total_out_bytes": 600000,
            "current_in_bytes": 500,
            "current_out_bytes": 600
        }
    }
}
```

## 注意事项

1. 确保目标服务器地址可达
2. 合理配置协议优先级
3. 注意内存使用监控
4. 实现适当的日志记录
5. 正确处理错误情况
6. 优雅关闭服务

## 许可证

MIT License

