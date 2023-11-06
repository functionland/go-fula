package blox

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/multiformats/go-multiaddr"
)

const Version0 = "0"

var (
	PubSubPrototypes struct {
		Announcement schema.TypedPrototype
	}

	//go:embed pubsub.ipldsch
	schemaBytes []byte
)

type (
	AnnouncementType int
	Announcement     struct {
		Version string
		Type    AnnouncementType
		Addrs   []string
	}
)

const (
	UnknownAnnouncementType AnnouncementType = iota
	IExistAnnouncementType
	PoolJoinRequestAnnouncementType
	PoolJoinApproveAnnouncementType
	NewManifestAnnouncementType
)

func init() {
	typeSystem, err := ipld.LoadSchemaBytes(schemaBytes)
	if err != nil {
		panic(fmt.Errorf("cannot load schema: %w", err))
	}
	PubSubPrototypes.Announcement = bindnode.Prototype((*Announcement)(nil), typeSystem.TypeByName("Announcement"))
}

func (a *Announcement) MarshalBinary() ([]byte, error) {
	n := bindnode.Wrap(a, PubSubPrototypes.Announcement.Type())
	var buf bytes.Buffer
	if err := dagcbor.Encode(n, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *Announcement) SetAddrs(ma ...multiaddr.Multiaddr) {
	a.Addrs = make([]string, len(ma))
	for i, m := range ma {
		a.Addrs[i] = m.String()
	}
}
func (a *Announcement) GetAddrs() ([]multiaddr.Multiaddr, error) {
	ma := make([]multiaddr.Multiaddr, len(a.Addrs))
	var err error
	for i, addr := range a.Addrs {
		ma[i], err = multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return ma, nil
}

func (a *Announcement) UnmarshalBinary(b []byte) error {
	buf := bytes.NewBuffer(b)
	builder := PubSubPrototypes.Announcement.NewBuilder()
	if err := dagcbor.Decode(builder, buf); err != nil {
		return err
	}
	da := bindnode.Unwrap(builder.Build()).(*Announcement)
	a.Version = da.Version
	a.Type = da.Type
	a.Addrs = da.Addrs
	return nil
}
