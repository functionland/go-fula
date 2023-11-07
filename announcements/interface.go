package announcements

import (
	"bytes"
	"context"
	_ "embed"

	"github.com/functionland/go-fula/common"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type Announcements interface {
	HandleAnnouncements(context.Context)
	AnnounceIExistPeriodically(context.Context)
	AnnounceJoinPoolRequestPeriodically(context.Context)
	ValidateAnnouncement(context.Context, peer.ID, *pubsub.Message, common.MemberStatus, bool) bool
	StopJoinPoolRequestAnnouncements()
	Shutdown(context.Context) error
}

// PoolJoinRequestHandler is the interface that will be called by the blockchain package.
type PoolJoinRequestHandler interface {
	HandlePoolJoinRequest(context.Context, peer.ID, string) error
}

var (
	PubSubPrototypes struct {
		Announcement schema.TypedPrototype
	}
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
