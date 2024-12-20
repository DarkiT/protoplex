package protocols

// NewSSHProtocol initializes a Protocol with a SSH signature.
func NewSSHProtocol(targetAddress string, establish ...EstablishConnection) *Protocol {
	return &Protocol{
		Name:                "SSH",
		Target:              targetAddress,
		EstablishConnection: establish,
		MatchStartBytes:     [][]byte{{'S', 'S', 'H', '-'}},
	}
}
