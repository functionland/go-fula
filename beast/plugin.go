package beast

import (
	"github.com/ipfs/kubo/plugin"
)

// Plugins is an exported list of plugins that will be loaded by go-ipfs.
var Plugins = []plugin.Plugin{
	&BeastPlugin{},
}
