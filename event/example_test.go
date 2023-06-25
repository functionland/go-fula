package event_test

import (
	"context"
	"fmt"

	"github.com/functionland/go-fula/event"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	"github.com/multiformats/go-multicodec"
)

var lp = cidlink.LinkPrototype{
	Prefix: cid.Prefix{
		Version:  1,
		Codec:    uint64(multicodec.DagCbor),
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: -1,
	},
}

// Example_eventChain creates three events, chains them together and stores them via ipld.DefaultLinkSystem.
func Example_eventChain() {
	first := &event.Event{
		Version:   event.Version0,
		Peer:      "1",
		Signature: []byte("sig1"),
	}
	second := &event.Event{
		Version:   event.Version0,
		Peer:      "2",
		Signature: []byte("sig2"),
	}
	third := &event.Event{
		Version:   event.Version0,
		Peer:      "3",
		Signature: []byte("sig3"),
	}

	ls := cidlink.DefaultLinkSystem()
	store := &memstore.Store{}
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	ctx := context.Background()

	n := bindnode.Wrap(first, event.Prototypes.Event.Type())
	if l, err := ls.Store(ipld.LinkContext{Ctx: ctx}, lp, n); err != nil {
		panic(err)
	} else {
		second.Previous = l
		fmt.Printf("Link to 1st event: %s\n", l.String())
	}

	n = bindnode.Wrap(second, event.Prototypes.Event.Type())
	if l, err := ls.Store(ipld.LinkContext{Ctx: ctx}, lp, n); err != nil {
		panic(err)
	} else {
		third.Previous = l
		fmt.Printf("Link to 2nd event: %s\n", l.String())
	}

	n = bindnode.Wrap(third, event.Prototypes.Event.Type())
	if l, err := ls.Store(ipld.LinkContext{Ctx: ctx}, lp, n); err != nil {
		panic(err)
	} else {
		fmt.Printf("Link to 3rd event: %s\n", l.String())
	}

	// output:
	// Link to 1st event: bafyreid3ehnrqi5bgyy73s42kevtcoa4tjol2rzwytnfk6222aict4hkam
	// Link to 2nd event: bafyreiaip6euzqqan5ujs322xe7ih653onk4lib2lsykeyvkd4uhcl6aea
	// Link to 3rd event: bafyreie7ycr7yqqprsn5k4zcuyvo6ap47dl2hwpea5dfo3owpdkj6ovtvm
}
