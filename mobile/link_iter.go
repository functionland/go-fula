package fulamobile

import (
	"errors"

	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

type LinkIterator struct {
	links  []ipld.Link
	offset int
}

func (i *LinkIterator) HasNext() bool {
	return i.offset < len(i.links)
}

func (i *LinkIterator) Next() ([]byte, error) {
	if !i.HasNext() {
		return nil, errors.New("no more items")
	}
	next := i.links[i.offset]
	i.offset++
	if next == nil {
		return nil, nil
	}
	return next.(cidlink.Link).Bytes(), nil
}
