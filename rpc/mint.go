package rpc

import (
	"encoding/json"

	"github.com/MixinNetwork/mixin/common"
)

func ListMintDistributions(rpc string, offset, count uint64) ([]*common.VersionedTransaction, error) {
	raw, err := callMixinRPC(rpc, "listmintdistributions", []any{offset, count, false})
	if err != nil || raw == nil {
		return nil, err
	}

	var mds []struct {
		Amount      string `json:"amount"`
		Batch       uint64 `json:"batch"`
		Transaction string `json:"transaction"`
	}
	err = json.Unmarshal(raw, &mds)
	if err != nil {
		panic(string(raw))
	}

	txs := make([]*common.VersionedTransaction, len(mds))
	for i, md := range mds {
		tx, _, err := GetTransaction(rpc, md.Transaction)
		if err != nil {
			return nil, err
		}
		m := tx.Inputs[0].Mint
		if m.Amount.String() != md.Amount {
			panic(md.Transaction)
		}
		if m.Batch != md.Batch {
			panic(md.Transaction)
		}
		txs[i] = tx
	}
	return txs, nil
}
