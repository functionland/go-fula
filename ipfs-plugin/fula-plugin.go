package ipfsplugin

import (
	"context"

	"github.com/functionland/go-fula/drive"
	fxfscore "github.com/functionland/go-fula/fxfs/core/api"
	filePL "github.com/functionland/go-fula/protocols/file"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/plugin"
	"github.com/libp2p/go-libp2p-core/network"
)

var log = logging.Logger("plugin/fulaPlugin")

func check(err error) {

}

type FulaPlugin struct {
	plugin.PluginDaemonInternal
}

var _ plugin.PluginDaemonInternal = (*FulaPlugin)(nil)

// Name returns the plugin's name, satisfying the plugin.Plugin interface.
func (*FulaPlugin) Name() string {
	return "FulaPlugin"
}

// Version returns the plugin's version, satisfying the plugin.Plugin interface.
func (*FulaPlugin) Version() string {
	return "0.1.0"
}

// Init initializes plugin, satisfying the plugin.Plugin interface. Put any
// initialization logic here.
func (*FulaPlugin) Init(env *plugin.Environment) error {
	return nil
}

func (*FulaPlugin) Start(core *core.IpfsNode) error {
	api, err := coreapi.NewCoreAPI(core)
	if err != nil {
		log.Panic("cant create core API ", err)
	}

	fapi, err := fxfscore.NewCoreAPI(core, api)
	if err != nil {
		log.Panic("Couldn't create FxFS core api")
	}
	ds := drive.NewDriveStore()
	ctx := context.Background()

	core.PeerHost.SetStreamHandler(filePL.ProtocolId, func(s network.Stream) {
		defer func() {
			s.Close()
		}()
		err := filePL.Handle(ctx, fapi, ds, s)
		if err != nil {
			log.Panic("Error in handler: ", err)
		}
	})
	log.Info("File Protocol registered")
	return nil
}

func (*FulaPlugin) Close() error {
	log.Info("file protocol handler plugin closed")
	return nil
}
