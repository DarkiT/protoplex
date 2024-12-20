package protocols

import (
	"regexp"
)

// NewHTTPWebDAVProtocol initializes a Protocol with both HTTP and WebDAV signatures.
func NewHTTPWebDAVProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                    "HTTP/WebDAV",
		Target:                  targetAddress,
		EstablishConnection:     establish,
		NoComparisonBeforeBytes: 16, // 使用WebDAV的较大值，以确保捕获所有方法
		MatchStartBytes: [][]byte{
			// HTTP methods
			{'G', 'E', 'T'},
			{'P', 'O', 'S', 'T'},
			{'P', 'U', 'T'},
			{'D', 'E', 'L', 'E', 'T', 'E'},
			{'H', 'E', 'A', 'D'},
			{'O', 'P', 'T', 'I', 'O', 'N', 'S'},
			{'C', 'O', 'N', 'N', 'E', 'C', 'T'},
			{'T', 'R', 'A', 'C', 'E'},
			{'P', 'A', 'T', 'C', 'H'},
			// WebDAV methods
			{'P', 'R', 'O', 'P', 'F', 'I', 'N', 'D'},
			{'P', 'R', 'O', 'P', 'P', 'A', 'T', 'C', 'H'},
			{'M', 'K', 'C', 'O', 'L'},
			{'C', 'O', 'P', 'Y'},
			{'M', 'O', 'V', 'E'},
			{'L', 'O', 'C', 'K'},
			{'U', 'N', 'L', 'O', 'C', 'K'},
		},
		MatchRegexes: []*regexp.Regexp{
			// This regex matches both HTTP and WebDAV methods
			regexp.MustCompile(`^(GET|POST|PUT|DELETE|HEAD|OPTIONS|CONNECT|TRACE|PATCH|PROPFIND|PROPPATCH|MKCOL|COPY|MOVE|LOCK|UNLOCK) .+ HTTP/`),
		},
	}
}
