package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/ethereum"
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
		ChainId:  d.Chain,
		AssetKey: d.AssetKey,
	}
}

func (d *DepositData) UniqueKey() crypto.Hash {
	index := fmt.Sprintf("%s:%d", d.Transaction, d.Index)
	return crypto.Sha256Hash([]byte(index)).ForNetwork(d.Chain)
}

func (tx *Transaction) DepositData() *DepositData {
	if len(tx.Inputs) != 1 {
		return nil
	}
	return tx.Inputs[0].Deposit
}

func (tx *Transaction) verifyDepositFormat() error {
	deposit := tx.Inputs[0].Deposit
	if err := deposit.Asset().Verify(); err != nil {
		return fmt.Errorf("invalid asset data %s", err.Error())
	}
	if id := deposit.Asset().AssetId(); id != tx.Asset {
		return fmt.Errorf("invalid asset %s %s", tx.Asset, id)
	}
	if deposit.Amount.Sign() <= 0 {
		return fmt.Errorf("invalid amount %s", deposit.Amount.String())
	}

	chainId := deposit.Asset().ChainId
	switch chainId {
	case ethereum.EthereumChainId:
		return ethereum.VerifyTransactionHash(deposit.Transaction)
	case bitcoin.BitcoinChainId:
		return bitcoin.VerifyTransactionHash(deposit.Transaction)
	}
	return fmt.Errorf("invalid deposit chain id %s", chainId)
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
	err := tx.verifyDepositFormat()
	if err != nil {
		return err
	}

	sig := sigs[0][0]
	if sig == nil {
		return fmt.Errorf("invalid domain signature index for deposit")
	}
	custodian, err := store.ReadCustodian(snapTime)
	if err != nil {
		return err
	}
	if !custodian.Custodian.PublicSpendKey.Verify(payloadHash, *sig) {
		return fmt.Errorf("invalid domain signature for deposit")
	}

	return store.CheckDepositInput(tx.Inputs[0].Deposit, payloadHash)
}

func (tx *Transaction) AddDepositInput(data *DepositData) {
	tx.Inputs = append(tx.Inputs, &Input{
		Deposit: data,
	})
}
