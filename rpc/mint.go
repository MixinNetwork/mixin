package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
)

func ListMintDistributions(rpc string, offset, count uint64) ([]*common.VersionedTransaction, error) {
	raw, err := CallMixinRPC(rpc, "listmintdistributions", []any{offset, count, false})
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
		return nil, fmt.Errorf("decode mint distributions: %w", err)
	}

	txs := make([]*common.VersionedTransaction, len(mds))
	for i, md := range mds {
		tx, _, err := GetTransaction(rpc, md.Transaction)
		if err != nil {
			return nil, err
		}
		if tx == nil || len(tx.Inputs) != 1 || tx.Inputs[0] == nil || tx.Inputs[0].Mint == nil {
			return nil, fmt.Errorf("invalid mint transaction %s", md.Transaction)
		}
		m := tx.Inputs[0].Mint
		if m.Amount.String() != md.Amount {
			return nil, fmt.Errorf("mint amount mismatch for transaction %s", md.Transaction)
		}
		if m.Batch != md.Batch {
			return nil, fmt.Errorf("mint batch mismatch for transaction %s", md.Transaction)
		}
		txs[i] = tx
	}
	return txs, nil
}
