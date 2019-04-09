package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

type DepositData struct {
	Chain           crypto.Hash `json:"chain"`
	AssetKey        string      `json:"asset"`
	TransactionHash string      `json:"transaction"`
	OutputIndex     uint64      `json:"index"`
	Amount          Integer     `json:"amount"`
}

func (tx *SignedTransaction) DepositData() *DepositData {
	return tx.Inputs[0].Deposit
}

func (tx *SignedTransaction) verifyDepositFormat() error {
	deposit := tx.Inputs[0].Deposit
	err := deposit.validateEthereumAssetInput()
	if err != nil {
		return err
	}
	if id := deposit.Asset().AssetId(); id != tx.Asset {
		return fmt.Errorf("invalid asset %s %s", tx.Asset, id)
	}
	return nil
}

func (tx *SignedTransaction) validateDeposit(store DataStore, msg []byte, payloadHash crypto.Hash) error {
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for deposit", len(tx.Inputs))
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for deposit", len(tx.Outputs))
	}
	if tx.Outputs[0].Type != OutputTypeScript {
		return fmt.Errorf("invalid deposit output type %d", tx.Outputs[0].Type)
	}
	if len(tx.Signatures) != 1 || len(tx.Signatures[0]) != 1 {
		return fmt.Errorf("invalid signatures count %d for deposit", len(tx.Signatures))
	}
	err := tx.verifyDepositFormat()
	if err != nil {
		return err
	}

	sig, valid := tx.Signatures[0][0], false
	for _, d := range store.ReadDomains() {
		if d.Account.PublicSpendKey.Verify(msg, sig) {
			valid = true
		}
	}
	if !valid {
		return fmt.Errorf("invalid domain signature for deposit")
	}

	return store.CheckDepositInput(tx.Inputs[0].Deposit, payloadHash)
}

func (tx *Transaction) AddDepositInput(data *DepositData) {
	tx.Inputs = append(tx.Inputs, &Input{
		Deposit: data,
	})
}
