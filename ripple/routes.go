package main

import (
	"encoding/json"
	"net/http"

	"github.com/dimfeld/httptreemux"
	"mixin.one/blockchain/external"
	"mixin.one/session"
	"mixin.one/views"
)

func RegisterRoutes(router *httptreemux.TreeMux) {
	router.GET("/height", getHeight)
	router.GET("/fee", getFee)
	router.GET("/assets/:id", getAsset)
	router.GET("/blocks/:id", getBlock)
	router.GET("/transactions/:id", getTransactionResult)
	router.POST("/transactions", postRaw)
}

func getHeight(w http.ResponseWriter, r *http.Request, params map[string]string) {
	rpc, err := NewRPC()
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	height, err := rpc.GetBlockHeight()
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	views.RenderDataResponse(w, r, map[string]interface{}{"height": height})
}

func getAsset(w http.ResponseWriter, r *http.Request, params map[string]string) {
	if params["id"] == external.RippleChainId {
		views.RenderDataResponse(w, r, map[string]interface{}{
			"chain_id":        external.RippleChainId,
			"asset_id":        external.RippleChainId,
			"chain_asset_key": external.RippleChainId,
			"symbol":          "XRP",
			"name":            "Ripple",
			"precision":       6,
		})
	} else {
		views.RenderErrorResponse(w, r, session.NotFoundError(r.Context()))
	}
}

func getFee(w http.ResponseWriter, r *http.Request, params map[string]string) {
	rpc, err := NewRPC()
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	fee, reserve, err := rpc.EstimateFee()
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	views.RenderDataResponse(w, r, map[string]interface{}{"fee_in_xrp": fee.Persist(), "reserve_in_xrp": reserve.Persist()})
}

func getBlock(w http.ResponseWriter, r *http.Request, params map[string]string) {
	rpc, err := NewRPC()
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	block, err := rpc.GetBlock(r.Context(), params["id"])
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	views.RenderDataResponse(w, r, block)
}

func getTransactionResult(w http.ResponseWriter, r *http.Request, params map[string]string) {
	rpc, err := NewRPC()
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	tx, err := rpc.GetTransactionResult(params["id"])
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	views.RenderDataResponse(w, r, map[string]interface{}{
		"confirmations": tx.Confirmations,
		"fee":           tx.Fee.Persist(),
		"receipt":       tx.Receipt,
	})
}

func postRaw(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct {
		Raw string `json:"raw"`
	}
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	rpc, err := NewRPC()
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	txId, err := rpc.SendRawTransaction(body.Raw)
	if err != nil {
		views.RenderErrorResponse(w, r, err)
		return
	}
	views.RenderDataResponse(w, r, map[string]interface{}{"transaction_hash": txId})
}
