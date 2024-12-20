package protocols

import "regexp"

// NewRDPProtocol initializes a Protocol with an updated RDP signature.
func NewRDPProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                "RDP",
		Target:              targetAddress,
		EstablishConnection: establish,
		Priority:            10, // 保持RDP的高优先级
		MatchStartBytes: [][]byte{
			{0x03, 0x00},             // TPKT 版本号
			{0x03, 0x00, 0x00, 0x13}, // 常见的RDP连接请求长度
			{0x03, 0x00, 0x00, 0x2f}, // 新观察到的RDP连接请求长度
		},
		MatchBytes: [][]byte{
			{0x43, 0x6f, 0x6f, 0x6b, 0x69, 0x65, 0x3a, 0x20}, // "Cookie: " 字符串
		},
		MatchRegexes: []*regexp.Regexp{
			regexp.MustCompile(`^\x03\x00.{2}(\x2a|\x02)[\xe0\xf0]`), // 匹配TPKT头和X.224连接请求的开始
			regexp.MustCompile(`Cookie: mstshash=`),                  // 匹配RDP cookie
		},
		NoComparisonBeforeBytes: 0,
		NoComparisonAfterBytes:  47, // 基于观察到的数据包长度
	}
}
