package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/bugsnag/bugsnag-go/errors"
	"github.com/dimfeld/httptreemux"
	"github.com/gorilla/handlers"
	"github.com/unrolled/render"
)

type R struct {
	Store storage.Store
	Node  *kernel.Node
}

type Call struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func NewRouter(store storage.Store, node *kernel.Node) *httptreemux.TreeMux {
	router := httptreemux.New()
	impl := &R{Store: store, Node: node}
	router.POST("/", impl.handle)
	registerHanders(router)
	return router
}

func registerHanders(router *httptreemux.TreeMux) {
	router.MethodNotAllowedHandler = func(w http.ResponseWriter, r *http.Request, _ map[string]httptreemux.HandlerFunc) {
		render.New().JSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
	}
	router.NotFoundHandler = func(w http.ResponseWriter, r *http.Request) {
		render.New().JSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
	}
	router.PanicHandler = func(w http.ResponseWriter, r *http.Request, rcv interface{}) {
		err := fmt.Errorf(string(errors.New(rcv, 2).Stack()))
		render.New().JSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}
}

func (impl *R) handle(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var call Call
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	if err := d.Decode(&call); err != nil {
		render.New().JSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	switch call.Method {
	case "getinfo":
		info, err := getInfo(impl.Store, impl.Node)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": info})
		}
	case "sendrawtransaction":
		id, err := queueTransaction(impl.Node, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": map[string]string{"hash": id}})
		}
	case "gettransaction":
		tx, err := getTransaction(impl.Store, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": tx})
		}
	case "getsnapshot":
		snap, err := getSnapshot(impl.Store, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": snap})
		}
	case "listsnapshots":
		snapshots, err := listSnapshots(impl.Store, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": snapshots})
		}
	case "listmintdistributions":
		distributions, err := listMintDistributions(impl.Store, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": distributions})
		}
	case "getroundbynumber":
		round, err := getRoundByNumber(impl.Store, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": round})
		}
	case "getroundbyhash":
		round, err := getRoundByHash(impl.Store, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": round})
		}
	case "getroundlink":
		link, err := getRoundLink(impl.Store, call.Params)
		if err != nil {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": err.Error()})
		} else {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"link": link}})
		}
	default:
		render.New().JSON(w, http.StatusOK, map[string]interface{}{"error": "invalid method"})
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

func StartHTTP(store storage.Store, node *kernel.Node, port int) error {
	router := NewRouter(store, node)
	handler := handleCORS(router)
	handler = handlers.ProxyHeaders(handler)

	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: handler}
	return server.ListenAndServe()
}
