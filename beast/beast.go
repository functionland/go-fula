package beast

import (
	"context"

	filePL "github.com/functionland/go-fula/protocols/file"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/plugin"
	"github.com/libp2p/go-libp2p-core/network"
)

var log = logging.Logger("plugin/beast")

// TODO implement check func or remove it
func check(err error) {}

type BeastPlugin struct {
	plugin.PluginDaemonInternal
}

var _ plugin.PluginDaemonInternal = (*BeastPlugin)(nil)

// Name returns the plugin's name, satisfying the plugin.Plugin interface.
func (*BeastPlugin) Name() string {
	return "Beast"
}

// Version returns the plugin's version, satisfying the plugin.Plugin interface.
func (*BeastPlugin) Version() string {
	return "0.1.0"
}

// Init initializes plugin, satisfying the plugin.Plugin interface. Put any
// initialization logic here.
func (*BeastPlugin) Init(env *plugin.Environment) error {
	return nil
}

func (*BeastPlugin) Start(core *core.IpfsNode) error {
	api, err := coreapi.NewCoreAPI(core)
	ctx := context.Background()
	if err != nil {
		log.Panic("cant create core API ", err)
	}

	core.PeerHost.SetStreamHandler(filePL.PROTOCOL, func(s network.Stream) {
		defer func() {
			// defer s.Close()
			// s.Close()
		}()
		filePL.RequestHandler(ctx, api, s)
	})
	log.Info("File Protocol registered")
	return nil
}

func (*BeastPlugin) Close() error {
	log.Info("have good day.")
	return nil
}
