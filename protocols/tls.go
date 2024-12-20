package protocols

// NewTLSProtocol initializes a Protocol with a TLS signature.
func NewTLSProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                "TLS",
		Target:              targetAddress,
		EstablishConnection: establish,
		MatchStartBytes:     [][]byte{{0x16, 0x03, 0x01}},
	}
}
