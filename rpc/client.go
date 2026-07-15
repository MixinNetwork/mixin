package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MixinNetwork/mixin/util"
)

var rpcHTTPClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 256,
		IdleConnTimeout:     90 * time.Second,
	},
}

func CallMixinRPC(node, method string, params []any) ([]byte, error) {
	body, err := json.Marshal(map[string]any{
		"method": method,
		"params": params,
	})
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest("POST", node, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := rpcHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer util.CloseOrPanic(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CallMixinRPC(%s, %s, %s) => status %d", node, method, params, resp.StatusCode)
	}

	var result struct {
		Data  any `json:"data"`
		Error any `json:"error"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	err = dec.Decode(&result)
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("CallMixinRPC(%s, %s, %s) => %v", node, method, params, result.Error)
	}
	if result.Data == nil {
		return nil, nil
	}

	return json.Marshal(result.Data)
}
