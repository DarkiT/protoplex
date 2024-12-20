package protoplex

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/darkit/protoplex/protocols"
	"github.com/darkit/slog"
)

var cache sync.Map

// Config 配置结构
type Config struct {
	MaxConnections  int           // 最大并发连接数
	BufferSize      int           // 缓冲区大小
	IdentifyTimeout time.Duration // 协议识别超时时间
	CacheTTL        time.Duration // 缓存过期时间
	DialTimeout     time.Duration // 连接目标服务器超时时间
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	MaxConnections:  1024,
	BufferSize:      32 * 1024,
	IdentifyTimeout: 15 * time.Second,
	CacheTTL:        5 * time.Minute,
	DialTimeout:     5 * time.Second,
}

// Metrics 指标结构
type Metrics struct {
	ActiveConnections atomic.Int64
	ProtocolHits      sync.Map // map[string]int64
	IdentifyErrors    atomic.Int64
	ProxyErrors       atomic.Int64
	CurrentInBytes    atomic.Int64 // 当前入站速率 (bytes/s)
	CurrentOutBytes   atomic.Int64 // 当前出站速率 (bytes/s)
	TotalInBytes      atomic.Int64 // 总入站流量
	TotalOutBytes     atomic.Int64 // 总出站流量
	LastInBytes       atomic.Int64 // 上一秒入站流量
	LastOutBytes      atomic.Int64 // 上一秒出站流量

	// 新增按协议统计的流量指标
	ProtocolTraffic sync.Map // map[string]*ProtocolTrafficStats
}

// ProtocolTrafficStats 协议流量统计
type ProtocolTrafficStats struct {
	TotalIn    atomic.Int64 // 协议总入站流量
	TotalOut   atomic.Int64 // 协议总出站流量
	CurrentIn  atomic.Int64 // 协议当前入站速率
	CurrentOut atomic.Int64 // 协议当前出站速率
	LastIn     atomic.Int64 // 协议上一秒入站流量
	LastOut    atomic.Int64 // 协议上一秒出站流量
}

type cacheEntry struct {
	protocol  *protocols.Protocol
	timestamp time.Time
	hits      int32 // 添加命中计数
}

type readResult struct {
	n   int
	err error
}

// Option 定义配置选项的函数类型
type Option func(*Config)

// WithMaxConnections 设置最大连接数
func WithMaxConnections(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.MaxConnections = n
		}
	}
}

// WithBufferSize 设置缓冲区大小
func WithBufferSize(size int) Option {
	return func(c *Config) {
		if size > 0 {
			c.BufferSize = size
		}
	}
}

// WithIdentifyTimeout 设置协议识别超时时间
func WithIdentifyTimeout(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.IdentifyTimeout = d
		}
	}
}

// WithCacheTTL 设置缓存过期时间
func WithCacheTTL(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.CacheTTL = d
		}
	}
}

// WithDialTimeout 设置连接超时时间
func WithDialTimeout(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.DialTimeout = d
		}
	}
}

// ProtocolManager 管理协议列表
type ProtocolManager struct {
	protocols []*protocols.Protocol
	mu        sync.RWMutex
	config    Config
	metrics   Metrics
	semaphore chan struct{}      // 用于连接数限制
	ctx       context.Context    // 用于控制后台任务
	cancel    context.CancelFunc // 用于取消后台任务
}

// NewProtocolManager 创建一个新的 ProtocolManager
func NewProtocolManager(opts ...Option) *ProtocolManager {
	// 使用默认配置
	cfg := DefaultConfig

	// 应用所有配置选项
	for _, opt := range opts {
		opt(&cfg)
	}

	// 创建一个内存池
	setBinaryPool(cfg.BufferSize)

	ctx, cancel := context.WithCancel(context.Background())

	pm := &ProtocolManager{
		protocols: make([]*protocols.Protocol, 0),
		config:    cfg,
		semaphore: make(chan struct{}, cfg.MaxConnections),
		ctx:       ctx,
		cancel:    cancel,
	}

	// 启动缓存清理器
	pm.startCacheCleaner(ctx)

	// 启动指标收集器
	pm.startMetricsCollector(ctx)

	return pm
}

// Close 关闭 ProtocolManager 及其所有后台任务
func (pm *ProtocolManager) Close() {
	if pm.cancel != nil {
		pm.cancel()
	}
}

// AddProtocol 添加新协议
func (pm *ProtocolManager) AddProtocol(p *protocols.Protocol) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.protocols = append(pm.protocols, p)
}

// RemoveProtocol 移除指定名称的协议
func (pm *ProtocolManager) RemoveProtocol(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for i, p := range pm.protocols {
		if p.Name == name {
			pm.protocols = append(pm.protocols[:i], pm.protocols[i+1:]...)
			return
		}
	}
}

// UpdateProtocol 更新指定名称的协议
func (pm *ProtocolManager) UpdateProtocol(name string, newProtocol *protocols.Protocol) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for i, p := range pm.protocols {
		if p.Name == name {
			pm.protocols[i] = newProtocol
			return
		}
	}
}

// GetProtocols 获取当前的协议列表
func (pm *ProtocolManager) GetProtocols() []*protocols.Protocol {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return append([]*protocols.Protocol(nil), pm.protocols...)
}

// RunServer 运行协议分流器
func (pm *ProtocolManager) RunServer(ctx context.Context, address string) error {
	if len(pm.GetProtocols()) == 0 {
		return errors.New("no available protocols")
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	getProtocols := pm.GetProtocols()

	slog.Info("协议分流器启动成功", "address", address, "protocols", len(getProtocols))
	for _, proto := range getProtocols {
		slog.Info("分流协议", "name", proto.Name, "target", proto.Target)
	}

	return serveConnections(ctx, listener, pm)
}

// 改进连接处理
func (pm *ProtocolManager) handleConnection(ctx context.Context, conn net.Conn) {
	// 使用 TCP 特定优化
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// 禁用 Nagle 算法,减少延迟
		tcpConn.SetNoDelay(true)
		// 启用 keep-alive
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	// 获取连接槽
	select {
	case pm.semaphore <- struct{}{}:
		defer func() { <-pm.semaphore }()
	default:
		slog.Error("达到最大连接数限制，拒绝新连接")
		conn.Close()
		return
	}

	pm.metrics.ActiveConnections.Add(1)
	defer pm.metrics.ActiveConnections.Add(-1)

	defer conn.Close()

	identifyBuffer := make([]byte, 1600)
	n, err := readWithTimeout(ctx, conn, identifyBuffer, pm.config.IdentifyTimeout)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
		case errors.Is(err, context.Canceled):
			slog.Info("上下文取消")
		case err == io.EOF:
			slog.Info("客户端关闭了连接")
		default:
			slog.Error("读取错误", "error", err.Error())
		}
		return
	}

	protocol := pm.identifyProtocol(conn.RemoteAddr().String(), identifyBuffer[:n])
	if protocol == nil {
		slog.Error("无法识别协议, 关闭连接.")
		return
	}

	slog.Info("连接已建立", "protocol", protocol.Name, "remote_addr", conn.RemoteAddr().String(), "target_addr", protocol.Target)
	if protocol.EstablishConnection != nil {
		protocol.EstablishConnection[0](conn, identifyBuffer[:n])
	} else {
		pm.handleProtocolConnection(ctx, conn, protocol, identifyBuffer[:n])
	}
	slog.Info("连接已关闭", "protocol", protocol.Name, "remote_addr", conn.RemoteAddr().String())
}

// GetMetrics 获取当前指标
func (pm *ProtocolManager) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	metrics["active_connections"] = pm.metrics.ActiveConnections.Load()
	metrics["identify_errors"] = pm.metrics.IdentifyErrors.Load()
	metrics["proxy_errors"] = pm.metrics.ProxyErrors.Load()

	// 添加流量统计指标
	metrics["total_in_bytes"] = pm.metrics.TotalInBytes.Load()
	metrics["total_out_bytes"] = pm.metrics.TotalOutBytes.Load()
	metrics["current_in_bytes"] = pm.metrics.LastInBytes.Load()
	metrics["current_out_bytes"] = pm.metrics.LastOutBytes.Load()

	// 收集协议命中数
	protocolHits := make(map[string]int64)
	pm.metrics.ProtocolHits.Range(func(key, value interface{}) bool {
		protocolHits[key.(string)] = *value.(*int64)
		return true
	})
	metrics["protocol_hits"] = protocolHits

	// 添加按协议的流量统计
	protocolTraffic := make(map[string]map[string]int64)
	pm.metrics.ProtocolTraffic.Range(func(key, value interface{}) bool {
		protocolName := key.(string)
		stats := value.(*ProtocolTrafficStats)

		protocolTraffic[protocolName] = map[string]int64{
			"total_in_bytes":    stats.TotalIn.Load(),
			"total_out_bytes":   stats.TotalOut.Load(),
			"current_in_bytes":  stats.LastIn.Load(),
			"current_out_bytes": stats.LastOut.Load(),
		}
		return true
	})
	metrics["protocol_traffic"] = protocolTraffic

	return metrics
}

// handleProtocolConnection 处理协议连接
func (pm *ProtocolManager) handleProtocolConnection(ctx context.Context, conn net.Conn, protocol *protocols.Protocol, initialData []byte) {
	// 使用 pm 的配置进行连接
	targetConn, err := dialWithTimeout(ctx, protocol.Target, pm.config.DialTimeout)
	if err != nil {
		slog.Error("远程连接失败", "error", err)
		return
	}
	defer targetConn.Close()

	if _, err = targetConn.Write(initialData); err != nil {
		slog.Error("写入初始数据失败", "error", err)
		return
	}

	// 确保协议流量统计结构存在
	if _, loaded := pm.metrics.ProtocolTraffic.LoadOrStore(protocol.Name, &ProtocolTrafficStats{}); !loaded {
		// 首次遇到此协议,初始化统计结构
		pm.metrics.ProtocolTraffic.Store(protocol.Name, &ProtocolTrafficStats{})
	}

	if err = optimizedProxy(ctx, conn, targetConn, pm.config.BufferSize, &pm.metrics, protocol.Name); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("代理过程中发生错误", "error", err)
	}
}

// identifyProtocol 协议识别
func (pm *ProtocolManager) identifyProtocol(cacheKey string, data []byte) *protocols.Protocol {
	// 使用原子操作增加命中计数
	if cachedValue, ok := cache.Load(cacheKey); ok {
		if entry, ok := cachedValue.(*cacheEntry); ok {
			if time.Since(entry.timestamp) < pm.config.CacheTTL {
				atomic.AddInt32(&entry.hits, 1)
				return entry.protocol
			}
			cache.Delete(cacheKey)
		}
	}

	// 使用 determineProtocol 函数进行协议识别
	protocol := determineProtocol(data, pm.GetProtocols())
	if protocol != nil {
		// 更新缓存
		cache.Store(cacheKey, &cacheEntry{
			protocol:  protocol,
			timestamp: time.Now(),
		})
		// 更新指标
		if v, ok := pm.metrics.ProtocolHits.LoadOrStore(protocol.Name, new(int64)); ok {
			atomic.AddInt64(v.(*int64), 1)
		}
	}
	return protocol
}

// startMetricsCollector 启动指标收集器
func (pm *ProtocolManager) startMetricsCollector(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 更新全局流量统计
				currentIn := pm.metrics.CurrentInBytes.Swap(0)
				currentOut := pm.metrics.CurrentOutBytes.Swap(0)
				pm.metrics.LastInBytes.Store(currentIn)
				pm.metrics.LastOutBytes.Store(currentOut)

				// 更新每个协议的流量统计
				pm.metrics.ProtocolTraffic.Range(func(key, value interface{}) bool {
					stats := value.(*ProtocolTrafficStats)
					currentIn := stats.CurrentIn.Swap(0)
					currentOut := stats.CurrentOut.Swap(0)
					stats.LastIn.Store(currentIn)
					stats.LastOut.Store(currentOut)
					return true
				})
			}
		}
	}()
}

// 定期清理过期缓存
func (pm *ProtocolManager) startCacheCleaner(ctx context.Context) {
	ticker := time.NewTicker(pm.config.CacheTTL)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				cache.Range(func(key, value interface{}) bool {
					if entry, ok := value.(*cacheEntry); ok {
						if now.Sub(entry.timestamp) > pm.config.CacheTTL {
							cache.Delete(key)
						}
					}
					return true
				})
			}
		}
	}()
}

func serveConnections(ctx context.Context, listener net.Listener, pm *ProtocolManager) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			slog.Error("Accept error", "error", err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			pm.handleConnection(ctx, conn)
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func readWithTimeout(ctx context.Context, conn net.Conn, buffer []byte, timeout time.Duration) (int, error) {
	readCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	readCh := make(chan readResult, 1)
	go func() {
		n, err := conn.Read(buffer)
		readCh <- readResult{n: n, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-readCtx.Done():
		return 0, readCtx.Err()
	case result := <-readCh:
		return result.n, result.err
	}
}

func dialWithTimeout(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}

	scheme := "tcp"
	if addr, err := url.Parse(address); err == nil {
		scheme = addr.Scheme
		address = addr.Host
	}

	return dialer.DialContext(ctx, scheme, address)
}

// determineProtocol 根据给定的握手确定协议，考虑所有匹配方法和优先级
func determineProtocol(data []byte, protocolList []*protocols.Protocol) *protocols.Protocol {
	dataLength := len(data)
	var matchedProtocols []*protocols.Protocol

	for _, protocol := range protocolList {
		if !isValidLength(dataLength, protocol.NoComparisonBeforeBytes, protocol.NoComparisonAfterBytes) {
			continue
		}

		if matchStartBytes(data, protocol.MatchStartBytes, protocol.NoComparisonBeforeBytes) ||
			matchBytes(data, protocol.MatchBytes) ||
			matchRegex(data, protocol.MatchRegexes) {
			matchedProtocols = append(matchedProtocols, protocol)
		}
	}

	if len(matchedProtocols) == 0 {
		return nil
	}

	// 对匹配到的协议根据优先级排序，优先级越高越靠前
	sort.Slice(matchedProtocols, func(i, j int) bool {
		return matchedProtocols[i].Priority > matchedProtocols[j].Priority
	})

	return matchedProtocols[0]
}

// isValidLength 检查数据长度是否在允许范围内
func isValidLength(dataLength, minLength, maxLength int) bool {
	return (minLength == 0 || dataLength >= minLength) && (maxLength == 0 || dataLength <= maxLength)
}

// matchStartBytes 检查数据是否与起始字节匹配
func matchStartBytes(data []byte, startBytes [][]byte, offset int) bool {
	for _, byteSlice := range startBytes {
		byteSliceLength := len(byteSlice)
		if len(data) >= byteSliceLength+offset && bytes.Equal(byteSlice, data[offset:byteSliceLength+offset]) {
			return true
		}
	}
	return false
}

// matchBytes 检查数据中是否包含特定字节序列
func matchBytes(data []byte, byteSlices [][]byte) bool {
	for _, byteSlice := range byteSlices {
		if len(data) >= len(byteSlice) && bytes.Contains(data, byteSlice) {
			return true
		}
	}
	return false
}

// matchRegex 检查数据是否与正则表达式匹配
func matchRegex(data []byte, regexes []*regexp.Regexp) bool {
	for _, regex := range regexes {
		if regex.Match(data) {
			return true
		}
	}
	return false
}
