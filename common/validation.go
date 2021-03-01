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

	if ver.Version != TxVersion || tx.Version != TxVersion {
		return fmt.Errorf("invalid tx version %d %d", ver.Version, tx.Version)
	}
	if txType == TransactionTypeUnknown {
		return fmt.Errorf("invalid tx type %d", txType)
	}
	if len(tx.Inputs) < 1 || len(tx.Outputs) < 1 {
		return fmt.Errorf("invalid tx inputs or outputs %d %d", len(tx.Inputs), len(tx.Outputs))
	}
	if len(tx.Inputs) != len(tx.Signatures) && txType != TransactionTypeNodeAccept && txType != TransactionTypeNodeRemove {
		return fmt.Errorf("invalid tx signature number %d %d %d", len(tx.Inputs), len(tx.Signatures), txType)
	}
	if len(tx.Extra) > ExtraSizeLimit {
		return fmt.Errorf("invalid extra size %d", len(tx.Extra))
	}
	if len(ver.Marshal()) > config.TransactionMaximumSize {
		return fmt.Errorf("invalid transaction size %d", len(msg))
	}

	inputsFilter, inputAmount, err := validateInputs(store, tx, msg, ver.PayloadHash(), txType)
	if err != nil {
		return err
	}
	outputAmount, err := validateOutputs(store, tx)
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
		return tx.validateDeposit(store, msg, ver.PayloadHash())
	case TransactionTypeWithdrawalSubmit:
		return tx.validateWithdrawalSubmit(inputsFilter)
	case TransactionTypeWithdrawalFuel:
		return tx.validateWithdrawalFuel(store, inputsFilter)
	case TransactionTypeWithdrawalClaim:
		return tx.validateWithdrawalClaim(store, inputsFilter, msg)
	case TransactionTypeNodePledge:
		return tx.validateNodePledge(store, inputsFilter)
	case TransactionTypeNodeCancel:
		return tx.validateNodeCancel(store, msg, ver.Signatures)
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
	keySigs := make(map[crypto.Key]*crypto.Signature)

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

		err = validateUTXO(i, &utxo.UTXO, tx.Signatures[i], nil, msg, txType, keySigs)
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		inputsFilter[fk] = &utxo.UTXO
		inputAmount = inputAmount.Add(utxo.Amount)
	}

	if len(keySigs) == 0 {
		return inputsFilter, inputAmount, nil
	}

	var keys []*crypto.Key
	var sigs []*crypto.Signature
	for k, s := range keySigs {
		keys = append(keys, &k)
		sigs = append(sigs, s)
	}
	if !crypto.BatchVerify(msg, keys, sigs) {
		return inputsFilter, inputAmount, fmt.Errorf("batch verification failure")
	}
	return inputsFilter, inputAmount, nil
}

func validateOutputs(store DataStore, tx *SignedTransaction) (Integer, error) {
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

func validateUTXO(index int, utxo *UTXO, sigs map[uint16]*crypto.Signature, sigsV1 []*crypto.Signature, msg []byte, txType uint8, keySigs map[crypto.Key]*crypto.Signature) error {
	switch utxo.Type {
	case OutputTypeScript, OutputTypeNodeRemove:
		var offset, valid int
		if len(sigsV1) > 0 {
			for _, sig := range sigsV1 {
				for i, k := range utxo.Keys {
					if i < offset {
						continue
					}
					if k.Verify(msg, *sig) {
						valid = valid + 1
						offset = i + 1
					}
				}
			}
		} else {
			for i := range sigs {
				keySigs[utxo.Keys[i]] = sigs[i]
				valid = valid + 1
			}
		}
		return utxo.Script.Validate(valid)
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
