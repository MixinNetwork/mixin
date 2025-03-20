package rpc

import (
	"encoding/hex"
	"encoding/json"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func GetBlock(rpc string, number uint64) (*common.BlockWithTransactions, error) {
	raw, err := CallMixinRPC(rpc, "getblock", []any{number})
	if err != nil || raw == nil {
		return nil, err
	}
	var rb struct {
		Number    uint64           `json:"number"`
		Timestamp uint64           `json:"timestamp"`
		Sequence  uint64           `json:"sequence"`
		Previous  crypto.Hash      `json:"previous"`
		NodeId    crypto.Hash      `json:"node"`
		Signature crypto.Signature `json:"signature"`
		Snapshots []*struct {
			Hex          string `json:"hex"`
			Transactions []*struct {
				Hex string `json:"hex"`
			}
		} `json:"snapshots"`
	}
	err = json.Unmarshal(raw, &rb)
	if err != nil {
		panic(string(raw))
	}
	block := &common.Block{
		Number:    rb.Number,
		Timestamp: rb.Timestamp,
		Sequence:  rb.Sequence,
		Previous:  rb.Previous,
		NodeId:    rb.NodeId,
		Signature: rb.Signature,
	}
	snapshots := make(map[crypto.Hash]*common.Snapshot)
	transactions := make(map[crypto.Hash]*common.VersionedTransaction)
	for _, sh := range rb.Snapshots {
		b, _ := hex.DecodeString(sh.Hex)
		s, _ := common.UnmarshalVersionedSnapshot(b)
		block.Snapshots = append(block.Snapshots, s.PayloadHash())
		snapshots[s.PayloadHash()] = s.Snapshot
		for _, th := range sh.Transactions {
			b, _ := hex.DecodeString(th.Hex)
			v, _ := common.UnmarshalVersionedTransaction(b)
			transactions[v.PayloadHash()] = v
		}
	}
	return &common.BlockWithTransactions{
		Block:        *block,
		Snapshots:    snapshots,
		Transactions: transactions,
	}, nil
}
