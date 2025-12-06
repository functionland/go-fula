package blockchain

import (
	"bytes"
	"net/http"
	"sync"
)

// NewTestBlockchain creates a minimal FxBlockchain instance for testing EVM calls.
// This instance can be used to test HandleEVMPoolList and HandleIsMemberOfPool
// without requiring a full libp2p host.
func NewTestBlockchain(timeout int) *FxBlockchain {
	if timeout <= 0 {
		timeout = 30
	}
	return &FxBlockchain{
		options: &options{
			timeout: timeout,
		},
		bufPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
		reqPool: &sync.Pool{
			New: func() interface{} {
				return new(http.Request)
			},
		},
		ch: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
	}
}
