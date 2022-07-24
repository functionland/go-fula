package beast

import (
	"context"

	filePL "github.com/functionland/go-fula/protocols/file"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
)

var log = logging.Logger("plugin/beast")

func check(err error) {

}

type BeastPlugin struct {
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

	core.PeerHost.SetStreamHandler(filePL.Protocol, func(s network.Stream) {
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
