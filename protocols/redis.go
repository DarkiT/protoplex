package protocols

import "regexp"

// NewRedisProtocol initializes a Protocol with a Redis signature.
func NewRedisProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	regexes := []*regexp.Regexp{
		regexp.MustCompile(`^\*[0-9]+\r\n`),
	}

	return &Protocol{
		Name:                "Redis",
		Target:              targetAddress,
		EstablishConnection: establish,
		MatchStartBytes: [][]byte{
			{0x2A}, // '*' 的ASCII码为 0x2A
		},
		MatchRegexes:            regexes,
		NoComparisonBeforeBytes: 0, // 确保从第一个字节开始比较
	}
}
