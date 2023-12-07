package targz

type config struct {
	ignores []string
}

// Option is a function that sets a value in a config.
type Option func(*config)

// getOpts creates a config and applies Options to it.
func getOpts(opts []Option) config {
	cfg := config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// WithIgnore specifies file names to ignore when creating an archive. Multiple
// names to ignore can be specified in a single call and in multiple calls to
// WithIgnore.
func WithIgnore(names ...string) Option {
	return func(c *config) {
		c.ignores = append(c.ignores, names...)
	}
}
