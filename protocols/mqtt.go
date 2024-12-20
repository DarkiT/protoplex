package protocols

import "regexp"

// NewMQTTProtocol initializes a Protocol with an MQTT signature.
func NewMQTTProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                    "MQTT",
		Target:                  targetAddress,
		EstablishConnection:     establish,
		NoComparisonBeforeBytes: 4,
		MatchStartBytes: [][]byte{
			{0x4D, 0x51, 0x54, 0x54},
			//{'M', 'Q', 'T', 'T'},
			//{0x10, 0x00, 0x00, 0x04, 0x4D, 0x51, 0x54, 0x54}, // CONNECT 包头部示例，协议名称长度 + 'MQTT'
		},
		// MatchStartBytes:         nil, // 不使用字节匹配
		// MatchBytes:              nil, // 不使用字节匹配
		// NoComparisonAfterBytes:  0,
		MatchRegexes: []*regexp.Regexp{
			regexp.MustCompile(`^MQTT`),
		},
	}
}
