package protocols

// NewSOCKS5Protocol initializes a Protocol with a SOCKS5 signature.
func NewSOCKS5Protocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                "SOCKS5",
		Target:              targetAddress,
		EstablishConnection: establish,
		MatchStartBytes:     [][]byte{{0x05}},
	}
}
