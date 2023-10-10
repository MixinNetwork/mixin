package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

func (ver *VersionedTransaction) Validate(store DataStore, snapTime uint64, fork bool) error {
	tx := &ver.SignedTransaction
	txType := tx.TransactionType()

	switch ver.Version {
	case TxVersionHashSignature:
	default:
		return fmt.Errorf("invalid tx version %d", ver.Version)
	}

	if txType == TransactionTypeUnknown {
		return fmt.Errorf("invalid tx type %d", txType)
	}
	if len(tx.Inputs) < 1 || len(tx.Outputs) < 1 {
		return fmt.Errorf("invalid tx inputs or outputs %d %d",
			len(tx.Inputs), len(tx.Outputs))
	}
	if len(tx.Inputs) > SliceCountLimit || len(tx.Outputs) > SliceCountLimit ||
		len(tx.References) > SliceCountLimit {
		return fmt.Errorf("invalid tx inputs or outputs %d %d %d",
			len(tx.Inputs), len(tx.Outputs), len(tx.References))
	}
	if len(tx.Extra) > tx.getExtraLimit() {
		return fmt.Errorf("invalid extra size %d", len(tx.Extra))
	}
	if len(ver.PayloadMarshal()) > config.TransactionMaximumSize {
		return fmt.Errorf("invalid transaction size %d", len(ver.PayloadMarshal()))
	}

	if tx.AggregatedSignature != nil {
		if tx.SignaturesMap != nil {
			return fmt.Errorf("invalid signatures map %d", len(tx.SignaturesMap))
		}
	} else {
		if len(tx.Inputs) != len(tx.SignaturesMap) && txType != TransactionTypeNodeAccept &&
			txType != TransactionTypeNodeRemove {
			return fmt.Errorf("invalid tx signature number %d %d %d",
				len(tx.Inputs), len(tx.SignaturesMap), txType)
		}
	}

	err := validateReferences(store, tx)
	if err != nil {
		return err
	}
	inputsFilter, inputAmount, err := tx.validateInputs(store, ver.PayloadHash(), txType, fork)
	if err != nil {
		return err
	}
	outputAmount, err := tx.validateOutputs(store, ver.PayloadHash(), fork)
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
		return tx.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, snapTime)
	case TransactionTypeWithdrawalSubmit:
		return tx.validateWithdrawalSubmit(inputsFilter)
	case TransactionTypeWithdrawalFuel:
		return tx.validateWithdrawalFuel(store, inputsFilter)
	case TransactionTypeWithdrawalClaim:
		return tx.validateWithdrawalClaim(store, inputsFilter, snapTime)
	case TransactionTypeNodePledge:
		return tx.validateNodePledge(store, inputsFilter, snapTime)
	case TransactionTypeNodeCancel:
		return tx.validateNodeCancel(store, ver.PayloadHash(), ver.SignaturesMap, snapTime)
	case TransactionTypeNodeAccept:
		return tx.validateNodeAccept(store, snapTime)
	case TransactionTypeNodeRemove:
		return tx.validateNodeRemove(store)
	case TransactionTypeCustodianUpdateNodes:
		return tx.validateCustodianUpdateNodes(store, snapTime)
	case TransactionTypeCustodianSlashNodes:
		return tx.validateCustodianSlashNodes(store)
	}
	return fmt.Errorf("invalid transaction type %d", txType)
}

func (tx *SignedTransaction) getExtraLimit() int {
	if tx.Version < TxVersionHashSignature {
		panic(tx.Version)
	}
	if tx.Asset != XINAssetId {
		return ExtraSizeGeneralLimit
	}
	if len(tx.Outputs) < 1 {
		return ExtraSizeGeneralLimit
	}
	out := tx.Outputs[0]
	if len(out.Keys) != 1 {
		return ExtraSizeGeneralLimit
	}
	if out.Script.String() != "fffe40" {
		return ExtraSizeGeneralLimit
	}
	switch out.Type {
	case OutputTypeScript:
	case OutputTypeCustodianUpdateNodes:
		return ExtraSizeStorageCapacity
	default:
		return ExtraSizeGeneralLimit
	}
	step := NewIntegerFromString(ExtraStoragePriceStep)
	if out.Amount.Cmp(step) < 0 {
		return ExtraSizeGeneralLimit
	}
	cells := out.Amount.Count(step)
	limit := cells * ExtraSizeStorageStep
	if limit > ExtraSizeStorageCapacity {
		return ExtraSizeStorageCapacity
	}
	return int(limit)
}

func validateScriptTransaction(inputs map[string]*UTXO) error {
	for _, in := range inputs {
		if in.Type != OutputTypeScript && in.Type != OutputTypeNodeRemove {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}
	return nil
}

func validateReferences(store UTXOLockReader, tx *SignedTransaction) error {
	if len(tx.References) > ReferencesCountLimit {
		return fmt.Errorf("too many references %d", len(tx.References))
	}

	for _, r := range tx.References {
		utxo, err := store.ReadUTXOLock(r, 0)
		if err != nil {
			return err
		}
		if utxo == nil {
			return fmt.Errorf("reference not found %s", r.String())
		}
	}

	return nil
}

func (tx *SignedTransaction) validateInputs(store UTXOLockReader, hash crypto.Hash, txType uint8, fork bool) (map[string]*UTXO, Integer, error) {
	inputAmount := NewInteger(0)
	inputsFilter := make(map[string]*UTXO)
	allKeys := make([]*crypto.Key, 0)
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

		utxo, err := store.ReadUTXOLock(in.Hash, in.Index)
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		if utxo == nil {
			err := fmt.Errorf("input not found %s:%d", in.Hash.String(), in.Index)
			return inputsFilter, inputAmount, err
		}
		if utxo.Asset != tx.Asset {
			err := fmt.Errorf("invalid input asset %s %s", utxo.Asset.String(), tx.Asset.String())
			return inputsFilter, inputAmount, err
		}
		if utxo.LockHash.HasValue() && utxo.LockHash != hash {
			if !fork {
				err := fmt.Errorf("input locked for transaction %s", utxo.LockHash)
				return inputsFilter, inputAmount, err
			}
		}

		err = validateUTXO(i, &utxo.UTXO, tx.SignaturesMap, tx.AggregatedSignature, hash, txType, keySigs, len(allKeys))
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		inputsFilter[fk] = &utxo.UTXO
		inputAmount = inputAmount.Add(utxo.Amount)
		allKeys = append(allKeys, utxo.Keys...)
	}

	if len(keySigs) == 0 && (txType == TransactionTypeNodeAccept || txType == TransactionTypeNodeRemove) {
		return inputsFilter, inputAmount, nil
	}
	if len(keySigs) < len(tx.Inputs) {
		err := fmt.Errorf("batch verification not ready %d %d", len(tx.Inputs), len(keySigs))
		return inputsFilter, inputAmount, err
	}
	if as := tx.AggregatedSignature; as != nil {
		err := crypto.AggregateVerify(&as.Signature, allKeys, as.Signers, hash)
		if err != nil {
			err := fmt.Errorf("aggregate verification failure %s", err)
			return inputsFilter, inputAmount, err
		}
	} else {
		var keys []*crypto.Key
		var sigs []*crypto.Signature
		for k, s := range keySigs {
			keys = append(keys, k)
			sigs = append(sigs, s)
		}
		if !crypto.BatchVerify(hash, keys, sigs) {
			err := fmt.Errorf("batch verification failure %d %d", len(keys), len(sigs))
			return inputsFilter, inputAmount, err
		}
	}
	return inputsFilter, inputAmount, nil
}

func (tx *Transaction) validateOutputs(store GhostLocker, hash crypto.Hash, fork bool) (Integer, error) {
	outputAmount := NewInteger(0)
	ghostKeysFilter := make(map[crypto.Key]bool)
	ghostKeys := make([]*crypto.Key, 0)
	for _, o := range tx.Outputs {
		if len(o.Keys) > SliceCountLimit {
			err := fmt.Errorf("invalid output keys count %d", len(o.Keys))
			return outputAmount, err
		}
		if o.Amount.Sign() <= 0 {
			err := fmt.Errorf("invalid output amount %s", o.Amount.String())
			return outputAmount, err
		}

		if o.Withdrawal != nil {
			outputAmount = outputAmount.Add(o.Amount)
			continue
		}

		for _, k := range o.Keys {
			if ghostKeysFilter[*k] {
				err := fmt.Errorf("invalid output key %s", k.String())
				return outputAmount, err
			}
			ghostKeysFilter[*k] = true
			if !k.CheckKey() {
				err := fmt.Errorf("invalid output key format %s", k.String())
				return outputAmount, err
			}
			ghostKeys = append(ghostKeys, k)
		}

		switch o.Type {
		case OutputTypeWithdrawalSubmit,
			OutputTypeWithdrawalFuel,
			OutputTypeWithdrawalClaim,
			OutputTypeNodePledge,
			OutputTypeNodeCancel,
			OutputTypeNodeAccept:
			if len(o.Keys) != 0 {
				err := fmt.Errorf("invalid output keys count %d for kernel multisig transaction", len(o.Keys))
				return outputAmount, err
			}
			if len(o.Script) != 0 {
				err := fmt.Errorf("invalid output script %s for kernel multisig transaction", o.Script)
				return outputAmount, err
			}
			if o.Mask.HasValue() {
				err := fmt.Errorf("invalid output empty mask %s for kernel multisig transaction", o.Mask)
				return outputAmount, err
			}
		default:
			err := o.Script.VerifyFormat()
			if err != nil {
				return outputAmount, err
			}
			if !o.Mask.HasValue() {
				err := fmt.Errorf("invalid script output empty mask %s", o.Mask)
				return outputAmount, err
			}
			if o.Withdrawal != nil {
				err := fmt.Errorf("invalid script output with withdrawal %s", o.Withdrawal.Address)
				return outputAmount, err
			}
		}
		outputAmount = outputAmount.Add(o.Amount)
	}

	err := store.LockGhostKeys(ghostKeys, hash, fork)
	if err != nil {
		return Zero, err
	}
	return outputAmount, nil
}

func validateUTXO(index int, utxo *UTXO, sigs []map[uint16]*crypto.Signature, as *AggregatedSignature, msg crypto.Hash, txType uint8, keySigs map[*crypto.Key]*crypto.Signature, offset int) error {
	switch utxo.Type {
	case OutputTypeScript, OutputTypeNodeRemove:
		if as != nil {
			signers, limit := 0, offset+len(utxo.Keys)
			for _, m := range as.Signers {
				if m >= limit {
					break
				} else if m < offset {
					continue
				}
				keySigs[utxo.Keys[m-offset]] = nil
				signers += 1
			}
			return utxo.Script.Validate(signers)
		} else {
			for i, sig := range sigs[index] {
				if int(i) >= len(utxo.Keys) {
					return fmt.Errorf("invalid signature map index %d %d", i, len(utxo.Keys))
				}
				keySigs[utxo.Keys[i]] = sig
			}
			return utxo.Script.Validate(len(sigs[index]))
		}
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
