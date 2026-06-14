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
	app.ProvideService(host.ProvideType(New(b.opts...)))
	host.ProvideType(NewDBStore())

	return app.Err()
}
