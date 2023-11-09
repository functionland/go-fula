package announcements

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	_ "embed"

	"github.com/functionland/go-fula/common"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

var (
	_ Announcements = (*FxAnnouncements)(nil)

	log = logging.Logger("fula/announcements")
)

type (
	FxAnnouncements struct {
		*options
		h                         host.Host
		sub                       *pubsub.Subscription
		topic                     *pubsub.Topic
		stopJoinPoolRequestChan   chan struct{} // add this line
		closeJoinPoolRequestOnce  sync.Once
		PoolJoinRequestHandler    PoolJoinRequestHandler
		announcingJoinPoolRequest bool
		announcingJoinPoolMutex   sync.Mutex
	}
)

var (
	//go:embed pubsub.ipldsch
	schemaBytes []byte
)

func NewFxAnnouncements(h host.Host, o ...Option) (*FxAnnouncements, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	an := &FxAnnouncements{
		options:                 opts,
		h:                       h,
		stopJoinPoolRequestChan: make(chan struct{}), // initialize the channel
	}
	return an, nil
}

func (an *FxAnnouncements) Start(ctx context.Context, validator pubsub.Validator) error {
	typeSystem, err := ipld.LoadSchemaBytes(schemaBytes)
	if err != nil {
		panic(fmt.Errorf("cannot load schema: %w", err))
	}
	PubSubPrototypes.Announcement = bindnode.Prototype((*Announcement)(nil), typeSystem.TypeByName("Announcement"))

	gsub, err := pubsub.NewGossipSub(ctx, an.h,
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageSigning(true),
		pubsub.WithDefaultValidator(validator),
	)
	if err != nil {
		return err
	}

	an.topic, err = gsub.Join(an.topicName)
	if err != nil {
		return err
	}
	an.sub, err = an.topic.Subscribe()
	if err != nil {
		return err
	}
	return nil
}

func (an *FxAnnouncements) processAnnouncement(ctx context.Context, from peer.ID, atype AnnouncementType) error {
	switch atype {
	case IExistAnnouncementType:
		log.Debug("IExist request")
	case PoolJoinRequestAnnouncementType:
		log.Debug("PoolJoin request")
		if err := an.PoolJoinRequestHandler.HandlePoolJoinRequest(ctx, from, strconv.Itoa(int(atype)), true); err != nil {
			log.Errorw("An error occurred in handling pool join request announcement", err)
			return err
		}
	default:
		log.Debug("Unknown request")
	}
	return nil
}

func (an *FxAnnouncements) HandleAnnouncements(ctx context.Context) {
	defer an.wg.Done()
	for {
		msg, err := an.sub.Next(ctx)
		switch {
		case ctx.Err() != nil || err == pubsub.ErrSubscriptionCancelled || err == pubsub.ErrTopicClosed:
			log.Info("stopped handling announcements")
			return
		case err != nil:
			log.Errorw("failed to get the next announcement", "err", err)
			continue
		}
		from, err := peer.IDFromBytes(msg.From)
		if err != nil {
			log.Errorw("failed to decode announcement sender", "err", err)
			continue
		}
		if from == an.h.ID() {
			continue
		}
		a := &Announcement{}
		if err = a.UnmarshalBinary(msg.Data); err != nil {
			log.Errorw("failed to decode announcement data", "err", err)
			continue
		}
		addrs, err := a.GetAddrs()
		if err != nil {
			log.Errorw("failed to decode announcement addrs", "err", err)
			continue
		}
		an.h.Peerstore().AddAddrs(from, addrs, peerstore.PermanentAddrTTL)
		log.Infow("received announcement", "from", from, "self", an.h.ID(), "announcement", a)
		err = an.processAnnouncement(ctx, from, a.Type)
		if err != nil {
			log.Errorw("failed to process announcement", "err", err)
			continue
		}
	}
}

func (an *FxAnnouncements) AnnounceIExistPeriodically(ctx context.Context) {
	defer an.wg.Done()
	ticker := time.NewTicker(an.announceInterval)
	for {
		select {
		case <-ctx.Done():
			log.Info("stopped making periodic iexist announcements")
			return
		case t := <-ticker.C:
			a := &Announcement{
				Version: common.Version0,
				Type:    IExistAnnouncementType,
			}
			a.SetAddrs(an.h.Addrs()...)
			b, err := a.MarshalBinary()
			if err != nil {
				log.Errorw("failed to encode iexist announcement", "err", err)
				continue
			}
			if err := an.topic.Publish(ctx, b); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Info("stopped making periodic iexist announcements")
					return
				}
				if errors.Is(err, pubsub.ErrTopicClosed) || errors.Is(err, pubsub.ErrSubscriptionCancelled) {
					log.Info("stopped making periodic iexist announcements as topic is closed or subscription cancelled")
					return
				}
				log.Errorw("failed to publish iexist announcement", "err", err)
				continue
			}
			log.Infow("announced iexist message", "from", an.h.ID(), "announcement", a, "time", t)
		}
	}
}

func (an *FxAnnouncements) AnnounceJoinPoolRequestPeriodically(ctx context.Context) {
	defer an.wg.Done()
	an.announcingJoinPoolMutex.Lock()
	if an.announcingJoinPoolRequest {
		an.announcingJoinPoolMutex.Unlock()
		log.Info("Join pool request announcements are already in progress.")
		return
	}
	an.announcingJoinPoolRequest = true
	an.announcingJoinPoolMutex.Unlock()
	defer func() {
		an.announcingJoinPoolMutex.Lock()
		an.announcingJoinPoolRequest = false
		an.announcingJoinPoolMutex.Unlock()
	}()
	ticker := time.NewTicker(an.announceInterval)
	for {
		select {
		case <-ctx.Done():
			log.Info("stopped making periodic announcements")
			return
		case <-an.stopJoinPoolRequestChan: // Assume an.stopChan is a `chan struct{}` used to signal stopping the ticker.
			log.Info("stopped making periodic joinpoolrequest announcements due to stop signal")
			return
		case t := <-ticker.C:
			a := &Announcement{
				Version: common.Version0,
				Type:    PoolJoinRequestAnnouncementType,
			}
			a.SetAddrs(an.h.Addrs()...)
			b, err := a.MarshalBinary()
			if err != nil {
				log.Errorw("failed to encode pool join request announcement", "err", err)
				continue
			}
			if err := an.topic.Publish(ctx, b); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Info("stopped making periodic announcements")
					return
				}
				if errors.Is(err, pubsub.ErrTopicClosed) || errors.Is(err, pubsub.ErrSubscriptionCancelled) {
					log.Info("stopped making periodic iexist announcements as topic is closed or subscription cancelled")
					return
				}
				log.Errorw("failed to publish pool join request announcement", "err", err)
				continue
			}
			log.Infow("announced pool join request message", "from", an.h.ID(), "announcement", a, "time", t)
		}
	}
}

func (an *FxAnnouncements) ValidateAnnouncement(ctx context.Context, id peer.ID, msg *pubsub.Message, status common.MemberStatus, exists bool) bool {
	a := &Announcement{}
	if err := a.UnmarshalBinary(msg.Data); err != nil {
		log.Errorw("failed to unmarshal announcement data", "err", err)
		return false
	}

	switch a.Type {
	case NewManifestAnnouncementType:
		// Check if sender is approved
		if !exists {
			log.Errorw("peer is not recognized", "peer", id)
			return false
		}
		if status != common.Approved {
			log.Errorw("peer is not an approved member", "peer", id)
			return false
		}
	case PoolJoinRequestAnnouncementType:
		if status != common.Unknown {
			log.Errorw("peer is no longer permitted to send this message type", "peer", id)
			return false
		}
	case PoolJoinApproveAnnouncementType, IExistAnnouncementType:
		// Any member status is valid for a pool join announcement
	default:
		return false
	}

	// If all checks pass, the message is valid.
	return true
}

func (an *FxAnnouncements) StopJoinPoolRequestAnnouncements() {
	an.closeJoinPoolRequestOnce.Do(func() {
		an.announcingJoinPoolMutex.Lock()
		an.announcingJoinPoolRequest = false
		an.announcingJoinPoolMutex.Unlock()
		close(an.stopJoinPoolRequestChan)
	})
}

func (an *FxAnnouncements) Shutdown(ctx context.Context) error {
	an.sub.Cancel()
	tErr := an.topic.Close()
	return tErr
}

// In the announcements package, add this to your concrete type that implements the Announcements interface.
func (an *FxAnnouncements) SetPoolJoinRequestHandler(handler PoolJoinRequestHandler) {
	// Set the handler
	an.PoolJoinRequestHandler = handler
}
