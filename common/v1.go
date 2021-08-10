package common

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

type SignedTransactionV1 struct {
	Transaction
	SignaturesSliceV1 [][]*crypto.Signature `msgpack:"Signatures"`
}

func (signed *SignedTransaction) SignRawV1(key crypto.Key) error {
	msg := MsgpackMarshalPanic(signed.Transaction)

	if len(signed.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d", len(signed.Inputs))
	}
	in := signed.Inputs[0]
	if in.Deposit == nil && in.Mint == nil {
		return fmt.Errorf("invalid input format")
	}
	if in.Deposit != nil {
		err := signed.verifyDepositFormat()
		if err != nil {
			return err
		}
	}
	sig := key.Sign(msg)
	signed.SignaturesSliceV1 = append(signed.SignaturesSliceV1, []*crypto.Signature{&sig})
	return nil
}

func (signed *SignedTransaction) SignInputV1(reader UTXOKeysReader, index int, accounts []*Address) error {
	msg := MsgpackMarshalPanic(signed.Transaction)

	if len(accounts) == 0 {
		return nil
	}
	if index >= len(signed.Inputs) {
		return fmt.Errorf("invalid input index %d/%d", index, len(signed.Inputs))
	}
	in := signed.Inputs[index]
	if in.Deposit != nil || in.Mint != nil {
		return signed.SignRawV1(accounts[0].PrivateSpendKey)
	}

	utxo, err := reader.ReadUTXOKeys(in.Hash, in.Index)
	if err != nil {
		return err
	}
	if utxo == nil {
		return fmt.Errorf("input not found %s:%d", in.Hash.String(), in.Index)
	}

	keysFilter := make(map[string]bool)
	for _, k := range utxo.Keys {
		keysFilter[k.String()] = true
	}

	sigs := make([]*crypto.Signature, 0)
	for _, acc := range accounts {
		priv := crypto.DeriveGhostPrivateKey(&utxo.Mask, &acc.PrivateViewKey, &acc.PrivateSpendKey, uint64(in.Index))
		if !keysFilter[priv.Public().String()] {
			return fmt.Errorf("invalid key for the input %s", acc.String())
		}
		sig := priv.Sign(msg)
		sigs = append(sigs, &sig)
	}
	signed.SignaturesSliceV1 = append(signed.SignaturesSliceV1, sigs)
	return nil
}

func (ver *VersionedTransaction) validateV1(store DataStore, fork bool) error {
	tx := &ver.SignedTransaction
	msg := ver.PayloadMarshal()
	txType := tx.TransactionType()

	if ver.Version != 1 {
		return fmt.Errorf("invalid tx version %d %d", ver.Version, tx.Version)
	}
	if txType == TransactionTypeUnknown {
		return fmt.Errorf("invalid tx type %d", txType)
	}
	if len(tx.Inputs) < 1 || len(tx.Outputs) < 1 {
		return fmt.Errorf("invalid tx inputs or outputs %d %d", len(tx.Inputs), len(tx.Outputs))
	}
	if len(tx.Inputs) != len(tx.SignaturesSliceV1) && txType != TransactionTypeNodeAccept && txType != TransactionTypeNodeRemove {
		return fmt.Errorf("invalid tx signature number %d %d %d", len(tx.Inputs), len(tx.SignaturesSliceV1), txType)
	}
	if len(tx.Extra) > ExtraSizeLimit {
		return fmt.Errorf("invalid extra size %d", len(tx.Extra))
	}
	if len(ver.Marshal()) > config.TransactionMaximumSize {
		return fmt.Errorf("invalid transaction size %d", len(msg))
	}

	inputsFilter, inputAmount, err := validateInputsV1(store, tx, msg, ver.PayloadHash(), txType, fork)
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
		return tx.validateDepositV1(store, msg, ver.PayloadHash(), ver.SignaturesSliceV1)
	case TransactionTypeWithdrawalSubmit:
		return tx.validateWithdrawalSubmit(inputsFilter)
	case TransactionTypeWithdrawalFuel:
		return tx.validateWithdrawalFuel(store, inputsFilter)
	case TransactionTypeWithdrawalClaim:
		return tx.validateWithdrawalClaim(store, inputsFilter, msg)
	case TransactionTypeNodePledge:
		return tx.validateNodePledge(store, inputsFilter)
	case TransactionTypeNodeCancel:
		return tx.validateNodeCancelV1(store, msg, ver.SignaturesSliceV1)
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

func validateInputsV1(store UTXOLockReader, tx *SignedTransaction, msg []byte, hash crypto.Hash, txType uint8, fork bool) (map[string]*UTXO, Integer, error) {
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

		utxo, err := store.ReadUTXOLock(in.Hash, in.Index)
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
			if !fork {
				return inputsFilter, inputAmount, fmt.Errorf("input locked for transaction %s", utxo.LockHash)
			}
		}

		err = validateUTXOV1(i, &utxo.UTXO, tx.SignaturesSliceV1, msg, txType, keySigs)
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		inputsFilter[fk] = &utxo.UTXO
		inputAmount = inputAmount.Add(utxo.Amount)
	}

	return inputsFilter, inputAmount, nil
}

func validateUTXOV1(index int, utxo *UTXO, sigs [][]*crypto.Signature, msg []byte, txType uint8, keySigs map[crypto.Key]*crypto.Signature) error {
	switch utxo.Type {
	case OutputTypeScript, OutputTypeNodeRemove:
		var offset, valid int
		for _, sig := range sigs[index] {
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

func (tx *SignedTransaction) validateDepositV1(store DataStore, msg []byte, payloadHash crypto.Hash, sigs [][]*crypto.Signature) error {
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

	sig, valid := sigs[0][0], false
	if sig == nil {
		return fmt.Errorf("invalid domain signature index for deposit")
	}
	for _, d := range store.ReadDomains() {
		if d.Account.PublicSpendKey.Verify(msg, *sig) {
			valid = true
		}
	}
	if !valid {
		return fmt.Errorf("invalid domain signature for deposit")
	}

	return store.CheckDepositInput(tx.Inputs[0].Deposit, payloadHash)
}

func (tx *Transaction) validateNodeCancelV1(store DataStore, msg []byte, sigs [][]*crypto.Signature) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 2 {
		return fmt.Errorf("invalid outputs count %d for cancel transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for cancel transaction", len(tx.Inputs))
	}
	if len(sigs) != 1 {
		return fmt.Errorf("invalid signatures count %d for cancel transaction", len(sigs))
	}
	if len(sigs[0]) != 1 {
		return fmt.Errorf("invalid signatures count %d for cancel transaction", len(sigs[0]))
	}
	if len(tx.Extra) != len(crypto.Key{})*3 {
		return fmt.Errorf("invalid extra %s for cancel transaction", hex.EncodeToString(tx.Extra))
	}
	cancel, script := tx.Outputs[0], tx.Outputs[1]
	if cancel.Type != OutputTypeNodeCancel || script.Type != OutputTypeScript {
		return fmt.Errorf("invalid outputs type %d %d for cancel transaction", cancel.Type, script.Type)
	}
	if len(script.Keys) != 1 {
		return fmt.Errorf("invalid script output keys %d for cancel transaction", len(script.Keys))
	}
	if script.Script.String() != NewThresholdScript(1).String() {
		return fmt.Errorf("invalid script output script %s for cancel transaction", script.Script)
	}

	var pledging *Node
	filter := make(map[string]string)
	nodes := store.ReadAllNodes(uint64(time.Now().UnixNano()), false) // FIXME offset incorrect
	for _, n := range nodes {
		filter[n.Signer.String()] = n.State
		if n.State == NodeStateAccepted || n.State == NodeStateCancelled || n.State == NodeStateRemoved {
			continue
		}
		if n.State == NodeStatePledging && pledging == nil {
			pledging = n
		} else {
			return fmt.Errorf("invalid pledging nodes %s %s", pledging.Signer.String(), n.Signer.String())
		}
	}
	if pledging == nil {
		return fmt.Errorf("no pledging node needs to get cancelled")
	}
	if pledging.Transaction != tx.Inputs[0].Hash {
		return fmt.Errorf("invalid plede utxo source %s %s", pledging.Transaction, tx.Inputs[0].Hash)
	}

	lastPledge, _, err := store.ReadTransaction(tx.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if len(lastPledge.Outputs) != 1 {
		return fmt.Errorf("invalid pledge utxo count %d", len(lastPledge.Outputs))
	}
	po := lastPledge.Outputs[0]
	if po.Type != OutputTypeNodePledge {
		return fmt.Errorf("invalid pledge utxo type %d", po.Type)
	}
	if cancel.Amount.Cmp(po.Amount.Div(100)) != 0 {
		return fmt.Errorf("invalid script output amount %s for cancel transaction", cancel.Amount)
	}
	var publicSpend crypto.Key
	copy(publicSpend[:], lastPledge.Extra)
	privateView := publicSpend.DeterministicHashDerive()
	acc := Address{
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
	if filter[acc.String()] != NodeStatePledging {
		return fmt.Errorf("invalid pledge utxo source %s", filter[acc.String()])
	}

	pit, _, err := store.ReadTransaction(lastPledge.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if pit == nil {
		return fmt.Errorf("invalid pledge input source %s:%d", lastPledge.Inputs[0].Hash, lastPledge.Inputs[0].Index)
	}
	pi := pit.Outputs[lastPledge.Inputs[0].Index]
	if len(pi.Keys) != 1 {
		return fmt.Errorf("invalid pledge input source keys %d", len(pi.Keys))
	}
	var a crypto.Key
	copy(a[:], tx.Extra[len(crypto.Key{})*2:])
	pledgeSpend := crypto.ViewGhostOutputKey(pi.Keys[0], &a, &pi.Mask, uint64(lastPledge.Inputs[0].Index))
	targetSpend := crypto.ViewGhostOutputKey(script.Keys[0], &a, &script.Mask, 1)
	if !bytes.Equal(lastPledge.Extra, tx.Extra[:len(crypto.Key{})*2]) {
		return fmt.Errorf("invalid pledge and cancel key %s %s", hex.EncodeToString(lastPledge.Extra), hex.EncodeToString(tx.Extra))
	}
	if !bytes.Equal(pledgeSpend[:], targetSpend[:]) {
		return fmt.Errorf("invalid pledge and cancel target %s %s", pledgeSpend, targetSpend)
	}
	if !pi.Keys[0].Verify(msg, *sigs[0][0]) {
		return fmt.Errorf("invalid cancel signature %s", sigs[0][0])
	}
	return nil
}

func decompressUnmarshalVersionedOne(val []byte) (*VersionedTransaction, error) {
	var v1 SignedTransactionV1
	err := DecompressMsgpackUnmarshal(val, &v1)
	if err != nil {
		return nil, err
	}

	ver := &VersionedTransaction{
		SignedTransaction: SignedTransaction{
			Transaction:       v1.Transaction,
			SignaturesSliceV1: v1.SignaturesSliceV1,
		},
	}

	if ver.Version == 1 && len(ver.Inputs) == 1 && hex.EncodeToString(ver.Inputs[0].Genesis) == config.MainnetId {
		var ght SignedGenesisHackTransaction
		err := DecompressMsgpackUnmarshal(val, &ght)
		if err != nil {
			return nil, err
		}
		ver.Version = 0
		ver.BadGenesis = &ght
	}
	return ver, nil
}

func unmarshalVersionedOne(val []byte) (*VersionedTransaction, error) {
	var v1 SignedTransactionV1
	err := MsgpackUnmarshal(val, &v1)
	if err != nil {
		return nil, err
	}

	ver := &VersionedTransaction{
		SignedTransaction: SignedTransaction{
			Transaction:       v1.Transaction,
			SignaturesSliceV1: v1.SignaturesSliceV1,
		},
	}

	if ver.Version == 1 && len(ver.Inputs) == 1 && hex.EncodeToString(ver.Inputs[0].Genesis) == config.MainnetId {
		var ght SignedGenesisHackTransaction
		err := MsgpackUnmarshal(val, &ght)
		if err != nil {
			return nil, err
		}
		ver.Version = 0
		ver.BadGenesis = &ght
	}
	return ver, nil
}

func compressMarshalV1(ver *VersionedTransaction) []byte {
	switch ver.Version {
	case 0:
		return CompressMsgpackMarshalPanic(ver.BadGenesis)
	case 1:
		val := CompressMsgpackMarshalPanic(SignedTransactionV1{
			Transaction:       ver.Transaction,
			SignaturesSliceV1: ver.SignaturesSliceV1,
		})
		if ver.Version == 1 && len(ver.Inputs) == 1 && hex.EncodeToString(ver.Inputs[0].Genesis) == config.MainnetId {
			ver, err := decompressUnmarshalVersionedOne(val)
			if err != nil {
				panic(err)
			}
			return compressMarshalV1(ver)
		}
		return val
	default:
		panic(ver.Version)
	}
}

func marshalV1(ver *VersionedTransaction) []byte {
	switch ver.Version {
	case 0:
		return MsgpackMarshalPanic(ver.BadGenesis)
	case 1:
		val := MsgpackMarshalPanic(SignedTransactionV1{
			Transaction:       ver.Transaction,
			SignaturesSliceV1: ver.SignaturesSliceV1,
		})
		if ver.Version == 1 && len(ver.Inputs) == 1 && hex.EncodeToString(ver.Inputs[0].Genesis) == config.MainnetId {
			ver, err := unmarshalVersionedOne(val)
			if err != nil {
				panic(err)
			}
			return marshalV1(ver)
		}
		return val
	default:
		panic(ver.Version)
	}
}

func payloadMarshalV1(ver *VersionedTransaction) []byte {
	switch ver.Version {
	case 0:
		return MsgpackMarshalPanic(ver.BadGenesis.GenesisHackTransaction)
	case 1:
		val := MsgpackMarshalPanic(ver.Transaction)
		if ver.Version == 1 && len(ver.Inputs) == 1 && hex.EncodeToString(ver.Inputs[0].Genesis) == config.MainnetId {
			ver, err := unmarshalVersionedOne(val)
			if err != nil {
				panic(err)
			}
			return payloadMarshalV1(ver)
		}
		return val
	default:
		panic(ver.Version)
	}
}

type GenesisHackInput struct {
	Hash    crypto.Hash
	Index   int
	Genesis []byte
	Deposit *DepositData
	Rebate  []byte
	Mint    []byte
}

type GenesisHackTransaction struct {
	Version uint8
	Asset   crypto.Hash
	Inputs  []*GenesisHackInput
	Outputs []*Output
	Extra   []byte
}

type SignedGenesisHackTransaction struct {
	GenesisHackTransaction
	Signatures [][]crypto.Signature
}
