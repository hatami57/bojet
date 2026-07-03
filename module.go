package bojet

import (
	"github.com/hatami57/microjet/host"
)

type BojetModule struct {
	opts []Option
}

func Module(opts ...Option) *BojetModule {
	return &BojetModule{
		opts: opts,
	}
}

func (b *BojetModule) Register(app *host.App) error {
	host.ProvideService(app, New(b.opts...))
	host.ProvideService(app, NewDBStore())

	return app.Err()
}
