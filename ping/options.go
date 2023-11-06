package ping

type (
	Option  func(*options) error
	options struct {
		allowTransientConnection bool
		timeout                  int
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
