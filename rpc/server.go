package rpc

import (
	"net/http"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/rpc/internal/server"
	"github.com/MixinNetwork/mixin/storage"
)

func NewServer(custom *config.Custom, store storage.Store, node *kernel.Node, port int) *http.Server {
	return server.NewServer(custom, store, node, port)
}
