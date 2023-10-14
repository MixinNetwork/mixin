package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

type WithdrawalData struct {
	Address string
	Tag     string
}

func (tx *Transaction) validateWithdrawalSubmit(inputs map[string]*UTXO) error {
	for _, in := range inputs {
		if in.Type != OutputTypeScript {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}

	if len(tx.Outputs) > 2 {
		return fmt.Errorf("invalid outputs count %d for withdrawal submit transaction", len(tx.Outputs))
	}
	if len(tx.Outputs) == 2 && tx.Outputs[1].Type != OutputTypeScript {
		return fmt.Errorf("invalid change type %d for withdrawal submit transaction", tx.Outputs[1].Type)
	}

	submit := tx.Outputs[0]
	if submit.Type != OutputTypeWithdrawalSubmit {
		return fmt.Errorf("invalid output type %d for withdrawal submit transaction", submit.Type)
	}
	if submit.Withdrawal == nil {
		return fmt.Errorf("invalid withdrawal submit data")
	}

	if len(submit.Keys) != 0 {
		return fmt.Errorf("invalid withdrawal submit keys %d", len(submit.Keys))
	}
	if len(submit.Script) != 0 {
		return fmt.Errorf("invalid withdrawal submit script %s", submit.Script)
	}
	if submit.Mask.HasValue() {
		return fmt.Errorf("invalid withdrawal submit mask %s", submit.Mask)
	}

	return nil
}

func (tx *Transaction) validateWithdrawalClaim(store DataStore, inputs map[string]*UTXO, snapTime uint64) error {
	for _, in := range inputs {
		if in.Type != OutputTypeScript {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}

	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid asset %s for withdrawal claim transaction", tx.Asset)
	}
	if len(tx.Outputs) > 2 {
		return fmt.Errorf("invalid outputs count %d for withdrawal claim transaction", len(tx.Outputs))
	}
	if len(tx.Outputs) == 2 && tx.Outputs[1].Type != OutputTypeScript {
		return fmt.Errorf("invalid change type %d for withdrawal claim transaction", tx.Outputs[1].Type)
	}

	claim := tx.Outputs[0]
	if claim.Type != OutputTypeWithdrawalClaim {
		return fmt.Errorf("invalid output type %d for withdrawal claim transaction", claim.Type)
	}
	if claim.Amount.Cmp(NewIntegerFromString(config.WithdrawalClaimFee)) < 0 {
		return fmt.Errorf("invalid output amount %s for withdrawal claim transaction", claim.Amount)
	}

	var hash crypto.Hash
	if len(tx.Extra) != len(hash) {
		return fmt.Errorf("invalid extra %d for withdrawal claim transaction", len(tx.Extra))
	}
	copy(hash[:], tx.Extra)
	submit, _, err := store.ReadTransaction(hash)
	if err != nil {
		return err
	}
	if submit == nil {
		return fmt.Errorf("invalid withdrawal submit data")
	}
	withdrawal := submit.Outputs[0].Withdrawal
	if withdrawal == nil || submit.Outputs[0].Type != OutputTypeWithdrawalSubmit {
		return fmt.Errorf("invalid withdrawal submit data")
	}

	custodian, err := store.ReadCustodian(snapTime)
	if err != nil {
		return err
	}
	view := custodian.Custodian.PublicSpendKey.DeterministicHashDerive()
	for _, utxo := range inputs {
		for _, key := range utxo.Keys {
			ghost := crypto.ViewGhostOutputKey(key, &view, &utxo.Mask, uint64(utxo.Index))
			valid := *ghost == custodian.Custodian.PublicSpendKey
			if !valid {
				return fmt.Errorf("invalid domain signature for withdrawal claim %s", key.String())
			}
		}
	}
	return nil
}
