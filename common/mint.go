package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

type MintData struct {
	Group  string  `json:"group"`
	Batch  uint64  `json:"batch"`
	Amount Integer `json:"amount"`
}

type MintDistribution struct {
	Group       string      `json:"group"`
	Batch       uint64      `json:"batch"`
	Amount      Integer     `json:"amount"`
	Transaction crypto.Hash `json:"transaction"`
}

func (tx *SignedTransaction) validateMintInput(store DataStore) error {
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for mint", len(tx.Inputs))
	}
	mint := tx.Inputs[0].Mint
	dist, err := store.ReadLastMintDistribution(mint.Group)
	if err != nil {
		return err
	}
	if mint.Batch < dist.Batch {
		return fmt.Errorf("backward mint batch %d %d", dist.Batch, mint.Batch)
	}
	if mint.Batch == dist.Batch && dist.Transaction != tx.PayloadHash() {
		return fmt.Errorf("invalid mint lock %s %s", dist.Transaction.String(), tx.PayloadHash().String())
	}
	return nil
}

func (tx *Transaction) AddMintInput(data *MintData) {
	tx.Inputs = append(tx.Inputs, &Input{
		Mint: data,
	})
}
