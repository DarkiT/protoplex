package protocols

import (
	"net"
	"regexp"
)

type EstablishConnection func(conn net.Conn, initialData []byte)

// Protocol 是协议签名的实现。
type Protocol struct {
	Name                    string                // 用于审计的协议名称
	Target                  string                // 代理目标
	MatchStartBytes         [][]byte              // 用于匹配此协议的字节串（前缀）
	MatchBytes              [][]byte              // 用于匹配此协议的字节串（包含）
	MatchRegexes            []*regexp.Regexp      // 用于匹配此协议的正则表达式
	NoComparisonBeforeBytes int                   // 在此字节数之前不会进行匹配，设置为0表示忽略
	NoComparisonAfterBytes  int                   // 在此字节数之后不会进行匹配，设置为0表示忽略
	EstablishConnection     []EstablishConnection // 用于建立连接的相关配置
	Priority                int                   // 协议的优先级
}
