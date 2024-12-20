package protocols

// NewSOCKS4Protocol initializes a Protocol with a SOCKS4 signature.
func NewSOCKS4Protocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                "SOCKS4",
		Target:              targetAddress,
		EstablishConnection: establish,
		MatchStartBytes:     [][]byte{{0x04}},
	}
}
