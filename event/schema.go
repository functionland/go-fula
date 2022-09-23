package event

import (
	_ "embed"
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
)

const Version0 = "0"

var (
	Prototypes struct {
		Event schema.TypedPrototype
	}

	//go:embed schema.ipldsch
	schemaBytes []byte
)

func init() {
	typeSystem, err := ipld.LoadSchemaBytes(schemaBytes)
	//TODO: add retry logic instead of panic
	if err != nil {
		panic(fmt.Errorf("cannot load schema: %w", err))
	}
	Prototypes.Event = bindnode.Prototype((*Event)(nil), typeSystem.TypeByName("Event"))
}
