package announcements

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/functionland/go-fula/common"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
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
	if an.topicName == "" || an.topicName == "0" {
		log.Warnw("Announcement do not have any topic to subscribe to", "on peer", an.h.ID())
		return errors.New("Announcement do not have any topic to subscribe to")
	}
	typeSystem, err := ipld.LoadSchemaBytes(schemaBytes)
	if err != nil {
		panic(fmt.Errorf("cannot load schema: %w", err))
	}
	PubSubPrototypes.Announcement = bindnode.Prototype((*Announcement)(nil), typeSystem.TypeByName("Announcement"))

	gr := pubsub.DefaultGossipSubRouter(an.h)

	/*var addrInfos []peer.AddrInfo
	for _, relay := range an.relays {
		// Parse the multiaddr
		ma, err := multiaddr.NewMultiaddr(relay)
		if err != nil {
			log.Warnln("Error parsing multiaddr:", err)
			continue
		}

		// Extract the peer ID
		addrInfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			log.Warnln("Error extracting peer ID:", err)
			continue
		}
		if addrInfo != nil {
			addrInfos = append(addrInfos, *addrInfo)
		}
	}*/

	gsub, err := pubsub.NewGossipSubWithRouter(ctx, an.h, gr,
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageSigning(true),
		pubsub.WithDefaultValidator(validator),
	)

	if err != nil {
		log.Errorw("Error happened while creating pubsub", "peer", an.h.ID())
		return err
	}

	log.Debugw("Created topic", "on peer", an.h.ID(), "topic", an.topicName)
	an.topic, err = gsub.Join(an.topicName)
	if err != nil {
		log.Errorw("Error happened while joining the topic", "peer", an.h.ID())
		return err
	}
	an.sub, err = an.topic.Subscribe()
	if err != nil {
		log.Errorw("Error happened while subscribing the topic", "peer", an.h.ID())
		return err
	}
	return nil
}

func (an *FxAnnouncements) processAnnouncement(ctx context.Context, from peer.ID, atype AnnouncementType, addrs []multiaddr.Multiaddr, topicString string) error {
	log.Infow("processing announcement", "on", an.h.ID(), "from", from)
	switch atype {
	case IExistAnnouncementType:
		log.Info("IExist request", "on", an.h.ID(), "from", from)
		an.h.Peerstore().AddAddrs(from, addrs, peerstore.ConnectedAddrTTL)
	case PoolJoinRequestAnnouncementType:
		log.Info("PoolJoin request", "on", an.h.ID(), "from", from)
		//TODO: second '' string should be account
		if err := an.PoolJoinRequestHandler.HandlePoolJoinRequest(ctx, from, "", topicString, true); err != nil {
			log.Errorw("An error occurred in handling pool join request announcement", "on", an.h.ID(), "from", from, err)
			return err
		}
	default:
		log.Info("Unknown request", "on", an.h.ID(), "from", from)
	}
	return nil
}

func (an *FxAnnouncements) HandleAnnouncements(ctx context.Context) {
	log.Debug("Starting to handle announcements")

	for {
		select {
		case <-ctx.Done():
			log.Info("Context cancelled, stopping handle announcements")
			return
		default:
			// Directly retrieve the next message
			msg, err := an.sub.Next(ctx)
			if err != nil {
				if err == pubsub.ErrSubscriptionCancelled || err == pubsub.ErrTopicClosed {
					log.Info("Subscription cancelled or topic closed, stopping handle announcements")
				} else {
					log.Errorw("Error while getting next announcement", "err", err)
				}
				return // Exit the loop in case of error
			}

			// Process the message
			if msg != nil {
				from, err := peer.IDFromBytes(msg.From)
				if err != nil {
					log.Errorw("Failed to decode announcement sender", "err", err)
					continue
				}
				if from == an.h.ID() {
					//log.Debug("Ignoring announcement from self")
					continue
				}
				a := &Announcement{}
				if err = a.UnmarshalBinary(msg.Data); err != nil {
					log.Errorw("Failed to decode announcement data", "err", err)
					continue
				}

				addrs, err := a.GetAddrs()
				if err != nil {
					log.Errorw("Failed to decode announcement addrs", "err", err)
					continue
				}

				log.Debugw("Received announcement", "from", from, "self", an.h.ID(), "announcement", a)
				log.Debug("processAnnouncement call")
				if msg.Topic != nil {
					err = an.processAnnouncement(ctx, from, a.Type, addrs, *msg.Topic)
					if err != nil {
						log.Errorw("failed to process announcement", "err", err)
						continue
					}
				} else {
					log.Debug("Topic is nil")
					continue
				}
			}
		}
	}
}

func (an *FxAnnouncements) AnnounceIExistPeriodically(ctx context.Context) {

	ticker := time.NewTicker(an.announceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Context cancelled, stopped making periodic iexist announcements")
			return
		case t := <-ticker.C:
			a := &Announcement{
				Version: common.Version0,
				Type:    IExistAnnouncementType,
			}
			a.SetAddrs(an.h.Addrs()...)
			b, err := a.MarshalBinary()
			if err != nil {
				log.Errorw("Failed to encode iexist announcement", "err", err)
				continue
			}
			if err := an.topic.Publish(ctx, b); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Info("Context cancelled or deadline exceeded, stopped making periodic iexist announcements")
					return
				}
				if errors.Is(err, pubsub.ErrTopicClosed) || errors.Is(err, pubsub.ErrSubscriptionCancelled) {
					log.Info("Topic closed or subscription cancelled, stopped making periodic iexist announcements")
					return
				}
				log.Errorw("Failed to publish iexist announcement", "err", err)
				continue
			}
			//log.Debugw("Announced iexist message", "from", an.h.ID(), "announcement", a, "time", t)
		}
	}
}

func (an *FxAnnouncements) AnnounceJoinPoolRequestPeriodically(ctx context.Context) {
	log.Debugw("Starting AnnounceJoinPoolRequestPeriodically pool join request", "peer", an.h.ID())
	log.Debugw("peerlist before AnnounceJoinPoolRequestPeriodically pool join request", "on", an.h.ID(), "peerlist", an.topic.ListPeers())

	an.announcingJoinPoolMutex.Lock()
	if an.announcingJoinPoolRequest {
		an.announcingJoinPoolMutex.Unlock()
		log.Info("pool join request announcements are already in progress.", "peer", an.h.ID())
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
	defer ticker.Stop()
	for {
		log.Debugw("inside ticker for join pool request", "peer", an.h.ID())
		select {
		case <-ctx.Done():
			log.Info("stopped making periodic pool join request announcements", "peer", an.h.ID())
			return
		case <-an.stopJoinPoolRequestChan: // Assume an.stopChan is a `chan struct{}` used to signal stopping the ticker.
			log.Info("stopped making periodic pool join request announcements due to stop signal", "peer", an.h.ID())
			return
		case t := <-ticker.C:
			a := &Announcement{
				Version: common.Version0,
				Type:    PoolJoinRequestAnnouncementType,
			}
			a.SetAddrs(an.h.Addrs()...)
			b, err := a.MarshalBinary()
			if err != nil {
				log.Errorw("failed to encode pool join request announcement", "peer", an.h.ID(), "err", err)
				continue
			}
			log.Debugw("inside ticker for join pool request now publishing", "peer", an.h.ID())
			if err := an.topic.Publish(ctx, b); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Info("stopped making periodic pool join request announcements", "peer", an.h.ID(), "err", err)
					return
				}
				if errors.Is(err, pubsub.ErrTopicClosed) || errors.Is(err, pubsub.ErrSubscriptionCancelled) {
					log.Info("stopped making periodic pool join request announcements as topic is closed or subscription cancelled", "peer", an.h.ID(), "err", err)
					return
				}
				log.Errorw("failed to publish pool join request announcement", "peer", an.h.ID(), "err", err)
				continue
			}
			log.Debugw("announced pool join request message", "from", an.h.ID(), "announcement", a, "time", t)
		}
	}
}

func (an *FxAnnouncements) ValidateAnnouncement(ctx context.Context, id peer.ID, msg *pubsub.Message, status common.MemberStatus, exists bool) bool {
	//log.Debugw("ValidateAnnouncement", "on peer", an.h.ID(), "from peerID", id)
	a := &Announcement{}
	if err := a.UnmarshalBinary(msg.Data); err != nil {
		log.Errorw("failed to unmarshal announcement data", "err", err)
		return false
	}
	//log.Debugw("ValidateAnnouncement", "on peer", an.h.ID(), "from peerID", id, "type", a.Type)

	switch a.Type {
	case NewManifestAnnouncementType:
		// Check if sender is approved
		if !exists {
			log.Debugw("peer is not recognized", "on peer", an.h.ID(), "from peer", id)
			return false
		}
		if status != common.Approved {
			log.Debugw("peer is not an approved member", "on peer", an.h.ID(), "from peer", id)
			return false
		}
	case PoolJoinRequestAnnouncementType:
		if status == common.Unknown {
			log.Debugw("peer is no longer permitted to send this message type", "on peer", an.h.ID(), "from peer", id, "status", status)
			return false
		} else {
			log.Debugw("PoolJoinRequestAnnouncementType status is not Unknown and ok")
		}
	case PoolJoinApproveAnnouncementType, IExistAnnouncementType:
		// Any member status is valid for a pool join announcement
	default:
		log.Debugw("The Type is not set ", a.Type)
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
	log.Debugw("closed topic", "peer", an.h.ID())
	if an.sub != nil {
		an.sub.Cancel()
		tErr := an.topic.Close()
		return tErr
	}
	log.Debug("Announcements are already closed")
	return nil
}

// In the announcements package, add this to your concrete type that implements the Announcements interface.
func (an *FxAnnouncements) SetPoolJoinRequestHandler(handler PoolJoinRequestHandler) {
	// Set the handler
	an.PoolJoinRequestHandler = handler
}
