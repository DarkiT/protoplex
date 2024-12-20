package protocols

import "regexp"

// NewRTSPProtocol initializes a Protocol with a RTSP signature.
func NewRTSPProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	regexes := []*regexp.Regexp{
		regexp.MustCompile(`^(OPTIONS|DESCRIBE|SETUP|PLAY|PAUSE|TEARDOWN) rtsp://`),
	}
	return &Protocol{
		Name:                    "RTSP",
		Target:                  targetAddress,
		EstablishConnection:     establish,
		NoComparisonBeforeBytes: 16, // 检查前16个字节
		MatchRegexes:            regexes,
		MatchStartBytes: [][]byte{
			{'O', 'P', 'T', 'I', 'O', 'N', 'S'},
			{'D', 'E', 'S', 'C', 'R', 'I', 'B', 'E'},
			{'A', 'N', 'N', 'O', 'U', 'N', 'C', 'E'},
			{'S', 'E', 'T', 'U', 'P'},
			{'P', 'L', 'A', 'Y'},
			{'P', 'A', 'U', 'S', 'E'},
			{'T', 'E', 'A', 'R', 'D', 'O', 'W', 'N'},
		},
	}
}
