package common

import (
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

type DepositData struct {
	Chain       crypto.Hash
	AssetKey    string
	Transaction string
	Index       uint64
	Amount      Integer
}

func (d *DepositData) Asset() *Asset {
	return &Asset{
		Chain:    d.Chain,
		AssetKey: d.AssetKey,
	}
}

func (d *DepositData) UniqueKey() crypto.Hash {
	index := fmt.Sprintf("%s:%s:%d", d.Chain, d.Transaction, d.Index)
	return crypto.Sha256Hash([]byte(index)).ForNetwork(d.Chain)
}

func (tx *Transaction) DepositData() *DepositData {
	if len(tx.Inputs) != 1 {
		return nil
	}
	return tx.Inputs[0].Deposit
}

func (tx *Transaction) verifyDepositData(store DataStore) error {
	deposit := tx.Inputs[0].Deposit
	asset := deposit.Asset()
	if err := asset.Verify(); err != nil {
		return fmt.Errorf("invalid asset data %s", err.Error())
	}
	if deposit.Amount.Sign() <= 0 {
		return fmt.Errorf("invalid amount %s", deposit.Amount.String())
	}
	if strings.TrimSpace(deposit.Transaction) != deposit.Transaction || len(deposit.Transaction) == 0 {
		return fmt.Errorf("invalid transaction hash %s", deposit.Transaction)
	}
	old, balance, err := store.ReadAssetWithBalance(tx.Asset)
	if err != nil || old == nil {
		return err
	}
	total := balance.Add(deposit.Amount)
	if total.Cmp(GetAssetCapacity(tx.Asset)) >= 0 {
		return fmt.Errorf("invalid deposit capacity %s", total.String())
	}
	if old.Chain == asset.Chain && old.AssetKey == asset.AssetKey {
		return nil
	}
	return fmt.Errorf("invalid asset info %s %v %v", tx.Asset, *old, *asset)
}

func (tx *SignedTransaction) validateDeposit(store DataStore, payloadHash crypto.Hash, sigs []map[uint16]*crypto.Signature, snapTime uint64) error {
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for deposit", len(tx.Inputs))
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for deposit", len(tx.Outputs))
	}
	if tx.Outputs[0].Type != OutputTypeScript {
		return fmt.Errorf("invalid deposit output type %d", tx.Outputs[0].Type)
	}
	if len(sigs) != 1 || len(sigs[0]) != 1 {
		return fmt.Errorf("invalid signatures count %d for deposit", len(sigs))
	}
	err := tx.verifyDepositData(store)
	if err != nil {
		return err
	}

	sig := sigs[0][0]
	if sig == nil {
		return fmt.Errorf("invalid custodian signature index for deposit")
	}
	custodian, err := store.ReadCustodian(snapTime)
	if err != nil {
		return err
	}
	if !custodian.Custodian.PublicSpendKey.Verify(payloadHash, *sig) {
		return fmt.Errorf("invalid custodian signature for deposit")
	}

	locked, err := store.ReadDepositLock(tx.DepositData())
	if err != nil {
		return err
	}
	if locked.HasValue() && locked != payloadHash {
		return fmt.Errorf("invalid lock %s %s", locked, payloadHash)
	}
	return nil
}

func (tx *Transaction) AddDepositInput(data *DepositData) {
	tx.Inputs = append(tx.Inputs, &Input{
		Deposit: data,
	})
}
