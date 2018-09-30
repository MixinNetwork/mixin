package rpc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func signTransaction(store storage.Store, params []interface{}) (string, error) {
	if len(params) != 2 {
		return "", errors.New("invalid params count")
	}

	var raw struct {
		Inputs []struct {
			Hash  crypto.Hash `json:"hash"`
			Index int         `json:"index"`
		} `json:"inputs"`
		Outputs []struct {
			Type     uint8            `json:"type"`
			Script   common.Script    `json:"script"`
			Accounts []common.Address `json:"accounts"`
			Amount   common.Integer   `json:"amount"`
		}
		Asset crypto.Hash `json:"asset"`
		Extra string      `json:"extra"`
	}
	err := json.Unmarshal([]byte(fmt.Sprint(params[0])), &raw)
	if err != nil {
		return "", err
	}

	tx := common.NewTransaction(raw.Asset)
	for _, in := range raw.Inputs {
		tx.AddInput(in.Hash, in.Index)
	}

	for _, out := range raw.Outputs {
		if out.Type != common.OutputTypeScript {
			return "", fmt.Errorf("invalid output type %d", out.Type)
		}
		tx.AddScriptOutput(out.Accounts, out.Script, out.Amount)
	}

	extra, err := hex.DecodeString(raw.Extra)
	if err != nil {
		return "", err
	}
	tx.Extra = extra

	key, err := hex.DecodeString(fmt.Sprint(params[1]))
	if err != nil {
		return "", err
	}
	if len(key) != 64 {
		return "", fmt.Errorf("invalid key length %d", len(key))
	}
	var account common.Address
	copy(account.PrivateViewKey[:], key[:32])
	copy(account.PrivateSpendKey[:], key[32:])

	signed := &common.SignedTransaction{Transaction: *tx}
	for i, _ := range signed.Inputs {
		err := signed.SignInput(store.SnapshotsLockUTXO, i, []common.Address{account})
		if err != nil {
			return "", err
		}
	}
	err = signed.Validate(store.SnapshotsLockUTXO, store.SnapshotsCheckGhost)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signed.Marshal()), nil
}

func queueTransaction(store storage.Store, params []interface{}) (string, error) {
	if len(params) != 1 {
		return "", errors.New("invalid params count")
	}
	raw, err := hex.DecodeString(fmt.Sprint(params[0]))
	if err != nil {
		return "", err
	}
	var tx common.SignedTransaction
	err = msgpack.Unmarshal(raw, &tx)
	if err != nil {
		return "", err
	}
	return kernel.QueueTransaction(store, &tx)
}

func getSnapshot(store storage.Store, params []interface{}) (*common.SnapshotWithTopologicalOrder, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	return store.SnapshotsReadSnapshotByTransactionHash(hash)
}

func listSnapshots(store storage.Store, params []interface{}) ([]*common.SnapshotWithTopologicalOrder, error) {
	if len(params) != 2 {
		return nil, errors.New("invalid params count")
	}
	offset, err := strconv.ParseUint(fmt.Sprint(params[0]), 10, 64)
	if err != nil {
		return nil, err
	}
	count, err := strconv.ParseUint(fmt.Sprint(params[1]), 10, 64)
	if err != nil {
		return nil, err
	}

	return store.SnapshotsReadSnapshotsSinceTopology(offset, count)
}
