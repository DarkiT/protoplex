package protoplex

import (
	"context"
	"io"
	"net"
	"sync"

	"golang.org/x/sync/errgroup"
)

var binaryPool *pool[[]byte]

// newPool 创建一个新的泛型内存池
func newPool[T any](f func() T) *pool[T] {
	return &pool[T]{
		p: sync.Pool{
			New: func() any {
				return f()
			},
		},
	}
}

// Pool 泛型内存池
type pool[T any] struct {
	p sync.Pool
}

// Put 将一个值放入池中
func (c *pool[T]) Put(v T) {
	c.p.Put(v)
}

// Get 从池中获取一个值
func (c *pool[T]) Get() T {
	return c.p.Get().(T)
}

// setBinaryPool 创建一个内存池
func setBinaryPool(bufSize int) {
	binaryPool = newPool[[]byte](func() []byte {
		return make([]byte, bufSize)
	})
}

// optimizedProxy 优化的代理函数，支持流量统计
func optimizedProxy(ctx context.Context, conn1, conn2 net.Conn, bufSize int, metrics *Metrics, protocol string) error {
	eg, ctx := errgroup.WithContext(ctx)

	// 添加 panic 恢复，用于记录指标
	defer func() {
		if r := recover(); r != nil {
			metrics.ProxyErrors.Add(1)
		}
	}()

	// 优化 TCP 连接参数
	optimizeTCPConn(conn1, bufSize)
	optimizeTCPConn(conn2, bufSize)

	// 使用 errgroup 管理双向数据流
	eg.Go(func() (err error) {
		return proxyConn(ctx, conn1, conn2, bufSize, metrics, protocol, true) // 入站流量
	})

	eg.Go(func() (err error) {
		return proxyConn(ctx, conn2, conn1, bufSize, metrics, protocol, false) // 出站流量
	})

	return eg.Wait()
}

// optimizeTCPConn 优化 TCP 连接参数
func optimizeTCPConn(conn net.Conn, bufSize int) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// 禁用 Nagle 算法
		tcpConn.SetNoDelay(true)
		// 启用 keep-alive
		tcpConn.SetKeepAlive(true)
		// 设置读写缓冲区
		tcpConn.SetReadBuffer(bufSize)
		tcpConn.SetWriteBuffer(bufSize)
	}
}

// proxyConn 处理单个连接的数据转发
func proxyConn(ctx context.Context, dst, src net.Conn, bufSize int, metrics *Metrics, protocol string, isIn bool) error {
	// 从内存池获取缓冲区
	buf := binaryPool.Get()
	defer binaryPool.Put(buf)

	counter := &TrafficCounter{
		metrics:  metrics,
		isIn:     isIn,
		protocol: protocol,
	}

	done := make(chan error, 1)
	go func() {
		_, err := io.CopyBuffer(
			io.MultiWriter(dst, counter), // 写入目标连接并统计流量
			src,                          // 从源连接读取
			buf,                          // 使用内存池中的缓冲区
		)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TrafficCounter 流量计数器
type TrafficCounter struct {
	metrics  *Metrics
	isIn     bool
	protocol string
}

func (tc *TrafficCounter) Read(p []byte) (n int, err error) {
	n = len(p)
	if tc.isIn {
		tc.metrics.TotalInBytes.Add(int64(n))
		tc.metrics.CurrentInBytes.Add(int64(n))
	} else {
		tc.metrics.TotalOutBytes.Add(int64(n))
		tc.metrics.CurrentOutBytes.Add(int64(n))
	}
	return n, nil
}

// Write 实现 io.Writer 接口
func (tc *TrafficCounter) Write(p []byte) (n int, err error) {
	n = len(p)
	if tc.isIn {
		tc.metrics.TotalInBytes.Add(int64(n))
		tc.metrics.CurrentInBytes.Add(int64(n))

		// 更新协议流量统计
		if stats, ok := tc.metrics.ProtocolTraffic.Load(tc.protocol); ok {
			protocolStats := stats.(*ProtocolTrafficStats)
			protocolStats.TotalIn.Add(int64(n))
			protocolStats.CurrentIn.Add(int64(n))
		}
	} else {
		tc.metrics.TotalOutBytes.Add(int64(n))
		tc.metrics.CurrentOutBytes.Add(int64(n))

		// 更新协议流量统计
		if stats, ok := tc.metrics.ProtocolTraffic.Load(tc.protocol); ok {
			protocolStats := stats.(*ProtocolTrafficStats)
			protocolStats.TotalOut.Add(int64(n))
			protocolStats.CurrentOut.Add(int64(n))
		}
	}
	return n, nil
}
