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

// ModuleName names the module in the host's registration logs and errors.
func (b *BojetModule) ModuleName() string { return "bojet" }

func (b *BojetModule) Register(app *host.App) error {
	host.ProvideService[UserStore](app, NewDBStore())
	host.ProvideService(app, New(b.opts...))

	return app.Err()
}
