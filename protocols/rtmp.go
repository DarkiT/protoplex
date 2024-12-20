package protocols

import "regexp"

// NewRTMPProtocol initializes a Protocol with an updated RTMP signature.
func NewRTMPProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                "RTMP",
		Target:              targetAddress,
		EstablishConnection: establish,
		Priority:            5,
		MatchStartBytes: [][]byte{
			{0x03, 0x00, 0x00, 0x00}, // RTMP 握手的C0和C1的开始
		},
		MatchRegexes: []*regexp.Regexp{
			regexp.MustCompile(`^\x03[\x00-\x05]`), // 匹配RTMP版本和可能的时间戳开始
			regexp.MustCompile(`^\x03.{4}\x14`),    // 匹配RTMP握手的C0和C1的开始，包括随机数据的开始
		},
		NoComparisonBeforeBytes: 0,
		NoComparisonAfterBytes:  1537, // RTMP握手的C0+C1阶段长度为1+1536字节
	}
}
