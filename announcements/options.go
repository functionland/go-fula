package announcements

import (
	"sync"
	"time"
)

type (
	Option  func(*options) error
	options struct {
		announceInterval time.Duration
		timeout          int
		topicName        string
		wg               *sync.WaitGroup
	}
)

func newOptions(o ...Option) (*options, error) {
	var opts options
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	return &opts, nil
}

func WithAnnounceInterval(t int) Option {
	return func(o *options) error {
		o.announceInterval = time.Duration(int64(t)) * time.Second
		return nil
	}
}

func WithTimeout(to int) Option {
	return func(o *options) error {
		o.timeout = to
		return nil
	}
}

func WithTopicName(n string) Option {
	return func(o *options) error {
		o.topicName = n
		return nil
	}
}

func WithWg(wg *sync.WaitGroup) Option {
	return func(o *options) error {
		o.wg = wg
		return nil
	}
}
