package protocols

import "regexp"

// NewTURNProtocol initializes a Protocol for TURN identification.
func NewTURNProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	regexes := []*regexp.Regexp{
		// 匹配 STUN 消息的特征：0x00 到 0x03 开头的两字节（表示不同的 STUN 消息类型），后面跟随 0x2112A442 的 Magic Cookie
		regexp.MustCompile(`^(\x00\x01|\x00\x02|\x00\x11|\x00\x12)\x00[\x00-\xff]{2}\x21\x12\xa4\x42`),
	}

	return &Protocol{
		Name:                "STUN/TURN",
		Target:              targetAddress,
		EstablishConnection: establish,
		MatchRegexes:        regexes,
	}
}
