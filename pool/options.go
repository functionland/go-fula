package pool

import (
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
)

type (
	Option  func(*options) error
	options struct {
		h                host.Host
		name             string
		topicName        string
		announceInterval time.Duration
	}
)

func newOptions(o ...Option) (*options, error) {
	opts := options{
		announceInterval: 5 * time.Second,
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	if opts.name == "" {
		return nil, errors.New("pool name must be specified")
	}
	if opts.topicName == "" {
		opts.topicName = fmt.Sprintf("/explore.fula/pools/%s", path.Clean(opts.name))
	}
	if opts.h == nil {
		var err error
		if opts.h, err = libp2p.New(); err != nil {
			return nil, err
		}
	}
	return &opts, nil
}

// WithHost sets the libp2p host on which the pool is exposed.
// If unset a default host with random identity is used.
// See: libp2p.New.
func WithHost(h host.Host) Option {
	return func(o *options) error {
		o.h = h
		return nil
	}
}

// WithPoolName sets a human readable name for the pool.
// Required.
func WithPoolName(n string) Option {
	return func(o *options) error {
		o.name = n
		return nil
	}
}

// WithTopicName sets the name of the topic onto which announcements are made.
// Defaults to "/explore.fula/pools/<pool-name>" if unset.
// See: WithPoolName.
func WithTopicName(n string) Option {
	return func(o *options) error {
		o.topicName = n
		return nil
	}
}

// WithAnnounceInterval sets the interval at which announcements are made on the pubsub.
// Defaults to 5 seconds if unset.
func WithAnnounceInterval(i time.Duration) Option {
	return func(o *options) error {
		o.announceInterval = i
		return nil
	}
}
