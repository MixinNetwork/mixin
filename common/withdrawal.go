package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/ethereum"
)

type WithdrawalData struct {
	Chain    crypto.Hash `json:"chain"`
	AssetKey string      `json:"asset"`
	Address  string      `json:"address"`
	Tag      string      `json:"tag"`
}

func (w *WithdrawalData) Asset() *Asset {
	return &Asset{
		ChainId:  w.Chain,
		AssetKey: w.AssetKey,
	}
}

func (tx *SignedTransaction) validateWithdrawalSubmit(inputs map[string]*UTXO) error {
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

	if err := submit.Withdrawal.Asset().Verify(); err != nil {
		return fmt.Errorf("invalid asset data %s", err.Error())
	}
	if id := submit.Withdrawal.Asset().AssetId(); id != tx.Asset {
		return fmt.Errorf("invalid asset %s %s", tx.Asset, id)
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

	switch submit.Withdrawal.Asset().ChainId {
	case ethereum.EthereumChainId:
		return ethereum.VerifyAddress(submit.Withdrawal.Address)
	case bitcoin.BitcoinChainId:
		return bitcoin.VerifyAddress(submit.Withdrawal.Address)
	}
	return nil
}

func (tx *SignedTransaction) validateWithdrawalFuel(store DataStore, inputs map[string]*UTXO) error {
	for _, in := range inputs {
		if in.Type != OutputTypeScript {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}

	if len(tx.Outputs) > 2 {
		return fmt.Errorf("invalid outputs count %d for withdrawal fuel transaction", len(tx.Outputs))
	}
	if len(tx.Outputs) == 2 && tx.Outputs[1].Type != OutputTypeScript {
		return fmt.Errorf("invalid change type %d for withdrawal fuel transaction", tx.Outputs[1].Type)
	}

	fuel := tx.Outputs[0]
	if fuel.Type != OutputTypeWithdrawalFuel {
		return fmt.Errorf("invalid output type %d for withdrawal fuel transaction", fuel.Type)
	}

	var hash crypto.Hash
	if len(tx.Extra) != len(hash) {
		return fmt.Errorf("invalid extra %d for withdrawal fuel transaction", len(tx.Extra))
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
	if id := withdrawal.Asset().FeeAssetId(); id != tx.Asset {
		return fmt.Errorf("invalid fee asset %s %s", tx.Asset, id)
	}
	return nil
}

func (tx *SignedTransaction) validateWithdrawalClaim(store DataStore, inputs map[string]*UTXO, msg []byte) error {
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

	var domainValid bool
	for _, d := range store.ReadDomains() {
		domainValid = true
		view := d.Account.PublicSpendKey.DeterministicHashDerive()
		for _, utxo := range inputs {
			mask := utxo.Mask.AsPublicKeyOrPanic()
			for _, key := range utxo.Keys {
				ghost := crypto.ViewGhostOutputKey(mask, key.AsPublicKeyOrPanic(), view, uint64(utxo.Index))
				valid := ghost == d.Account.PublicSpendKey
				domainValid = domainValid && valid
			}
		}
		if domainValid {
			break
		}
	}
	if !domainValid {
		return fmt.Errorf("invalid domain signature for withdrawal claim")
	}
	return nil
}
