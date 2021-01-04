package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/dimfeld/httptreemux"
	"github.com/gorilla/handlers"
	"github.com/unrolled/render"
)

type R struct {
	Store  storage.Store
	Node   *kernel.Node
	custom *config.Custom
}

type Call struct {
	Id     string        `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func NewRouter(custom *config.Custom, store storage.Store, node *kernel.Node) *httptreemux.TreeMux {
	router := httptreemux.New()
	impl := &R{Store: store, Node: node, custom: custom}
	router.POST("/", impl.handle)
	registerHandlers(router)
	return router
}

func registerHandlers(router *httptreemux.TreeMux) {
	router.MethodNotAllowedHandler = func(w http.ResponseWriter, r *http.Request, _ map[string]httptreemux.HandlerFunc) {
		render.New().JSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
	}
	router.NotFoundHandler = func(w http.ResponseWriter, r *http.Request) {
		render.New().JSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
	}
	router.PanicHandler = func(w http.ResponseWriter, r *http.Request, rcv interface{}) {
		render.New().JSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "server error"})
	}
}

type Render struct {
	w     http.ResponseWriter
	impl  *render.Render
	start time.Time
	id    string
}

func (r *Render) RenderData(data interface{}) {
	body := map[string]interface{}{"data": data}
	if r.id != "" {
		body["id"] = r.id
	}
	if !r.start.IsZero() {
		body["runtime"] = fmt.Sprint(time.Since(r.start).Seconds())
	}
	r.impl.JSON(r.w, http.StatusOK, body)
}

func (r *Render) RenderError(err error) {
	body := map[string]interface{}{"error": err.Error()}
	if r.id != "" {
		body["id"] = r.id
	}
	if !r.start.IsZero() {
		body["runtime"] = fmt.Sprint(time.Since(r.start).Seconds())
	}
	r.impl.JSON(r.w, http.StatusOK, body)
}

func (impl *R) handle(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var call Call
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	if err := d.Decode(&call); err != nil {
		render.New().JSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	renderer := &Render{w: w, impl: render.New(), id: call.Id}
	if impl.custom.RPC.Runtime {
		renderer.start = time.Now()
	}
	switch call.Method {
	case "getinfo":
		info, err := getInfo(impl.Store, impl.Node)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(info)
		}
	case "dumpgraphhead":
		data, err := dumpGraphHead(impl.Node, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(data)
		}
	case "sendrawtransaction":
		id, err := queueTransaction(impl.Node, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(map[string]string{"hash": id})
		}
	case "gettransaction":
		tx, err := getTransaction(impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(tx)
		}
	case "getcachetransaction":
		tx, err := getCacheTransaction(impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(tx)
		}
	case "getutxo":
		utxo, err := getUTXO(impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(utxo)
		}
	case "getsnapshot":
		snap, err := getSnapshot(impl.Node, impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(snap)
		}
	case "listsnapshots":
		snapshots, err := listSnapshots(impl.Node, impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(snapshots)
		}
	case "listmintdistributions":
		distributions, err := listMintDistributions(impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(distributions)
		}
	case "listallnodes":
		nodes, err := listAllNodes(impl.Store, impl.Node, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(nodes)
		}
	case "getroundbynumber":
		round, err := getRoundByNumber(impl.Node, impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(round)
		}
	case "getroundbyhash":
		round, err := getRoundByHash(impl.Node, impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(round)
		}
	case "getroundlink":
		link, err := getRoundLink(impl.Store, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(map[string]interface{}{"link": link})
		}
	default:
		renderer.RenderError(fmt.Errorf("invalid method %s", call.Method))
	}
}

func handleCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			handler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type,Authorization,Mixin-Conversation-ID")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS,GET,POST,DELETE")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == "OPTIONS" {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{})
		} else {
			handler.ServeHTTP(w, r)
		}
	})
}

func NewServer(custom *config.Custom, store storage.Store, node *kernel.Node, port int) *http.Server {
	router := NewRouter(custom, store, node)
	handler := handleCORS(router)
	handler = handlers.ProxyHeaders(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return server
}
