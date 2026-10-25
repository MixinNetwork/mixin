package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func useRPCResponse(t *testing.T, response func(string) string) {
	t.Helper()
	previous := rpcHTTPClient
	rpcHTTPClient = &http.Client{Transport: handlerTransport{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call struct{ Method string }
		require.NoError(t, json.NewDecoder(r.Body).Decode(&call))
		fmt.Fprint(w, response(call.Method))
	})}}
	t.Cleanup(func() { rpcHTTPClient = previous })
}

func TestReadersReturnMalformedResponseErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		data []string
		read func() error
	}{
		{"transaction", []string{`[]`, `{}`, `{"hex":1}`, `{"hex":"zz"}`, `{"hex":"00"}`, `{"snapshot":1}`}, func() error {
			_, _, err := GetTransaction("http://rpc/", "hash")
			return err
		}},
		{"deposit", []string{`[]`, `{}`, `{"hex":false}`, `{"hex":"zz"}`, `{"hex":"00"}`, `{"snapshot":1}`}, func() error {
			_, _, err := GetDepositTransaction("http://rpc/", "chain", "hash", 0)
			return err
		}},
		{"snapshot", []string{`[]`, `{}`, `{"hex":1}`, `{"hex":"zz"}`, `{"hex":"00"}`}, func() error {
			_, err := GetSnapshot("http://rpc/", "hash")
			return err
		}},
		{"utxo", []string{`[]`, `{"amount":"invalid"}`, `{"amount":"-1"}`, `{"mask":"invalid"}`}, func() error {
			_, err := GetUTXO("http://rpc/", "hash", 0)
			return err
		}},
		{"info", []string{`[]`, `{"mint":{"pool":"invalid"}}`, `{"consensus":"invalid"}`}, func() error {
			_, err := GetInfo("http://rpc/")
			return err
		}},
		{"submission", []string{`null`, `[]`, `{}`, `{"hash":1}`, `{"hash":"invalid"}`, `{"hash":"` + (crypto.Hash{}).String() + `"}`}, func() error {
			_, err := SendRawTransaction("http://rpc/", "raw")
			return err
		}},
		{"mints", []string{`{}`, `[{"batch":"invalid"}]`}, func() error {
			_, err := ListMintDistributions("http://rpc/", 0, 1)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, data := range test.data {
				t.Run(data, func(t *testing.T) {
					useRPCResponse(t, func(string) string { return `{"data":` + data + `}` })
					require.Error(t, test.read())
				})
			}
		})
	}
}

func TestListMintDistributionsChecksReferencedTransaction(t *testing.T) {
	for _, name := range []string{"missing", "empty inputs", "not mint", "amount mismatch", "batch mismatch", "valid"} {
		t.Run(name, func(t *testing.T) {
			tx := common.NewTransactionV5(common.XINAssetId)
			switch name {
			case "not mint":
				tx.AddInput(crypto.Blake3Hash([]byte("input")), 0)
			case "empty inputs", "missing":
			default:
				tx.AddUniversalMintInput(1, common.NewInteger(1))
			}
			ver := tx.AsVersioned()
			amount, batch := "1.00000000", uint64(1)
			if name == "amount mismatch" {
				amount = "2.00000000"
			}
			if name == "batch mismatch" {
				batch = 2
			}
			useRPCResponse(t, func(method string) string {
				if method == "listmintdistributions" {
					return fmt.Sprintf(`{"data":[{"amount":%q,"batch":%d,"transaction":%q}]}`, amount, batch, ver.PayloadHash())
				}
				if name == "missing" {
					return `{"data":null}`
				}
				return fmt.Sprintf(`{"data":{"hex":%q}}`, hex.EncodeToString(ver.Marshal()))
			})
			got, err := ListMintDistributions("http://rpc/", 0, 1)
			if name == "valid" {
				require.NoError(t, err)
				require.Len(t, got, 1)
				require.Equal(t, ver.PayloadHash(), got[0].PayloadHash())
			} else {
				require.Error(t, err)
				require.Nil(t, got)
			}
		})
	}
}

func TestCallMixinRPCRejectsUnencodableParams(t *testing.T) {
	_, err := CallMixinRPC("http://rpc/", "method", []any{make(chan int)})
	require.Error(t, err)
}
