package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
)

type RPC struct {
	Store  storage.Store
	Node   *kernel.Node
	custom *config.Custom
}

type Call struct {
	Id     string        `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func handlePanic(w http.ResponseWriter, r *http.Request) {
	rcv := recover()
	if rcv == nil {
		return
	}
	rdr := &Render{w: w}
	rdr.RenderError(fmt.Errorf("bad request"))
}

type Render struct {
	w     http.ResponseWriter
	start time.Time
	id    string
}

func (r *Render) RenderData(data interface{}) {
	body := map[string]interface{}{"data": data}
	r.render(body)
}

func (r *Render) RenderError(err error) {
	body := map[string]interface{}{"error": err.Error()}
	r.render(body)
}

func (r *Render) render(body map[string]interface{}) {
	if r.id != "" {
		body["id"] = r.id
	}
	if !r.start.IsZero() {
		body["runtime"] = fmt.Sprint(time.Since(r.start).Seconds())
	}
	b, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	r.w.Header().Set("Content-Type", "application/json")
	r.w.WriteHeader(http.StatusOK)
	_, err = r.w.Write(b)
	if err != nil {
		panic(err)
	}
}

func (impl *RPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer handlePanic(w, r)

	rdr := &Render{w: w}
	if r.URL.Path != "/" || r.Method != "POST" {
		rdr.RenderError(fmt.Errorf("bad request %s %s", r.Method, r.URL.Path))
		return
	}

	var call Call
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	if err := d.Decode(&call); err != nil {
		rdr.RenderError(fmt.Errorf("bad request %s", err.Error()))
		return
	}
	renderer := &Render{w: w, id: call.Id}
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
	case "listmintworks":
		works, err := listMintWorks(impl.Node, call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(works)
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
			rdr := Render{w: w}
			rdr.render(map[string]interface{}{})
		} else {
			handler.ServeHTTP(w, r)
		}
	})
}

func NewServer(custom *config.Custom, store storage.Store, node *kernel.Node, port int) *http.Server {
	rpc := &RPC{Store: store, Node: node, custom: custom}
	handler := handleCORS(rpc)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return server
}
