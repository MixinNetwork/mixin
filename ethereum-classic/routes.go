package main

import (
	"encoding/json"
	"net/http"

	"github.com/dimfeld/httptreemux"
	"mixin.one/blockchain/external"
	"mixin.one/number"
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
	if params["id"] == external.EthereumClassicChainId {
		views.RenderDataResponse(w, r, map[string]interface{}{
			"chain_id":        external.EthereumClassicChainId,
			"asset_id":        external.EthereumClassicChainId,
			"chain_asset_key": "0x0000000000000000000000000000000000000000",
			"symbol":          "ETC",
			"name":            "Ether Classic",
			"precision":       18,
		})
		return
	}
	token, err := persistReadToken(r.Context(), params["id"])
	if err != nil {
		views.RenderErrorResponse(w, r, err)
	} else if token == nil {
		views.RenderErrorResponse(w, r, session.NotFoundError(r.Context()))
	} else {
		views.RenderDataResponse(w, r, map[string]interface{}{
			"chain_id":        token.Chain,
			"asset_id":        token.Id,
			"chain_asset_key": token.Address,
			"symbol":          token.Symbol,
			"name":            token.Name,
			"precision":       token.Decimals,
		})
	}
}

func getFee(w http.ResponseWriter, r *http.Request, params map[string]string) {
	gasPrice := number.FromString("0.00000005")
	gasLimit := number.FromString("200000")
	views.RenderDataResponse(w, r, map[string]interface{}{
		"gas_price": gasPrice.Persist(),
		"gas_limit": gasLimit.Persist(),
	})
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
