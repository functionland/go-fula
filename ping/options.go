package ping

import (
	"sync"
)

type (
	Option  func(*options) error
	options struct {
		allowTransientConnection bool
		timeout                  int
		wg                       *sync.WaitGroup
		count                    int
	}
)

func newOptions(o ...Option) (*options, error) {
	opts := options{
		allowTransientConnection: true, // Example: 5 minutes
		timeout:                  30,   // Example: 30 seconds
		count:                    5,    // Example: default topic name
		wg:                       nil,
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	return &opts, nil
}

func WithAllowTransientConnection(t bool) Option {
	return func(o *options) error {
		o.allowTransientConnection = t
		return nil
	}
}

func WithTimeout(to int) Option {
	return func(o *options) error {
		o.timeout = to
		return nil
	}
}

func WithWg(wg *sync.WaitGroup) Option {
	return func(o *options) error {
		o.wg = wg
		return nil
	}
}

func WithCount(n int) Option {
	return func(o *options) error {
		o.count = n
		return nil
	}
}
