package fulamobile

import (
	"io"
	"time"

	"github.com/functionland/go-fula/blockchain"
)

type StreamIterator struct {
	buffer    *blockchain.StreamBuffer
	timeout   time.Duration
	completed bool
}

func NewStreamIterator(buffer *blockchain.StreamBuffer) *StreamIterator {
	return &StreamIterator{
		buffer:  buffer,
		timeout: 10 * time.Second, // Default timeout
	}
}

func (i *StreamIterator) HasNext() bool {
	if i.completed {
		return false
	}

	select {
	case chunk := <-i.buffer.Chunks:
		// Return chunk to channel if we're just peeking
		go func() { i.buffer.Chunks <- chunk }()
		return true
	default:
		return !i.buffer.IsClosed()
	}
}

func (i *StreamIterator) Next() (string, error) {
	select {
	case chunk, ok := <-i.buffer.Chunks:
		if !ok {
			i.completed = true
			return "", io.EOF
		}
		return chunk, nil
	case <-time.After(i.timeout):
		return "", nil
	}
}

func (i *StreamIterator) IsComplete() bool {
	return i.completed || (i.buffer.IsClosed() && len(i.buffer.Chunks) == 0)
}

// Close implements automatic cleanup in Go
func (i *StreamIterator) Close() {
	i.buffer.Close(nil)
}
