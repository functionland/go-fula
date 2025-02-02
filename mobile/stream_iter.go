package fulamobile

import (
	"github.com/functionland/go-fula/blockchain"
)

type StreamIterator struct {
	buffer *blockchain.StreamBuffer
}

func (i *StreamIterator) HasNext() bool {
	chunk, _ := i.buffer.GetChunk()
	return chunk != ""
}

func (i *StreamIterator) Next() (string, error) {
	return i.buffer.GetChunk()
}
