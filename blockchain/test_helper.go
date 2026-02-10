package blockchain

// NewTestBlockchain creates a minimal FxBlockchain instance for testing EVM calls.
// This instance can be used to test HandleEVMPoolList and HandleIsMemberOfPool
// without requiring a full libp2p host.
func NewTestBlockchain(timeout int) (*FxBlockchain, error) {
	if timeout <= 0 {
		timeout = 30
	}
	return NewFxBlockchain(
		NewSimpleKeyStorer(""),
		WithTimeout(timeout),
	)
}
