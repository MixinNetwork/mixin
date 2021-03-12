package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

func (ver *VersionedTransaction) Validate(store DataStore) error {
	tx := &ver.SignedTransaction
	msg := ver.PayloadMarshal()
	txType := tx.TransactionType()

	if ver.Version < TxVersion {
		return ver.validateV1(store)
	}

	if ver.Version != TxVersion {
		return fmt.Errorf("invalid tx version %d %d", ver.Version, tx.Version)
	}
	if txType == TransactionTypeUnknown {
		return fmt.Errorf("invalid tx type %d", txType)
	}
	if len(tx.Inputs) < 1 || len(tx.Outputs) < 1 {
		return fmt.Errorf("invalid tx inputs or outputs %d %d", len(tx.Inputs), len(tx.Outputs))
	}
	if len(tx.Inputs) != len(tx.SignaturesMap) && txType != TransactionTypeNodeAccept && txType != TransactionTypeNodeRemove {
		return fmt.Errorf("invalid tx signature number %d %d %d", len(tx.Inputs), len(tx.SignaturesMap), txType)
	}
	if len(tx.Extra) > ExtraSizeLimit {
		return fmt.Errorf("invalid extra size %d", len(tx.Extra))
	}
	if len(msg) > config.TransactionMaximumSize {
		return fmt.Errorf("invalid transaction size %d", len(msg))
	}

	inputsFilter, inputAmount, err := validateInputs(store, tx, msg, ver.PayloadHash(), txType)
	if err != nil {
		return err
	}
	outputAmount, err := tx.validateOutputs(store)
	if err != nil {
		return err
	}

	if inputAmount.Sign() <= 0 || inputAmount.Cmp(outputAmount) != 0 {
		return fmt.Errorf("invalid input output amount %s %s", inputAmount.String(), outputAmount.String())
	}

	switch txType {
	case TransactionTypeScript:
		return validateScriptTransaction(inputsFilter)
	case TransactionTypeMint:
		return ver.validateMint(store)
	case TransactionTypeDeposit:
		return tx.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	case TransactionTypeWithdrawalSubmit:
		return tx.validateWithdrawalSubmit(inputsFilter)
	case TransactionTypeWithdrawalFuel:
		return tx.validateWithdrawalFuel(store, inputsFilter)
	case TransactionTypeWithdrawalClaim:
		return tx.validateWithdrawalClaim(store, inputsFilter, msg)
	case TransactionTypeNodePledge:
		return tx.validateNodePledge(store, inputsFilter)
	case TransactionTypeNodeCancel:
		return tx.validateNodeCancel(store, msg, ver.SignaturesMap)
	case TransactionTypeNodeAccept:
		return tx.validateNodeAccept(store)
	case TransactionTypeNodeRemove:
		return tx.validateNodeRemove(store)
	case TransactionTypeDomainAccept:
		return fmt.Errorf("invalid transaction type %d", txType)
	case TransactionTypeDomainRemove:
		return fmt.Errorf("invalid transaction type %d", txType)
	}
	return fmt.Errorf("invalid transaction type %d", txType)
}

func validateScriptTransaction(inputs map[string]*UTXO) error {
	for _, in := range inputs {
		if in.Type != OutputTypeScript && in.Type != OutputTypeNodeRemove {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}
	return nil
}

func validateInputs(store DataStore, tx *SignedTransaction, msg []byte, hash crypto.Hash, txType uint8) (map[string]*UTXO, Integer, error) {
	inputAmount := NewInteger(0)
	inputsFilter := make(map[string]*UTXO)
	keySigs := make(map[*crypto.Key]*crypto.Signature)

	for i, in := range tx.Inputs {
		if in.Mint != nil {
			return inputsFilter, in.Mint.Amount, nil
		}

		if in.Deposit != nil {
			return inputsFilter, in.Deposit.Amount, nil
		}

		fk := fmt.Sprintf("%s:%d", in.Hash.String(), in.Index)
		if inputsFilter[fk] != nil {
			return inputsFilter, inputAmount, fmt.Errorf("invalid input %s", fk)
		}

		utxo, err := store.ReadUTXO(in.Hash, in.Index)
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		if utxo == nil {
			return inputsFilter, inputAmount, fmt.Errorf("input not found %s:%d", in.Hash.String(), in.Index)
		}
		if utxo.Asset != tx.Asset {
			return inputsFilter, inputAmount, fmt.Errorf("invalid input asset %s %s", utxo.Asset.String(), tx.Asset.String())
		}
		if utxo.LockHash.HasValue() && utxo.LockHash != hash {
			return inputsFilter, inputAmount, fmt.Errorf("input locked for transaction %s", utxo.LockHash)
		}

		err = validateUTXO(i, &utxo.UTXO, tx.SignaturesMap, msg, txType, keySigs)
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		inputsFilter[fk] = &utxo.UTXO
		inputAmount = inputAmount.Add(utxo.Amount)
	}

	if len(keySigs) == 0 && (txType == TransactionTypeNodeAccept || txType == TransactionTypeNodeRemove) {
		return inputsFilter, inputAmount, nil
	}
	if len(keySigs) < len(tx.Inputs) {
		return inputsFilter, inputAmount, fmt.Errorf("batch verification not ready %d %d", len(tx.Inputs), len(keySigs))
	}

	var keys []*crypto.Key
	var sigs []*crypto.Signature
	for k, s := range keySigs {
		keys = append(keys, k)
		sigs = append(sigs, s)
	}
	if !crypto.BatchVerify(msg, keys, sigs) {
		return inputsFilter, inputAmount, fmt.Errorf("batch verification failure %d %d", len(keys), len(sigs))
	}
	return inputsFilter, inputAmount, nil
}

func (tx *Transaction) validateOutputs(store DataStore) (Integer, error) {
	outputAmount := NewInteger(0)
	outputsFilter := make(map[crypto.Key]bool)
	for _, o := range tx.Outputs {
		if o.Amount.Sign() <= 0 {
			return outputAmount, fmt.Errorf("invalid output amount %s", o.Amount.String())
		}

		if o.Withdrawal != nil {
			outputAmount = outputAmount.Add(o.Amount)
			continue
		}

		for _, k := range o.Keys {
			if outputsFilter[k] {
				return outputAmount, fmt.Errorf("invalid output key %s", k.String())
			}
			outputsFilter[k] = true
			if !k.CheckKey() {
				return outputAmount, fmt.Errorf("invalid output key format %s", k.String())
			}
			exist, err := store.CheckGhost(k)
			if err != nil {
				return outputAmount, err
			} else if exist {
				return outputAmount, fmt.Errorf("invalid output key %s", k.String())
			}
		}

		switch o.Type {
		case OutputTypeWithdrawalSubmit,
			OutputTypeWithdrawalFuel,
			OutputTypeWithdrawalClaim,
			OutputTypeNodePledge,
			OutputTypeNodeCancel,
			OutputTypeNodeAccept:
			if len(o.Keys) != 0 {
				return outputAmount, fmt.Errorf("invalid output keys count %d for kernel multisig transaction", len(o.Keys))
			}
			if len(o.Script) != 0 {
				return outputAmount, fmt.Errorf("invalid output script %s for kernel multisig transaction", o.Script)
			}
			if o.Mask.HasValue() {
				return outputAmount, fmt.Errorf("invalid output empty mask %s for kernel multisig transaction", o.Mask)
			}
		default:
			err := o.Script.VerifyFormat()
			if err != nil {
				return outputAmount, err
			}
			if !o.Mask.HasValue() {
				return outputAmount, fmt.Errorf("invalid script output empty mask %s", o.Mask)
			}
			if o.Withdrawal != nil {
				return outputAmount, fmt.Errorf("invalid script output with withdrawal %s", o.Withdrawal.Address)
			}
		}
		outputAmount = outputAmount.Add(o.Amount)
	}
	return outputAmount, nil
}

func validateUTXO(index int, utxo *UTXO, sigs []map[uint16]*crypto.Signature, msg []byte, txType uint8, keySigs map[*crypto.Key]*crypto.Signature) error {
	switch utxo.Type {
	case OutputTypeScript, OutputTypeNodeRemove:
		for i, sig := range sigs[index] {
			if int(i) >= len(utxo.Keys) {
				return fmt.Errorf("invalid signature map index %d %d", i, len(utxo.Keys))
			}
			keySigs[&utxo.Keys[i]] = sig
		}
		return utxo.Script.Validate(len(sigs[index]))
	case OutputTypeNodePledge:
		if txType == TransactionTypeNodeAccept || txType == TransactionTypeNodeCancel {
			return nil
		}
		return fmt.Errorf("pledge input used for invalid transaction type %d", txType)
	case OutputTypeNodeAccept:
		if txType == TransactionTypeNodeRemove {
			return nil
		}
		return fmt.Errorf("accept input used for invalid transaction type %d", txType)
	case OutputTypeNodeCancel:
		return fmt.Errorf("should do more validation on those %d UTXOs", utxo.Type)
	default:
		return fmt.Errorf("invalid input type %d", utxo.Type)
	}
}
