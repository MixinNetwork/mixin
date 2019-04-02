package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	MintGroupKernelNode = "KERNELNODE"
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

func (m *MintData) Distribute(tx crypto.Hash) *MintDistribution {
	return &MintDistribution{
		Group:       m.Group,
		Batch:       m.Batch,
		Amount:      m.Amount,
		Transaction: tx,
	}
}

func (tx *VersionedTransaction) validateMintInput(store DataStore) error {
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for mint", len(tx.Inputs))
	}
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid mint asset %s", tx.Asset.String())
	}

	mint := tx.Inputs[0].Mint
	if mint.Group != MintGroupKernelNode {
		return fmt.Errorf("invalid mint group %s", mint.Group)
	}

	dist, err := store.ReadLastMintDistribution(mint.Group)
	if err != nil {
		return err
	}
	if mint.Batch > dist.Batch {
		return nil
	}
	if mint.Batch < dist.Batch {
		return fmt.Errorf("backward mint batch %d %d", dist.Batch, mint.Batch)
	}
	if dist.Transaction != tx.PayloadHash() || dist.Amount.Cmp(mint.Amount) != 0 {
		return fmt.Errorf("invalid mint lock %s %s", dist.Transaction.String(), tx.PayloadHash().String())
	}
	return nil
}

func (tx *Transaction) AddKernelNodeMintInput(batch uint64, amount Integer) {
	tx.Inputs = append(tx.Inputs, &Input{
		Mint: &MintData{
			Group:  MintGroupKernelNode,
			Batch:  batch,
			Amount: amount,
		},
	})
}
