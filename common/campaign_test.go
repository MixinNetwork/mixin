package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestMintValidationCampaign(t *testing.T) {
	require := require.New(t)

	tx := NewTransactionV5(XINAssetId)
	store := &campaignStore{
		mintDist: &MintDistribution{
			MintData: MintData{
				Group:  mintGroupUniversal,
				Batch:  5,
				Amount: NewInteger(10),
			},
			Transaction: crypto.Blake3Hash([]byte("mint-dist")),
		},
	}

	require.ErrorContains(tx.AsVersioned().validateMint(store), "invalid inputs count")

	tx.AddUniversalMintInput(5, NewInteger(10))
	tx.Outputs = []*Output{{Type: OutputTypeWithdrawalSubmit, Amount: NewInteger(10)}}
	require.ErrorContains(tx.AsVersioned().validateMint(store), "invalid mint output type")

	tx.Outputs[0].Type = OutputTypeScript
	tx.Asset = BitcoinAssetId
	require.ErrorContains(tx.AsVersioned().validateMint(store), "invalid mint asset")

	tx.Asset = XINAssetId
	tx.Inputs[0].Mint.Group = "bad"
	require.ErrorContains(tx.AsVersioned().validateMint(store), "invalid mint group")

	tx.Inputs[0].Mint.Group = mintGroupUniversal
	store.mintErr = errors.New("mint read failure")
	require.ErrorIs(tx.AsVersioned().validateMint(store), store.mintErr)

	store.mintErr = nil
	store.mintDist = nil
	require.Nil(tx.AsVersioned().validateMint(store))

	store.mintDist = &MintDistribution{
		MintData: MintData{
			Group:  mintGroupUniversal,
			Batch:  6,
			Amount: NewInteger(10),
		},
		Transaction: crypto.Blake3Hash([]byte("mint-lock")),
	}
	require.ErrorContains(tx.AsVersioned().validateMint(store), "backward mint batch")

	tx.Inputs[0].Mint.Batch = 7
	require.Nil(tx.AsVersioned().validateMint(store))

	tx.Inputs[0].Mint.Batch = 6
	require.ErrorContains(tx.AsVersioned().validateMint(store), "invalid mint lock")

	ver := tx.AsVersioned()
	store.mintDist.Transaction = ver.PayloadHash()
	store.mintDist.Amount = tx.Inputs[0].Mint.Amount
	require.Nil(ver.validateMint(store))
}

func TestWithdrawalValidationCampaign(t *testing.T) {
	require := require.New(t)

	validInputs := map[string]*UTXO{
		"script:0": {
			Output: Output{Type: OutputTypeScript},
		},
	}

	submit := &Transaction{
		Outputs: []*Output{{
			Type:       OutputTypeWithdrawalSubmit,
			Amount:     NewInteger(1),
			Withdrawal: &WithdrawalData{Address: "destination", Tag: "memo"},
		}},
	}
	require.Nil(submit.validateWithdrawalSubmit(validInputs))

	require.ErrorContains((&Transaction{
		Outputs: submit.Outputs,
	}).validateWithdrawalSubmit(map[string]*UTXO{
		"bad:0": {Output: Output{Type: OutputTypeNodeAccept}},
	}), "invalid utxo type")

	require.ErrorContains((&Transaction{
		Outputs: []*Output{
			submit.Outputs[0],
			{Type: OutputTypeNodeAccept, Amount: NewInteger(1)},
		},
	}).validateWithdrawalSubmit(validInputs), "invalid change type")

	require.ErrorContains((&Transaction{
		Outputs: []*Output{{Type: OutputTypeScript, Amount: NewInteger(1)}},
	}).validateWithdrawalSubmit(validInputs), "invalid output type")

	require.ErrorContains((&Transaction{
		Outputs: []*Output{{Type: OutputTypeWithdrawalSubmit, Amount: NewInteger(1)}},
	}).validateWithdrawalSubmit(validInputs), "invalid withdrawal submit data")

	mask := crypto.NewKeyFromSeed(bytes.Repeat([]byte{9}, 64)).Public()
	require.ErrorContains((&Transaction{
		Outputs: []*Output{{
			Type:       OutputTypeWithdrawalSubmit,
			Amount:     NewInteger(1),
			Withdrawal: &WithdrawalData{Address: "destination"},
			Keys:       []*crypto.Key{&mask},
		}},
	}).validateWithdrawalSubmit(validInputs), "invalid withdrawal submit keys")

	require.ErrorContains((&Transaction{
		Outputs: []*Output{{
			Type:       OutputTypeWithdrawalSubmit,
			Amount:     NewInteger(1),
			Withdrawal: &WithdrawalData{Address: "destination"},
			Script:     NewThresholdScript(1),
		}},
	}).validateWithdrawalSubmit(validInputs), "invalid withdrawal submit script")

	require.ErrorContains((&Transaction{
		Outputs: []*Output{{
			Type:       OutputTypeWithdrawalSubmit,
			Amount:     NewInteger(1),
			Withdrawal: &WithdrawalData{Address: "destination"},
			Mask:       mask,
		}},
	}).validateWithdrawalSubmit(validInputs), "invalid withdrawal submit mask")

	custodian := deterministicAddress(1)
	submitTx := NewTransactionV5(XINAssetId)
	submitTx.Outputs = []*Output{{
		Type:       OutputTypeWithdrawalSubmit,
		Amount:     NewInteger(1),
		Withdrawal: &WithdrawalData{Address: "destination", Tag: "memo"},
	}}
	submitVer := submitTx.AsVersioned()

	claim := &Transaction{
		Asset: XINAssetId,
		Outputs: []*Output{{
			Type:   OutputTypeWithdrawalClaim,
			Amount: NewIntegerFromString(config.WithdrawalClaimFee),
		}},
		References: []crypto.Hash{submitVer.PayloadHash()},
	}
	store := &campaignStore{
		txs: map[string]*VersionedTransaction{
			submitVer.PayloadHash().String(): submitVer,
		},
		custodian: &CustodianUpdateRequest{Custodian: &custodian},
	}

	claimData := []byte("claim-data")
	claimHash := crypto.Blake3Hash(claimData)
	claimSig := custodian.PrivateSpendKey.Sign(claimHash)
	claim.Extra = append(claimSig[:], claimData...)
	require.Nil(claim.validateWithdrawalClaim(store, validInputs, 1))

	require.ErrorContains((&Transaction{
		Asset:      XINAssetId,
		Outputs:    claim.Outputs,
		References: claim.References,
		Extra:      claim.Extra,
	}).validateWithdrawalClaim(store, map[string]*UTXO{
		"bad:0": {Output: Output{Type: OutputTypeNodeAccept}},
	}, 1), "invalid utxo type")

	require.ErrorContains((&Transaction{
		Asset:      BitcoinAssetId,
		Outputs:    claim.Outputs,
		References: claim.References,
		Extra:      claim.Extra,
	}).validateWithdrawalClaim(store, validInputs, 1), "invalid asset")

	require.ErrorContains((&Transaction{
		Asset: XINAssetId,
		Outputs: []*Output{
			claim.Outputs[0],
			{Type: OutputTypeNodeAccept, Amount: NewInteger(1)},
		},
		References: claim.References,
		Extra:      claim.Extra,
	}).validateWithdrawalClaim(store, validInputs, 1), "invalid change type")

	require.ErrorContains((&Transaction{
		Asset:      XINAssetId,
		Outputs:    claim.Outputs,
		References: nil,
		Extra:      claim.Extra,
	}).validateWithdrawalClaim(store, validInputs, 1), "invalid references count")

	require.ErrorContains((&Transaction{
		Asset: XINAssetId,
		Outputs: []*Output{{
			Type:   OutputTypeScript,
			Amount: claim.Outputs[0].Amount,
		}},
		References: claim.References,
		Extra:      claim.Extra,
	}).validateWithdrawalClaim(store, validInputs, 1), "invalid output type")

	require.ErrorContains((&Transaction{
		Asset: XINAssetId,
		Outputs: []*Output{{
			Type:   OutputTypeWithdrawalClaim,
			Amount: NewIntegerFromString("0.00001"),
		}},
		References: claim.References,
		Extra:      claim.Extra,
	}).validateWithdrawalClaim(store, validInputs, 1), "invalid output amount")

	store.readTxErr = errors.New("read tx failure")
	require.ErrorIs(claim.validateWithdrawalClaim(store, validInputs, 1), store.readTxErr)
	store.readTxErr = nil

	delete(store.txs, submitVer.PayloadHash().String())
	require.ErrorContains(claim.validateWithdrawalClaim(store, validInputs, 1), "invalid withdrawal submit data")
	store.txs[submitVer.PayloadHash().String()] = submitVer

	store.txs[submitVer.PayloadHash().String()] = (&Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{{Type: OutputTypeScript, Amount: NewInteger(1)}},
	}).AsVersioned()
	require.ErrorContains(claim.validateWithdrawalClaim(store, validInputs, 1), "invalid withdrawal submit data")
	store.txs[submitVer.PayloadHash().String()] = submitVer

	shortExtra := *claim
	shortExtra.Extra = []byte{1, 2, 3}
	require.ErrorContains(shortExtra.validateWithdrawalClaim(store, validInputs, 1), "invalid withdrawal claim information")

	store.custodianErr = errors.New("custodian read failure")
	require.ErrorIs(claim.validateWithdrawalClaim(store, validInputs, 1), store.custodianErr)
	store.custodianErr = nil

	other := deterministicAddress(2)
	badHash := crypto.Blake3Hash(claimData)
	badSig := other.PrivateSpendKey.Sign(badHash)
	badClaim := *claim
	badClaim.Extra = append(badSig[:], claimData...)
	require.ErrorContains(badClaim.validateWithdrawalClaim(store, validInputs, 1), "invalid custodian signature")
}

func TestNodePledgeAcceptRemoveCampaign(t *testing.T) {
	require := require.New(t)

	signer := deterministicAddress(10)
	payee := deterministicAddress(11)
	custodian := deterministicAddress(12)
	networkID := crypto.Blake3Hash([]byte("node-network"))

	node := &Node{Signer: signer}
	require.Equal(signer.Hash().ForNetwork(networkID), node.IdForNetwork(networkID))

	extra := append(signer.PublicSpendKey[:], payee.PublicSpendKey[:]...)
	basePledge := &Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: crypto.Blake3Hash([]byte("pledge-input")), Index: 0}},
		Outputs: []*Output{{Type: OutputTypeNodePledge, Amount: KernelNodePledgeAmount}},
		Extra:   extra,
	}

	signerFromExtra := basePledge.NodeTransactionExtraAsSigner()
	require.Equal(signer.PublicSpendKey, signerFromExtra.PublicSpendKey)
	require.Equal(signer.PublicSpendKey.DeterministicHashDerive().Public(), signerFromExtra.PublicViewKey)
	require.Panics(func() {
		(&Transaction{Version: TxVersionHashSignature}).NodeTransactionExtraAsSigner()
	})

	inputs := map[string]*UTXO{
		utxoRef(basePledge.Inputs[0].Hash, 0): {
			Output: Output{Type: OutputTypeScript},
		},
	}
	store := &campaignStore{}

	require.Nil(basePledge.validateNodePledge(store, inputs, 0))

	badPledge := *basePledge
	badPledge.Asset = BitcoinAssetId
	require.ErrorContains(badPledge.validateNodePledge(store, inputs, 0), "invalid node asset")

	badPledge = *basePledge
	badPledge.Outputs = append(badPledge.Outputs, &Output{Type: OutputTypeNodePledge, Amount: KernelNodePledgeAmount})
	require.ErrorContains(badPledge.validateNodePledge(store, inputs, 0), "invalid outputs count")

	badPledge = *basePledge
	badPledge.Inputs = nil
	require.ErrorContains(badPledge.validateNodePledge(store, inputs, 0), "invalid inputs count")

	require.ErrorContains(basePledge.validateNodePledge(store, map[string]*UTXO{
		utxoRef(basePledge.Inputs[0].Hash, 0): {Output: Output{Type: OutputTypeNodeAccept}},
	}, 0), "invalid utxo type")

	badPledge = *basePledge
	badPledge.Extra = []byte{1}
	require.ErrorContains(badPledge.validateNodePledge(store, inputs, 0), "invalid extra length")

	store.nodes = []*Node{{Signer: custodian, State: NodeStatePledging}}
	require.ErrorContains(basePledge.validateNodePledge(store, inputs, 0), "invalid node pending state")

	store.nodes = []*Node{{Signer: *signerFromExtra, State: NodeStateAccepted}}
	require.ErrorContains(basePledge.validateNodePledge(store, inputs, 0), "invalid node signer key")

	store.nodes = []*Node{{Payee: *signerFromExtra, State: NodeStateAccepted}}
	require.ErrorContains(basePledge.validateNodePledge(store, inputs, 0), "invalid node signer key")

	store.nodes = nil
	require.Nil(basePledge.validateNodePledge(store, inputs, 0))

	lastPledge := &Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: crypto.Blake3Hash([]byte("stake")), Index: 0}},
		Outputs: []*Output{{Type: OutputTypeNodePledge, Amount: KernelNodePledgeAmount}},
		Extra:   extra,
	}
	lastPledgeVer := lastPledge.AsVersioned()
	store = &campaignStore{
		txs: map[string]*VersionedTransaction{
			lastPledgeVer.PayloadHash().String(): lastPledgeVer,
		},
		nodes: []*Node{{
			Signer:      *lastPledge.NodeTransactionExtraAsSigner(),
			State:       NodeStatePledging,
			Transaction: lastPledgeVer.PayloadHash(),
		}},
	}

	accept := &Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: lastPledgeVer.PayloadHash(), Index: 0}},
		Outputs: []*Output{{Type: OutputTypeNodeAccept, Amount: KernelNodePledgeAmount}},
		Extra:   append([]byte{}, lastPledge.Extra...),
	}
	require.Nil(accept.validateNodeAccept(store, 0))

	badAccept := *accept
	badAccept.Asset = BitcoinAssetId
	require.ErrorContains(badAccept.validateNodeAccept(store, 0), "invalid node asset")

	badAccept = *accept
	badAccept.Outputs = append(badAccept.Outputs, &Output{Type: OutputTypeNodeAccept, Amount: KernelNodePledgeAmount})
	require.ErrorContains(badAccept.validateNodeAccept(store, 0), "invalid outputs count")

	badAccept = *accept
	badAccept.Inputs = append(badAccept.Inputs, &Input{Hash: crypto.Blake3Hash([]byte("extra")), Index: 0})
	require.ErrorContains(badAccept.validateNodeAccept(store, 0), "invalid inputs count")

	store.nodes = []*Node{
		{Signer: deterministicAddress(13), State: NodeStatePledging},
		{Signer: deterministicAddress(14), State: NodeStatePledging},
	}
	require.ErrorContains(accept.validateNodeAccept(store, 0), "invalid pledging nodes")

	store.nodes = nil
	require.ErrorContains(accept.validateNodeAccept(store, 0), "no pledging node")

	store.nodes = []*Node{{
		Signer:      *lastPledge.NodeTransactionExtraAsSigner(),
		State:       NodeStatePledging,
		Transaction: crypto.Blake3Hash([]byte("other")),
	}}
	require.ErrorContains(accept.validateNodeAccept(store, 0), "invalid pledge utxo source")

	store.readTxErr = errors.New("accept read failure")
	store.nodes = []*Node{{
		Signer:      *lastPledge.NodeTransactionExtraAsSigner(),
		State:       NodeStatePledging,
		Transaction: lastPledgeVer.PayloadHash(),
	}}
	require.ErrorIs(accept.validateNodeAccept(store, 0), store.readTxErr)
	store.readTxErr = nil

	store.txs[lastPledgeVer.PayloadHash().String()] = (&Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{},
	}).AsVersioned()
	require.ErrorContains(accept.validateNodeAccept(store, 0), "invalid pledge utxo count")
	store.txs[lastPledgeVer.PayloadHash().String()] = lastPledgeVer

	store.txs[lastPledgeVer.PayloadHash().String()] = (&Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{{Type: OutputTypeScript, Amount: KernelNodePledgeAmount}},
		Extra:   lastPledge.Extra,
	}).AsVersioned()
	require.ErrorContains(accept.validateNodeAccept(store, 0), "invalid pledge utxo type")
	store.txs[lastPledgeVer.PayloadHash().String()] = lastPledgeVer

	store.nodes = []*Node{{
		Signer:      deterministicAddress(15),
		State:       NodeStatePledging,
		Transaction: lastPledgeVer.PayloadHash(),
	}}
	require.ErrorContains(accept.validateNodeAccept(store, 0), "invalid pledge utxo source")

	store.nodes = []*Node{{
		Signer:      *lastPledge.NodeTransactionExtraAsSigner(),
		State:       NodeStatePledging,
		Transaction: lastPledgeVer.PayloadHash(),
	}}
	badAccept = *accept
	badAccept.Extra = append([]byte{}, accept.Extra...)
	badAccept.Extra[0] ^= 0xff
	require.ErrorContains(badAccept.validateNodeAccept(store, 0), "invalid pledge and accept key")

	store = &campaignStore{
		txs: map[string]*VersionedTransaction{
			accept.AsVersioned().PayloadHash().String(): accept.AsVersioned(),
		},
	}
	remove := &Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: accept.AsVersioned().PayloadHash(), Index: 0}},
		Outputs: []*Output{{Type: OutputTypeNodeRemove, Amount: KernelNodePledgeAmount}},
		Extra:   append([]byte{}, accept.Extra...),
	}
	require.Nil(remove.validateNodeRemove(store))

	badRemove := *remove
	badRemove.Asset = BitcoinAssetId
	require.ErrorContains(badRemove.validateNodeRemove(store), "invalid node asset")

	badRemove = *remove
	badRemove.Outputs = append(badRemove.Outputs, &Output{Type: OutputTypeNodeRemove, Amount: KernelNodePledgeAmount})
	require.ErrorContains(badRemove.validateNodeRemove(store), "invalid outputs count")

	badRemove = *remove
	badRemove.Inputs = append(badRemove.Inputs, &Input{Hash: crypto.Blake3Hash([]byte("extra")), Index: 0})
	require.ErrorContains(badRemove.validateNodeRemove(store), "invalid inputs count")

	store.readTxErr = errors.New("remove read failure")
	require.ErrorIs(remove.validateNodeRemove(store), store.readTxErr)
	store.readTxErr = nil

	malformedHash := crypto.Blake3Hash([]byte("malformed"))
	store.txs = map[string]*VersionedTransaction{malformedHash.String(): accept.AsVersioned()}
	badRemove = *remove
	badRemove.Inputs = []*Input{{Hash: malformedHash, Index: 0}}
	require.ErrorContains(badRemove.validateNodeRemove(store), "accept transaction malformed")

	countVer := (&Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{},
		Extra:   accept.Extra,
	}).AsVersioned()
	countVer.hash = remove.Inputs[0].Hash
	store.txs = map[string]*VersionedTransaction{
		remove.Inputs[0].Hash.String(): countVer,
	}
	require.ErrorContains(remove.validateNodeRemove(store), "invalid accept utxo count")

	typeVer := (&Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{{Type: OutputTypeScript, Amount: KernelNodePledgeAmount}},
		Extra:   accept.Extra,
	}).AsVersioned()
	typeVer.hash = remove.Inputs[0].Hash
	store.txs = map[string]*VersionedTransaction{
		remove.Inputs[0].Hash.String(): typeVer,
	}
	require.ErrorContains(remove.validateNodeRemove(store), "invalid accept utxo type")

	store.txs = map[string]*VersionedTransaction{
		remove.Inputs[0].Hash.String(): accept.AsVersioned(),
	}
	badRemove = *remove
	badRemove.Extra = append([]byte{}, remove.Extra...)
	badRemove.Extra[0] ^= 0xff
	require.ErrorContains(badRemove.validateNodeRemove(store), "invalid accept and remove key")
}

func TestNodeCancelCampaign(t *testing.T) {
	require := require.New(t)

	store, tx, hash, sigs := buildNodeCancelScenario()
	require.Nil(tx.validateNodeCancel(store, hash, sigs, 0))

	bad := *tx
	bad.Asset = BitcoinAssetId
	require.ErrorContains(bad.validateNodeCancel(store, hash, sigs, 0), "invalid node asset")

	bad = *tx
	bad.Outputs = bad.Outputs[:1]
	require.ErrorContains(bad.validateNodeCancel(store, hash, sigs, 0), "invalid outputs count")

	bad = *tx
	bad.Inputs = nil
	require.ErrorContains(bad.validateNodeCancel(store, hash, sigs, 0), "invalid inputs count")

	require.ErrorContains(tx.validateNodeCancel(store, hash, nil, 0), "invalid signatures")

	bad = *tx
	bad.Extra = []byte{1}
	require.ErrorContains(bad.validateNodeCancel(store, hash, sigs, 0), "invalid extra")

	bad = *tx
	bad.Outputs = []*Output{
		{Type: OutputTypeNodeAccept, Amount: tx.Outputs[0].Amount},
		tx.Outputs[1],
	}
	require.ErrorContains(bad.validateNodeCancel(store, hash, sigs, 0), "invalid outputs type")

	bad = *tx
	scriptCopy := *tx.Outputs[1]
	scriptCopy.Keys = nil
	bad.Outputs = []*Output{tx.Outputs[0], &scriptCopy}
	require.ErrorContains(bad.validateNodeCancel(store, hash, sigs, 0), "invalid script output keys")

	bad = *tx
	scriptCopy = *tx.Outputs[1]
	scriptCopy.Script = NewThresholdScript(2)
	bad.Outputs = []*Output{tx.Outputs[0], &scriptCopy}
	require.ErrorContains(bad.validateNodeCancel(store, hash, sigs, 0), "invalid script output script")

	noNodeStore, _, _, _ := buildNodeCancelScenario()
	noNodeStore.nodes = nil
	require.ErrorContains(tx.validateNodeCancel(noNodeStore, hash, sigs, 0), "no pledging node")

	dupNodeStore, _, _, _ := buildNodeCancelScenario()
	dupNodeStore.nodes = append(dupNodeStore.nodes, &Node{
		Signer:      deterministicAddress(40),
		State:       NodeStatePledging,
		Transaction: crypto.Blake3Hash([]byte("other-pledge")),
	})
	require.ErrorContains(tx.validateNodeCancel(dupNodeStore, hash, sigs, 0), "invalid pledging nodes")

	mismatchStore, badTx, _, badSigs := buildNodeCancelScenario()
	badTx.Inputs[0].Hash = crypto.Blake3Hash([]byte("wrong-source"))
	require.ErrorContains(badTx.validateNodeCancel(mismatchStore, badTx.AsVersioned().PayloadHash(), badSigs, 0), "invalid pledge utxo source")

	readErrStore, _, _, _ := buildNodeCancelScenario()
	readErrStore.readTxErr = errors.New("cancel read failure")
	require.ErrorIs(tx.validateNodeCancel(readErrStore, hash, sigs, 0), readErrStore.readTxErr)

	countStore, _, _, _ := buildNodeCancelScenario()
	lastPledgeHash := tx.Inputs[0].Hash
	countStore.txs[lastPledgeHash.String()] = (&Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{},
		Extra:   countStore.txs[lastPledgeHash.String()].Extra,
	}).AsVersioned()
	require.ErrorContains(tx.validateNodeCancel(countStore, hash, sigs, 0), "invalid pledge utxo count")

	typeStore, _, _, _ := buildNodeCancelScenario()
	typeStore.txs[tx.Inputs[0].Hash.String()] = (&Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: crypto.Blake3Hash([]byte("pit")), Index: 1}},
		Outputs: []*Output{{Type: OutputTypeScript, Amount: NewInteger(100)}},
		Extra:   typeStore.txs[tx.Inputs[0].Hash.String()].Extra,
	}).AsVersioned()
	require.ErrorContains(tx.validateNodeCancel(typeStore, hash, sigs, 0), "invalid pledge utxo type")

	amountStore, _, _, _ := buildNodeCancelScenario()
	bad = *tx
	cancelCopy := *tx.Outputs[0]
	cancelCopy.Amount = NewInteger(2)
	bad.Outputs = []*Output{&cancelCopy, tx.Outputs[1]}
	require.ErrorContains(bad.validateNodeCancel(amountStore, hash, sigs, 0), "invalid script output amount")

	filterStore, _, _, _ := buildNodeCancelScenario()
	filterStore.nodes = []*Node{{
		Signer:      deterministicAddress(41),
		State:       NodeStatePledging,
		Transaction: tx.Inputs[0].Hash,
	}}
	require.ErrorContains(tx.validateNodeCancel(filterStore, hash, sigs, 0), "invalid pledge utxo source")

	pitStore, _, _, _ := buildNodeCancelScenario()
	delete(pitStore.txs, pitStore.txs[tx.Inputs[0].Hash.String()].Inputs[0].Hash.String())
	require.ErrorContains(tx.validateNodeCancel(pitStore, hash, sigs, 0), "invalid pledge input source")

	keysStore, _, _, _ := buildNodeCancelScenario()
	last := keysStore.txs[tx.Inputs[0].Hash.String()]
	pitHash := last.Inputs[0].Hash
	pit := keysStore.txs[pitHash.String()]
	pit.Outputs[1].Keys = nil
	require.ErrorContains(tx.validateNodeCancel(keysStore, hash, sigs, 0), "invalid pledge input source keys")

	keyStore, badKeyTx, badHash, badKeySigs := buildNodeCancelScenario()
	badKeyTx.Extra = append([]byte{}, badKeyTx.Extra...)
	badKeyTx.Extra[0] ^= 0xff
	require.ErrorContains(badKeyTx.validateNodeCancel(keyStore, badHash, badKeySigs, 0), "invalid pledge and cancel key")

	targetStore, badTargetTx, badTargetHash, badTargetSigs := buildNodeCancelScenario()
	otherOwner := deterministicAddress(42)
	otherUTXO, _ := makeScriptUTXO(otherOwner, crypto.Blake3Hash([]byte("other")), 1, NewInteger(99))
	badTargetScript := *badTargetTx.Outputs[1]
	badTargetScript.Keys = []*crypto.Key{otherUTXO.Keys[0]}
	badTargetScript.Mask = otherUTXO.Mask
	badTargetTx.Outputs = []*Output{badTargetTx.Outputs[0], &badTargetScript}
	require.ErrorContains(badTargetTx.validateNodeCancel(targetStore, badTargetHash, badTargetSigs, 0), "invalid pledge and cancel target")

	sigStore, _, _, _ := buildNodeCancelScenario()
	badSigner := deterministicAddress(43)
	badSig := badSigner.PrivateSpendKey.Sign(hash)
	require.ErrorContains(tx.validateNodeCancel(sigStore, hash, []map[uint16]*crypto.Signature{{0: &badSig}}, 0), "invalid cancel signature")
}

func TestValidationHelpersCampaign(t *testing.T) {
	require := require.New(t)

	account := deterministicAddress(50)
	utxoHash := crypto.Blake3Hash([]byte("validation-utxo"))
	utxo, ghost := makeScriptUTXO(account, utxoHash, 0, NewInteger(10))
	store := &campaignStore{
		asset:   XINAsset,
		balance: NewInteger(100),
		txs:     make(map[string]*VersionedTransaction),
		utxos: map[string]*UTXOWithLock{
			utxoRef(utxoHash, 0): utxo,
		},
	}

	var invalidVersion VersionedTransaction
	invalidVersion.Version = 1
	require.ErrorContains(invalidVersion.Validate(store, 0, false), "invalid tx version")

	unknown := NewTransactionV5(XINAssetId)
	unknown.Inputs = []*Input{{Genesis: []byte("genesis")}}
	unknown.Outputs = []*Output{{Type: OutputTypeScript, Amount: NewInteger(1), Keys: []*crypto.Key{utxo.Keys[0]}, Mask: utxo.Mask, Script: NewThresholdScript(1)}}
	require.ErrorContains(unknown.AsVersioned().Validate(store, 0, false), "invalid tx type")

	require.ErrorContains(NewTransactionV5(XINAssetId).AsVersioned().Validate(store, 0, false), "invalid tx inputs or outputs")

	extraTx := NewTransactionV5(BitcoinAssetId)
	extraTx.AddUniversalMintInput(1, NewInteger(1))
	extraTx.Outputs = []*Output{{Type: OutputTypeScript, Amount: NewInteger(1)}}
	extraTx.Extra = make([]byte, ExtraSizeGeneralLimit+1)
	require.ErrorContains(extraTx.AsVersioned().Validate(store, 0, false), "invalid extra size")

	sigTx := NewTransactionV5(XINAssetId)
	sigTx.AddInput(utxoHash, 0)
	sigTx.Outputs = []*Output{{Type: OutputTypeScript, Amount: NewInteger(10), Keys: utxo.Keys, Mask: utxo.Mask, Script: NewThresholdScript(1)}}
	ver := sigTx.AsVersioned()
	ver.AggregatedSignature = &AggregatedSignature{}
	ver.SignaturesMap = []map[uint16]*crypto.Signature{{}}
	require.ErrorContains(ver.Validate(store, 0, false), "invalid signatures map")

	ver = sigTx.AsVersioned()
	require.ErrorContains(ver.Validate(store, 0, false), "invalid tx signature number")

	refTx := &SignedTransaction{}
	for i := 0; i < ReferencesCountLimit+1; i++ {
		refTx.References = append(refTx.References, crypto.Blake3Hash([]byte(fmt.Sprintf("ref-%d", i))))
	}
	require.ErrorContains(validateReferences(store, refTx), "too many references")

	store.readTxErr = errors.New("reference read failure")
	require.ErrorIs(validateReferences(store, &SignedTransaction{Transaction: Transaction{References: []crypto.Hash{crypto.Blake3Hash([]byte("ref"))}}}), store.readTxErr)
	store.readTxErr = nil

	require.ErrorContains(validateReferences(store, &SignedTransaction{Transaction: Transaction{References: []crypto.Hash{crypto.Blake3Hash([]byte("missing"))}}}), "reference not found")

	refVer := (&Transaction{Version: TxVersionHashSignature, Asset: XINAssetId, Outputs: []*Output{{Type: OutputTypeScript, Amount: NewInteger(1)}}}).AsVersioned()
	store.txs[crypto.Blake3Hash([]byte("existing")).String()] = refVer
	require.Nil(validateReferences(store, &SignedTransaction{Transaction: Transaction{References: []crypto.Hash{crypto.Blake3Hash([]byte("existing"))}}}))

	require.ErrorContains(validateScriptTransaction(map[string]*UTXO{
		"bad:0": {Output: Output{Type: OutputTypeNodeAccept}},
	}), "invalid utxo type")
	require.Nil(validateScriptTransaction(map[string]*UTXO{
		"ok:0": {Output: Output{Type: OutputTypeNodeRemove}},
	}))

	require.ErrorContains(validateUTXO(0, &utxo.UTXO, nil, &AggregatedSignature{Signers: []int{1, 0}}, TransactionTypeScript, map[*crypto.Key]*crypto.Signature{}, 0), "invalid aggregated signer order")
	require.ErrorContains(validateUTXO(0, &utxo.UTXO, []map[uint16]*crypto.Signature{{1: nil}}, nil, TransactionTypeScript, map[*crypto.Key]*crypto.Signature{}, 0), "invalid signature map index")
	require.Nil(validateUTXO(0, &UTXO{Output: Output{Type: OutputTypeNodePledge}}, nil, nil, TransactionTypeNodeAccept, map[*crypto.Key]*crypto.Signature{}, 0))
	require.ErrorContains(validateUTXO(0, &UTXO{Output: Output{Type: OutputTypeNodePledge}}, nil, nil, TransactionTypeScript, map[*crypto.Key]*crypto.Signature{}, 0), "pledge input used")
	require.Nil(validateUTXO(0, &UTXO{Output: Output{Type: OutputTypeNodeAccept}}, nil, nil, TransactionTypeNodeRemove, map[*crypto.Key]*crypto.Signature{}, 0))
	require.ErrorContains(validateUTXO(0, &UTXO{Output: Output{Type: OutputTypeNodeAccept}}, nil, nil, TransactionTypeScript, map[*crypto.Key]*crypto.Signature{}, 0), "accept input used")
	require.ErrorContains(validateUTXO(0, &UTXO{Output: Output{Type: OutputTypeNodeCancel}}, nil, nil, TransactionTypeScript, map[*crypto.Key]*crypto.Signature{}, 0), "should do more validation")
	require.ErrorContains(validateUTXO(0, &UTXO{Output: Output{Type: OutputTypeWithdrawalSubmit}}, nil, nil, TransactionTypeScript, map[*crypto.Key]*crypto.Signature{}, 0), "invalid input type")

	outputStore := &campaignStore{}
	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Keys:   make([]*crypto.Key, SliceCountLimit+1),
		Mask:   utxo.Mask,
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid output keys count")

	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: Zero,
		Keys:   utxo.Keys,
		Mask:   utxo.Mask,
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid output amount")

	require.ErrorContains((&Transaction{Outputs: []*Output{
		{Type: OutputTypeScript, Amount: NewInteger(1), Keys: utxo.Keys, Mask: utxo.Mask, Script: NewThresholdScript(1)},
		{Type: OutputTypeScript, Amount: NewInteger(1), Keys: utxo.Keys, Mask: utxo.Mask, Script: NewThresholdScript(1)},
	}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(2), false), "invalid output key")

	var identity crypto.Key
	identity[0] = 1
	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Keys:   []*crypto.Key{&identity},
		Mask:   utxo.Mask,
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid output key format")

	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeNodeAccept,
		Amount: NewInteger(1),
		Keys:   utxo.Keys,
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid output keys count")

	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Mask:   utxo.Mask,
		Script: Script{1},
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid script length")

	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Mask:   crypto.Key{},
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid script output empty mask")

	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Mask:   identity,
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid output mask format")

	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:       OutputTypeScript,
		Amount:     NewInteger(1),
		Keys:       utxo.Keys,
		Mask:       utxo.Mask,
		Script:     NewThresholdScript(1),
		Withdrawal: &WithdrawalData{Address: "bad"},
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), "invalid script output with withdrawal")

	require.ErrorContains((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Keys:   utxo.Keys,
		Mask:   utxo.Mask,
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(2), false), "invalid input output amount")

	outputStore.ghostErr = errors.New("ghost lock failure")
	require.ErrorIs((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Keys:   utxo.Keys,
		Mask:   utxo.Mask,
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false), outputStore.ghostErr)
	outputStore.ghostErr = nil
	require.Nil((&Transaction{Outputs: []*Output{{
		Type:   OutputTypeScript,
		Amount: NewInteger(1),
		Keys:   utxo.Keys,
		Mask:   utxo.Mask,
		Script: NewThresholdScript(1),
	}}}).validateOutputs(outputStore, crypto.Blake3Hash([]byte("out")), NewInteger(1), false))

	mintInput := &SignedTransaction{Transaction: Transaction{Inputs: []*Input{{Mint: &MintData{Batch: 1, Amount: NewInteger(7)}}}}}
	_, amount, err := mintInput.validateInputs(store, crypto.Hash{}, TransactionTypeMint, false)
	require.Nil(err)
	require.Equal(NewInteger(7), amount)

	depositInput := &SignedTransaction{Transaction: Transaction{Inputs: []*Input{{Deposit: &DepositData{Amount: NewInteger(8)}}}}}
	_, amount, err = depositInput.validateInputs(store, crypto.Hash{}, TransactionTypeDeposit, false)
	require.Nil(err)
	require.Equal(NewInteger(8), amount)

	_, _, err = (&SignedTransaction{Transaction: Transaction{
		Inputs: []*Input{{Hash: utxoHash, Index: 0, Genesis: []byte("genesis")}},
		Asset:  XINAssetId,
	}}).validateInputs(store, crypto.Hash{}, TransactionTypeScript, false)
	require.ErrorContains(err, "invalid genesis")

	dup := &SignedTransaction{Transaction: Transaction{
		Asset:  XINAssetId,
		Inputs: []*Input{{Hash: utxoHash, Index: 0}, {Hash: utxoHash, Index: 0}},
	}}
	dup.SignaturesMap = []map[uint16]*crypto.Signature{{0: nil}}
	_, _, err = dup.validateInputs(store, crypto.Hash{}, TransactionTypeScript, false)
	require.ErrorContains(err, "invalid input")

	store.readUTXOErr = errors.New("utxo read failure")
	_, _, err = (&SignedTransaction{Transaction: Transaction{
		Asset:  XINAssetId,
		Inputs: []*Input{{Hash: utxoHash, Index: 0}},
	}}).validateInputs(store, crypto.Hash{}, TransactionTypeScript, false)
	require.ErrorIs(err, store.readUTXOErr)
	store.readUTXOErr = nil

	_, _, err = (&SignedTransaction{Transaction: Transaction{
		Asset:  XINAssetId,
		Inputs: []*Input{{Hash: crypto.Blake3Hash([]byte("missing")), Index: 0}},
	}}).validateInputs(store, crypto.Hash{}, TransactionTypeScript, false)
	require.ErrorContains(err, "input not found")

	store.utxos[utxoRef(utxoHash, 0)].Asset = BitcoinAssetId
	_, _, err = (&SignedTransaction{Transaction: Transaction{
		Asset:  XINAssetId,
		Inputs: []*Input{{Hash: utxoHash, Index: 0}},
	}}).validateInputs(store, crypto.Hash{}, TransactionTypeScript, false)
	require.ErrorContains(err, "invalid input asset")
	store.utxos[utxoRef(utxoHash, 0)].Asset = XINAssetId

	store.utxos[utxoRef(utxoHash, 0)].LockHash = crypto.Blake3Hash([]byte("locked"))
	_, _, err = (&SignedTransaction{Transaction: Transaction{
		Asset:  XINAssetId,
		Inputs: []*Input{{Hash: utxoHash, Index: 0}},
	}}).validateInputs(store, crypto.Blake3Hash([]byte("other")), TransactionTypeScript, false)
	require.ErrorContains(err, "input locked for transaction")

	lockedFork := &SignedTransaction{Transaction: Transaction{
		Asset:  XINAssetId,
		Inputs: []*Input{{Hash: utxoHash, Index: 0}},
	}}
	lockedFork.SignaturesMap = []map[uint16]*crypto.Signature{{}}
	store.utxos[utxoRef(utxoHash, 0)].Script = NewThresholdScript(0)
	_, _, err = lockedFork.validateInputs(store, crypto.Blake3Hash([]byte("other")), TransactionTypeScript, true)
	require.ErrorContains(err, "batch verification not ready")
	store.utxos[utxoRef(utxoHash, 0)].Script = NewThresholdScript(1)
	store.utxos[utxoRef(utxoHash, 0)].LockHash = crypto.Hash{}

	nodePledgeUTXO := &UTXOWithLock{UTXO: UTXO{
		Input:  Input{Hash: crypto.Blake3Hash([]byte("pledge")), Index: 0},
		Output: Output{Type: OutputTypeNodePledge, Amount: NewInteger(10)},
		Asset:  XINAssetId,
	}}
	store.utxos[utxoRef(nodePledgeUTXO.Hash, nodePledgeUTXO.Index)] = nodePledgeUTXO
	_, _, err = (&SignedTransaction{Transaction: Transaction{
		Asset:  XINAssetId,
		Inputs: []*Input{{Hash: nodePledgeUTXO.Hash, Index: nodePledgeUTXO.Index}},
	}}).validateInputs(store, crypto.Hash{}, TransactionTypeNodeAccept, false)
	require.Nil(err)

	validSigTx := &SignedTransaction{Transaction: Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: utxoHash, Index: 0}},
	}}
	hash := validSigTx.AsVersioned().PayloadHash()
	sig := ghost.Sign(hash)
	validSigTx.SignaturesMap = []map[uint16]*crypto.Signature{{0: &sig}}
	inputsFilter, inputAmount, err := validSigTx.validateInputs(store, hash, TransactionTypeScript, false)
	require.Nil(err)
	require.Equal(NewInteger(10), inputAmount)
	require.Contains(inputsFilter, utxoRef(utxoHash, 0))

	require.Equal(ExtraSizeGeneralLimit, (&SignedTransaction{Transaction: Transaction{Version: TxVersionHashSignature, Asset: BitcoinAssetId}}).GetExtraLimit())
}

func TestSignUTXOAndTransactionTypeCampaign(t *testing.T) {
	require := require.New(t)

	account := deterministicAddress(60)
	utxoHash := crypto.Blake3Hash([]byte("sign-utxo"))
	utxo, ghost := makeScriptUTXO(account, utxoHash, 0, NewInteger(10))

	signed := &SignedTransaction{Transaction: *NewTransactionV5(XINAssetId)}
	require.Nil(signed.SignUTXO(&utxo.UTXO, nil))

	wrongAccount := deterministicAddress(61)
	require.ErrorContains(signed.SignUTXO(&utxo.UTXO, []*Address{&wrongAccount}), "invalid key for the input")

	require.Nil(signed.SignUTXO(&utxo.UTXO, []*Address{&account}))
	require.Len(signed.SignaturesMap, 1)
	sig := signed.SignaturesMap[0][0]
	require.NotNil(sig)
	require.True(utxo.Keys[0].Verify(signed.AsVersioned().PayloadHash(), *sig))
	require.Equal(ghost.Public(), *utxo.Keys[0])

	tx := NewTransactionV5(XINAssetId).AsVersioned()
	tx.Outputs = []*Output{{Type: OutputTypeNodePledge, Amount: NewInteger(1)}}
	require.EqualValues(TransactionTypeNodePledge, tx.TransactionType())
	tx.Outputs[0].Type = OutputTypeNodeAccept
	require.EqualValues(TransactionTypeNodeAccept, tx.TransactionType())
	tx.Outputs[0].Type = OutputTypeNodeRemove
	require.EqualValues(TransactionTypeNodeRemove, tx.TransactionType())
	tx.Outputs[0].Type = OutputTypeWithdrawalClaim
	require.EqualValues(TransactionTypeWithdrawalClaim, tx.TransactionType())
	tx.Outputs[0].Type = OutputTypeCustodianUpdateNodes
	require.EqualValues(TransactionTypeCustodianUpdateNodes, tx.TransactionType())
	tx.Outputs[0].Type = OutputTypeCustodianSlashNodes
	require.EqualValues(TransactionTypeCustodianSlashNodes, tx.TransactionType())
}

func TestDepositAndCapacityCampaign(t *testing.T) {
	require := require.New(t)

	for _, tc := range []struct {
		id   crypto.Hash
		want string
	}{
		{BitcoinAssetId, "2500.00000000"},
		{EthereumAssetId, "5000.00000000"},
		{XINAssetId, "750000.00000000"},
		{BOXAssetId, "200000000.00000000"},
		{MOBAssetId, "30000000.00000000"},
		{USDTEthereumAssetId, "12000000.00000000"},
		{USDTTRONAssetId, "20000000.00000000"},
		{PandoUSDAssetId, "1000000000000.00000000"},
		{USDCAssetId, "3000000.00000000"},
		{EOSAssetId, "3500000.00000000"},
		{SOLAssetId, "60000.00000000"},
		{UNIAssetId, "1100000.00000000"},
		{DOGEAssetId, "25000000.00000000"},
	} {
		require.Equal(tc.want, GetAssetCapacity(tc.id).String())
	}

	custodian := deterministicAddress(140)
	receiver := deterministicAddress(141)
	deposit := &DepositData{
		Chain:       BitcoinAssetId,
		AssetKey:    "btc",
		Transaction: "deposit-transaction",
		Index:       0,
		Amount:      NewInteger(5),
	}
	tx := NewTransactionV5(XINAssetId)
	tx.AddDepositInput(deposit)
	tx.AddScriptOutput([]*Address{&receiver}, NewThresholdScript(1), deposit.Amount, bytes.Repeat([]byte{3}, 64))
	store := &campaignStore{
		asset:     &Asset{Chain: deposit.Chain, AssetKey: deposit.AssetKey},
		balance:   NewInteger(10),
		custodian: &CustodianUpdateRequest{Custodian: &custodian},
	}

	require.Nil(tx.verifyDepositData(store))

	badDeposit := *deposit
	badDeposit.Chain = crypto.Hash{}
	tx.Inputs[0].Deposit = &badDeposit
	require.ErrorContains(tx.verifyDepositData(store), "invalid asset data")

	badDeposit = *deposit
	badDeposit.Amount = Zero
	tx.Inputs[0].Deposit = &badDeposit
	require.ErrorContains(tx.verifyDepositData(store), "invalid amount")

	badDeposit = *deposit
	badDeposit.Transaction = " bad "
	tx.Inputs[0].Deposit = &badDeposit
	require.ErrorContains(tx.verifyDepositData(store), "invalid transaction hash")

	tx.Inputs[0].Deposit = deposit
	store.assetErr = errors.New("asset read failure")
	require.ErrorIs(tx.verifyDepositData(store), store.assetErr)
	store.assetErr = nil

	store.asset = nil
	require.Nil(tx.verifyDepositData(store))
	store.asset = &Asset{Chain: deposit.Chain, AssetKey: deposit.AssetKey}

	store.balance = NewIntegerFromString("749995")
	require.ErrorContains(tx.verifyDepositData(store), "invalid deposit capacity")
	store.balance = NewInteger(10)

	store.asset = &Asset{Chain: EthereumAssetId, AssetKey: "eth"}
	require.ErrorContains(tx.verifyDepositData(store), "invalid asset info")
	store.asset = &Asset{Chain: deposit.Chain, AssetKey: deposit.AssetKey}

	ver := tx.AsVersioned()
	payload := ver.PayloadHash()
	require.ErrorContains(ver.validateDeposit(store, payload, nil, 0), "invalid signatures count")
	require.ErrorContains(ver.validateDeposit(store, payload, []map[uint16]*crypto.Signature{{0: nil}}, 0), "invalid custodian signature index")

	store.custodianErr = errors.New("custodian read failure")
	sig := custodian.PrivateSpendKey.Sign(payload)
	require.ErrorIs(ver.validateDeposit(store, payload, []map[uint16]*crypto.Signature{{0: &sig}}, 0), store.custodianErr)
	store.custodianErr = nil

	wrong := deterministicAddress(142)
	badSig := wrong.PrivateSpendKey.Sign(payload)
	require.ErrorContains(ver.validateDeposit(store, payload, []map[uint16]*crypto.Signature{{0: &badSig}}, 0), "invalid custodian signature for deposit")

	store.depositLock = crypto.Blake3Hash([]byte("locked"))
	require.ErrorContains(ver.validateDeposit(store, payload, []map[uint16]*crypto.Signature{{0: &sig}}, 0), "invalid lock")
	store.depositLock = crypto.Hash{}

	require.Nil(ver.validateDeposit(store, payload, []map[uint16]*crypto.Signature{{0: &sig}}, 0))
	ver.SignaturesMap = []map[uint16]*crypto.Signature{{0: &sig}}
	require.Nil(ver.Validate(store, 0, false))
}

func TestSigningAndValidateDispatchCampaign(t *testing.T) {
	require := require.New(t)

	account := deterministicAddress(150)
	utxoHash := crypto.Blake3Hash([]byte("sign-input"))
	utxo, ghost := makeScriptUTXO(account, utxoHash, 0, NewInteger(10))
	store := &campaignStore{
		utxos: map[string]*UTXOWithLock{
			utxoRef(utxoHash, 0): utxo,
		},
	}

	signTx := NewTransactionV5(XINAssetId)
	signTx.AddInput(utxoHash, 0)
	signTx.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), utxo.Amount, bytes.Repeat([]byte{4}, 64))
	signVer := signTx.AsVersioned()
	require.Nil(signVer.SignInput(store, 0, nil))
	require.ErrorContains(signVer.SignInput(store, 1, []*Address{&account}), "invalid input index")

	store.readUTXOErr = errors.New("sign input read failure")
	require.ErrorIs(signVer.SignInput(store, 0, []*Address{&account}), store.readUTXOErr)
	store.readUTXOErr = nil

	delete(store.utxos, utxoRef(utxoHash, 0))
	require.ErrorContains(signVer.SignInput(store, 0, []*Address{&account}), "input not found")
	store.utxos[utxoRef(utxoHash, 0)] = utxo

	wrong := deterministicAddress(151)
	require.ErrorContains(signVer.SignInput(store, 0, []*Address{&wrong}), "invalid key for the input")
	require.Nil(signVer.SignInput(store, 0, []*Address{&account}))
	require.Len(signVer.SignaturesMap, 1)
	require.True(utxo.Keys[0].Verify(signVer.PayloadHash(), *signVer.SignaturesMap[0][0]))
	require.Equal(ghost.Public(), *utxo.Keys[0])

	depositTx := NewTransactionV5(XINAssetId)
	depositTx.AddDepositInput(&DepositData{Chain: BitcoinAssetId, AssetKey: "btc", Transaction: "raw", Amount: NewInteger(1)})
	depositTx.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), NewInteger(1), bytes.Repeat([]byte{5}, 64))
	depositVer := depositTx.AsVersioned()
	require.Nil(depositVer.SignInput(store, 0, []*Address{&account}))
	require.Len(depositVer.SignaturesMap, 1)
	require.Len(depositVer.SignaturesMap[0], 1)

	raw := NewTransactionV5(XINAssetId).AsVersioned()
	require.ErrorContains(raw.SignRaw(account.PrivateSpendKey), "invalid inputs count")
	raw.AddInput(crypto.Blake3Hash([]byte("raw")), 0)
	require.ErrorContains(raw.SignRaw(account.PrivateSpendKey), "invalid input format")

	mintTx := NewTransactionV5(XINAssetId)
	mintTx.AddUniversalMintInput(2, NewInteger(6))
	mintTx.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), NewInteger(6), bytes.Repeat([]byte{6}, 64))
	mintVer := mintTx.AsVersioned()
	require.Nil(mintVer.SignRaw(account.PrivateSpendKey))

	mintStore := &campaignStore{
		mintDist: &MintDistribution{
			MintData:    MintData{Group: mintGroupUniversal, Batch: 1, Amount: NewInteger(1)},
			Transaction: crypto.Blake3Hash([]byte("mint-prev")),
		},
	}
	mintVer.SignaturesMap = []map[uint16]*crypto.Signature{{}}
	require.Nil(mintVer.Validate(mintStore, 0, false))

	withdrawStore := &campaignStore{
		utxos: map[string]*UTXOWithLock{
			utxoRef(utxoHash, 0): utxo,
		},
	}
	withdrawSubmit := NewTransactionV5(XINAssetId)
	withdrawSubmit.AddInput(utxoHash, 0)
	withdrawSubmit.Outputs = append(withdrawSubmit.Outputs, &Output{
		Type:       OutputTypeWithdrawalSubmit,
		Amount:     NewInteger(1),
		Withdrawal: &WithdrawalData{Address: "destination"},
	})
	withdrawSubmit.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), utxo.Amount.Sub(NewInteger(1)), bytes.Repeat([]byte{7}, 64))
	withdrawSubmitVer := withdrawSubmit.AsVersioned()
	submitSig := ghost.Sign(withdrawSubmitVer.PayloadHash())
	withdrawSubmitVer.SignaturesMap = []map[uint16]*crypto.Signature{{0: &submitSig}}
	require.Nil(withdrawSubmitVer.Validate(withdrawStore, 0, false))

	custodian := deterministicAddress(152)
	withdrawClaim := NewTransactionV5(XINAssetId)
	withdrawClaim.AddInput(utxoHash, 0)
	fee := NewIntegerFromString(config.WithdrawalClaimFee)
	withdrawClaim.Outputs = append(withdrawClaim.Outputs, &Output{
		Type:   OutputTypeWithdrawalClaim,
		Amount: fee,
	})
	withdrawClaim.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), utxo.Amount.Sub(fee), bytes.Repeat([]byte{8}, 64))
	withdrawClaim.References = []crypto.Hash{withdrawSubmitVer.PayloadHash()}
	claimData := []byte("withdraw-claim")
	claimHash := crypto.Blake3Hash(claimData)
	claimSig := custodian.PrivateSpendKey.Sign(claimHash)
	withdrawClaim.Extra = append(claimSig[:], claimData...)
	withdrawClaimVer := withdrawClaim.AsVersioned()
	inputSig := ghost.Sign(withdrawClaimVer.PayloadHash())
	withdrawClaimVer.SignaturesMap = []map[uint16]*crypto.Signature{{0: &inputSig}}
	withdrawStore.txs = map[string]*VersionedTransaction{
		withdrawSubmitVer.PayloadHash().String(): withdrawSubmitVer,
	}
	withdrawStore.custodian = &CustodianUpdateRequest{Custodian: &custodian}
	require.Nil(withdrawClaimVer.Validate(withdrawStore, 0, false))

	nodeSigner := deterministicAddress(153)
	nodePayee := deterministicAddress(154)
	nodeStore := &campaignStore{
		utxos: map[string]*UTXOWithLock{
			utxoRef(utxoHash, 0): &UTXOWithLock{UTXO: UTXO{
				Input:  utxo.Input,
				Output: Output{Type: OutputTypeScript, Amount: KernelNodePledgeAmount, Keys: utxo.Keys, Mask: utxo.Mask, Script: NewThresholdScript(1)},
				Asset:  XINAssetId,
			}},
		},
	}
	nodePledge := NewTransactionV5(XINAssetId)
	nodePledge.AddInput(utxoHash, 0)
	nodePledge.Outputs = []*Output{{Type: OutputTypeNodePledge, Amount: KernelNodePledgeAmount}}
	nodePledge.Extra = append(nodeSigner.PublicSpendKey[:], nodePayee.PublicSpendKey[:]...)
	nodePledgeVer := nodePledge.AsVersioned()
	nodePledgeSig := ghost.Sign(nodePledgeVer.PayloadHash())
	nodePledgeVer.SignaturesMap = []map[uint16]*crypto.Signature{{0: &nodePledgeSig}}
	nodeStore.nodes = []*Node{{Signer: deterministicAddress(155), State: NodeStateAccepted}}
	require.Nil(nodePledgeVer.Validate(nodeStore, 0, false))

	nodeAcceptInput := &UTXOWithLock{UTXO: UTXO{
		Input:  Input{Hash: nodePledgeVer.PayloadHash(), Index: 0},
		Output: Output{Type: OutputTypeNodePledge, Amount: KernelNodePledgeAmount},
		Asset:  XINAssetId,
	}}
	acceptStore := &campaignStore{
		utxos: map[string]*UTXOWithLock{
			utxoRef(nodePledgeVer.PayloadHash(), 0): nodeAcceptInput,
		},
		txs: map[string]*VersionedTransaction{
			nodePledgeVer.PayloadHash().String(): nodePledgeVer,
		},
		nodes: []*Node{{
			Signer:      *nodePledge.NodeTransactionExtraAsSigner(),
			State:       NodeStatePledging,
			Transaction: nodePledgeVer.PayloadHash(),
		}},
	}
	nodeAccept := NewTransactionV5(XINAssetId)
	nodeAccept.AddInput(nodePledgeVer.PayloadHash(), 0)
	nodeAccept.Outputs = []*Output{{Type: OutputTypeNodeAccept, Amount: KernelNodePledgeAmount}}
	nodeAccept.Extra = append([]byte{}, nodePledge.Extra...)
	nodeAcceptVer := nodeAccept.AsVersioned()
	require.Nil(nodeAcceptVer.Validate(acceptStore, 0, false))

	nodeRemoveInput := &UTXOWithLock{UTXO: UTXO{
		Input:  Input{Hash: nodeAcceptVer.PayloadHash(), Index: 0},
		Output: Output{Type: OutputTypeNodeAccept, Amount: KernelNodePledgeAmount},
		Asset:  XINAssetId,
	}}
	removeStore := &campaignStore{
		utxos: map[string]*UTXOWithLock{
			utxoRef(nodeAcceptVer.PayloadHash(), 0): nodeRemoveInput,
		},
		txs: map[string]*VersionedTransaction{
			nodeAcceptVer.PayloadHash().String(): nodeAcceptVer,
		},
	}
	nodeRemove := NewTransactionV5(XINAssetId)
	nodeRemove.AddInput(nodeAcceptVer.PayloadHash(), 0)
	nodeRemove.AddOutputWithType(OutputTypeNodeRemove, []*Address{&account}, NewThresholdScript(1), KernelNodePledgeAmount, bytes.Repeat([]byte{10}, 64))
	nodeRemove.Extra = append([]byte{}, nodeAccept.Extra...)
	nodeRemoveVer := nodeRemove.AsVersioned()
	require.Nil(nodeRemoveVer.Validate(removeStore, 0, false))

	cancelStore, cancelTx, cancelHash, cancelSigs := buildNodeCancelScenario()
	cancelScript := *cancelTx.Outputs[1]
	cancelUTXO := &UTXOWithLock{UTXO: UTXO{
		Input: Input{Hash: cancelTx.Inputs[0].Hash, Index: 0},
		Output: Output{
			Type:   OutputTypeScript,
			Amount: NewInteger(100),
			Keys:   cancelScript.Keys,
			Mask:   cancelScript.Mask,
			Script: cancelScript.Script,
		},
		Asset: XINAssetId,
	}}
	cancelStore.utxos = map[string]*UTXOWithLock{
		utxoRef(cancelTx.Inputs[0].Hash, 0): cancelUTXO,
	}
	cancelVer := cancelTx.AsVersioned()
	cancelVer.hash = cancelHash
	cancelVer.SignaturesMap = cancelSigs
	require.Nil(cancelVer.Validate(cancelStore, 0, false))

	slashStore := &campaignStore{
		utxos: map[string]*UTXOWithLock{
			utxoRef(utxoHash, 0): utxo,
		},
	}
	slashTx := NewTransactionV5(XINAssetId)
	slashTx.AddInput(utxoHash, 0)
	slashTx.AddOutputWithType(OutputTypeCustodianSlashNodes, []*Address{&account}, NewThresholdScript(1), utxo.Amount, bytes.Repeat([]byte{9}, 64))
	slashVer := slashTx.AsVersioned()
	slashSig := ghost.Sign(slashVer.PayloadHash())
	slashVer.SignaturesMap = []map[uint16]*crypto.Signature{{0: &slashSig}}
	require.ErrorContains(slashVer.Validate(slashStore, 0, false), "not implemented")
}

func TestGenesisCampaign(t *testing.T) {
	require := require.New(t)

	gns := buildGenesisFixture()
	require.Equal(uint64(1_700_000_000_000_000_000), gns.EpochTimestamp())
	require.Equal(gns.NetworkId(), gns.NetworkId())

	rounds, snapshots, txs, err := gns.BuildSnapshots()
	require.Nil(err)
	require.Len(rounds, len(gns.Nodes)*2)
	require.Len(snapshots, len(gns.Nodes)+1)
	require.Len(txs, len(gns.Nodes)+1)
	require.EqualValues(OutputTypeNodeAccept, txs[0].Outputs[0].Type)
	require.EqualValues(OutputTypeCustodianUpdateNodes, txs[len(txs)-1].Outputs[0].Type)
	require.Equal(uint64(len(gns.Nodes)), snapshots[len(snapshots)-1].TopologicalOrder)
	require.True(rounds[0].Hash.HasValue())
	require.True(rounds[1].References.Self.HasValue())

	path := filepath.Join(t.TempDir(), "genesis.json")
	data, err := json.Marshal(gns)
	require.Nil(err)
	require.Nil(os.WriteFile(path, data, 0o644))

	read, err := ReadGenesis(path)
	require.Nil(err)
	require.Equal(gns.NetworkId(), read.NetworkId())

	invalid := *gns
	invalid.Custodian = nil
	writeGenesis(t, &invalid, path)
	_, err = ReadGenesis(path)
	require.ErrorContains(err, "invalid genesis custodian")

	invalid = *gns
	invalid.Nodes = invalid.Nodes[:config.KernelMinimumNodesCount-1]
	writeGenesis(t, &invalid, path)
	_, err = ReadGenesis(path)
	require.ErrorContains(err, "invalid genesis inputs number")

	invalid = *gns
	dup := *invalid.Nodes[0]
	invalid.Nodes[1] = &dup
	writeGenesis(t, &invalid, path)
	_, err = ReadGenesis(path)
	require.ErrorContains(err, "duplicated genesis node input")

	invalid = *gns
	invalid.Nodes[0] = &struct {
		Signer    *Address `json:"signer"`
		Payee     *Address `json:"payee"`
		Custodian *Address `json:"custodian"`
		Balance   Integer  `json:"balance"`
	}{
		Signer:    invalid.Nodes[0].Signer,
		Payee:     invalid.Nodes[0].Payee,
		Custodian: invalid.Nodes[0].Custodian,
		Balance:   NewInteger(1),
	}
	writeGenesis(t, &invalid, path)
	_, err = ReadGenesis(path)
	require.ErrorContains(err, "invalid genesis node input amount")

	invalid = *gns
	badSigner := *invalid.Nodes[0].Signer
	badSigner.PublicViewKey = deterministicAddress(90).PublicViewKey
	invalid.Nodes[0] = &struct {
		Signer    *Address `json:"signer"`
		Payee     *Address `json:"payee"`
		Custodian *Address `json:"custodian"`
		Balance   Integer  `json:"balance"`
	}{
		Signer:    &badSigner,
		Payee:     invalid.Nodes[0].Payee,
		Custodian: invalid.Nodes[0].Custodian,
		Balance:   KernelNodePledgeAmount,
	}
	writeGenesis(t, &invalid, path)
	_, err = ReadGenesis(path)
	require.ErrorContains(err, "invalid node key format")
}

func TestVersionAndLimitEdgeCampaign(t *testing.T) {
	require := require.New(t)

	require.Panics(func() {
		(&Transaction{Version: 1}).AsVersioned()
	})
	require.Panics(func() {
		(&SignedTransaction{Transaction: Transaction{Version: 1}}).AsVersioned()
	})
	require.Panics(func() {
		(&SignedTransaction{Transaction: Transaction{Version: 1}}).GetExtraLimit()
	})

	account := deterministicAddress(160)

	nonXIN := &SignedTransaction{Transaction: Transaction{Version: TxVersionHashSignature, Asset: BitcoinAssetId}}
	require.Equal(ExtraSizeGeneralLimit, nonXIN.GetExtraLimit())

	noStorage := &SignedTransaction{Transaction: Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{{
			Type:   OutputTypeScript,
			Amount: NewInteger(1),
			Keys:   []*crypto.Key{&account.PublicSpendKey, &account.PublicViewKey},
			Mask:   account.PublicSpendKey,
			Script: NewThresholdScript(1),
		}},
	}}
	require.Equal(ExtraSizeGeneralLimit, noStorage.GetExtraLimit())

	smallStorage := NewTransactionV5(XINAssetId)
	smallStorage.AddOutputWithType(OutputTypeScript, []*Address{&account}, NewThresholdScript(64), NewIntegerFromString("0.00001"), bytes.Repeat([]byte{11}, 64))
	require.Equal(ExtraSizeGeneralLimit, smallStorage.AsVersioned().GetExtraLimit())

	cappedStorage := NewTransactionV5(XINAssetId)
	cappedStorage.AddOutputWithType(OutputTypeScript, []*Address{&account}, NewThresholdScript(64), NewIntegerFromString("1000"), bytes.Repeat([]byte{12}, 64))
	require.Equal(ExtraSizeStorageCapacity, cappedStorage.AsVersioned().GetExtraLimit())

	custodianStorage := NewTransactionV5(XINAssetId)
	custodianStorage.AddOutputWithType(OutputTypeCustodianUpdateNodes, []*Address{&account}, NewThresholdScript(64), NewInteger(1), bytes.Repeat([]byte{13}, 64))
	require.Equal(ExtraSizeStorageCapacity, custodianStorage.AsVersioned().GetExtraLimit())

	require.Zero(checkTxVersion(nil))
	require.Zero(checkTxVersion([]byte{0, 0, 0, 0}))
	require.EqualValues(TxVersionHashSignature, checkTxVersion([]byte{0x77, 0x77, 0x00, TxVersionHashSignature}))
	require.Zero(checkSnapVersion(nil))
	require.Zero(checkSnapVersion([]byte{0, 0, 0, 0}))
	require.EqualValues(SnapshotVersionCommonEncoding, checkSnapVersion([]byte{0x77, 0x77, 0x00, SnapshotVersionCommonEncoding}))

	_, err := UnmarshalVersionedTransaction(make([]byte, config.TransactionMaximumSize+1))
	require.ErrorContains(err, "transaction too large")

	require.ErrorContains((func() error {
		_, err := UnmarshalRound([]byte{1, 2, 3})
		return err
	})(), "invalid round size")
	require.ErrorContains((func() error {
		_, err := UnmarshalUTXO([]byte{1, 2, 3})
		return err
	})(), "invalid UTXO size")

	ver := &VersionedTransaction{SignedTransaction: SignedTransaction{Transaction: Transaction{Version: 1}}}
	require.Panics(func() {
		ver.marshal()
	})
	require.Panics(func() {
		ver.payloadMarshal()
	})
	require.Panics(func() {
		(&SnapshotWithTopologicalOrder{Snapshot: &Snapshot{Version: 1}}).VersionedMarshal()
	})
	require.Panics(func() {
		(&Snapshot{Version: 1}).versionedPayload()
	})

	inputPanic := NewTransactionV5(XINAssetId)
	inputPanic.Inputs = make([]*Input, SliceCountLimit)
	require.Panics(func() {
		inputPanic.AddInput(crypto.Hash{}, 0)
	})

	outputPanic := NewTransactionV5(XINAssetId)
	outputPanic.Outputs = make([]*Output, SliceCountLimit)
	require.Panics(func() {
		outputPanic.AddOutputWithType(OutputTypeScript, []*Address{&account}, NewThresholdScript(1), NewInteger(1), bytes.Repeat([]byte{14}, 64))
	})
}

type campaignStore struct {
	asset        *Asset
	balance      Integer
	utxos        map[string]*UTXOWithLock
	txs          map[string]*VersionedTransaction
	nodes        []*Node
	custodian    *CustodianUpdateRequest
	mintDist     *MintDistribution
	depositLock  crypto.Hash
	ghostErr     error
	readTxErr    error
	readUTXOErr  error
	custodianErr error
	mintErr      error
	assetErr     error
}

func (s *campaignStore) ReadAssetWithBalance(_ crypto.Hash) (*Asset, Integer, error) {
	return s.asset, s.balance, s.assetErr
}

func (s *campaignStore) ReadUTXOKeys(hash crypto.Hash, index uint) (*UTXOKeys, error) {
	utxo, err := s.ReadUTXOLock(hash, index)
	if err != nil || utxo == nil {
		return nil, err
	}
	return &UTXOKeys{Mask: utxo.Mask, Keys: utxo.Keys}, nil
}

func (s *campaignStore) ReadUTXOLock(hash crypto.Hash, index uint) (*UTXOWithLock, error) {
	if s.readUTXOErr != nil {
		return nil, s.readUTXOErr
	}
	return s.utxos[utxoRef(hash, index)], nil
}

func (s *campaignStore) ReadDepositLock(_ *DepositData) (crypto.Hash, error) {
	return s.depositLock, nil
}

func (s *campaignStore) ReadLastMintDistribution(_ uint64) (*MintDistribution, error) {
	return s.mintDist, s.mintErr
}

func (s *campaignStore) LockUTXOs(_ []*Input, _ crypto.Hash, _ bool) error {
	return nil
}

func (s *campaignStore) LockDepositInput(_ *DepositData, _ crypto.Hash, _ bool) error {
	return nil
}

func (s *campaignStore) LockMintInput(_ *MintData, _ crypto.Hash, _ bool) error {
	return nil
}

func (s *campaignStore) LockGhostKeys(_ []*crypto.Key, _ crypto.Hash, _ bool) error {
	return s.ghostErr
}

func (s *campaignStore) ReadAllNodes(_ uint64, _ bool) []*Node {
	return s.nodes
}

func (s *campaignStore) ReadCustodian(_ uint64) (*CustodianUpdateRequest, error) {
	return s.custodian, s.custodianErr
}

func (s *campaignStore) ReadTransaction(hash crypto.Hash) (*VersionedTransaction, string, error) {
	if s.readTxErr != nil {
		return nil, "", s.readTxErr
	}
	tx := s.txs[hash.String()]
	if tx == nil {
		return nil, "", nil
	}
	return tx, hash.String(), nil
}

func deterministicAddress(base byte) Address {
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = base + byte(i)
	}
	return NewAddressFromSeed(seed)
}

func deterministicGenesisAddress(base byte) Address {
	addr := deterministicAddress(base)
	addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
	addr.PublicViewKey = addr.PrivateViewKey.Public()
	return addr
}

func makeScriptUTXO(account Address, hash crypto.Hash, index uint, amount Integer) (*UTXOWithLock, *crypto.Key) {
	seed := bytes.Repeat([]byte{byte(index + 1)}, 64)
	maskKey := crypto.NewKeyFromSeed(seed)
	mask := maskKey.Public()
	key := crypto.DeriveGhostPublicKey(&maskKey, &account.PublicViewKey, &account.PublicSpendKey, uint64(index))
	utxo := &UTXOWithLock{
		UTXO: UTXO{
			Input: Input{
				Hash:  hash,
				Index: index,
			},
			Output: Output{
				Type:   OutputTypeScript,
				Amount: amount,
				Keys:   []*crypto.Key{key},
				Mask:   mask,
				Script: NewThresholdScript(1),
			},
			Asset: XINAssetId,
		},
	}
	priv := crypto.DeriveGhostPrivateKey(&utxo.Mask, &account.PrivateViewKey, &account.PrivateSpendKey, uint64(index))
	return utxo, priv
}

func buildNodeCancelScenario() (*campaignStore, *Transaction, crypto.Hash, []map[uint16]*crypto.Signature) {
	owner := deterministicAddress(70)
	signer := deterministicAddress(71)
	payee := deterministicAddress(72)

	pitHash := crypto.Blake3Hash([]byte("pit"))
	pitUTXO, _ := makeScriptUTXO(owner, pitHash, 1, NewInteger(99))
	pit := &Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Outputs: []*Output{
			{
				Type:   OutputTypeScript,
				Amount: NewInteger(1),
				Keys:   pitUTXO.Keys,
				Mask:   pitUTXO.Mask,
				Script: NewThresholdScript(1),
			},
			{
				Type:   OutputTypeScript,
				Amount: pitUTXO.Amount,
				Keys:   pitUTXO.Keys,
				Mask:   pitUTXO.Mask,
				Script: NewThresholdScript(1),
			},
		},
	}
	pitVer := pit.AsVersioned()

	extra := append(signer.PublicSpendKey[:], payee.PublicSpendKey[:]...)
	lastPledge := &Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: pitVer.PayloadHash(), Index: 1}},
		Outputs: []*Output{{Type: OutputTypeNodePledge, Amount: NewInteger(100)}},
		Extra:   extra,
	}
	lastPledgeVer := lastPledge.AsVersioned()

	cancel := &Transaction{
		Version: TxVersionHashSignature,
		Asset:   XINAssetId,
		Inputs:  []*Input{{Hash: lastPledgeVer.PayloadHash(), Index: 0}},
		Outputs: []*Output{
			{Type: OutputTypeNodeCancel, Amount: NewInteger(1)},
			{
				Type:   OutputTypeScript,
				Amount: NewInteger(99),
				Keys:   pitUTXO.Keys,
				Mask:   pitUTXO.Mask,
				Script: NewThresholdScript(1),
			},
		},
		Extra: append(extra, owner.PrivateViewKey[:]...),
	}
	hash := cancel.AsVersioned().PayloadHash()
	ghost := crypto.DeriveGhostPrivateKey(&pitUTXO.Mask, &owner.PrivateViewKey, &owner.PrivateSpendKey, 1)
	sig := ghost.Sign(hash)

	store := &campaignStore{
		txs: map[string]*VersionedTransaction{
			lastPledgeVer.PayloadHash().String(): lastPledgeVer,
			pitVer.PayloadHash().String():        pitVer,
		},
		nodes: []*Node{{
			Signer:      *lastPledge.NodeTransactionExtraAsSigner(),
			State:       NodeStatePledging,
			Transaction: lastPledgeVer.PayloadHash(),
		}},
	}
	return store, cancel, hash, []map[uint16]*crypto.Signature{{0: &sig}}
}

func buildGenesisFixture() *Genesis {
	nodes := make([]*struct {
		Signer    *Address `json:"signer"`
		Payee     *Address `json:"payee"`
		Custodian *Address `json:"custodian"`
		Balance   Integer  `json:"balance"`
	}, config.KernelMinimumNodesCount)
	for i := range nodes {
		signer := deterministicGenesisAddress(byte(100 + i*3))
		payee := deterministicGenesisAddress(byte(101 + i*3))
		custodian := deterministicGenesisAddress(byte(102 + i*3))
		nodes[i] = &struct {
			Signer    *Address `json:"signer"`
			Payee     *Address `json:"payee"`
			Custodian *Address `json:"custodian"`
			Balance   Integer  `json:"balance"`
		}{
			Signer:    &signer,
			Payee:     &payee,
			Custodian: &custodian,
			Balance:   KernelNodePledgeAmount,
		}
	}

	custodian := deterministicGenesisAddress(130)
	return &Genesis{
		Epoch:     1_700_000_000,
		Nodes:     nodes,
		Custodian: &custodian,
	}
}

func writeGenesis(t *testing.T, gns *Genesis, path string) {
	t.Helper()
	data, err := json.Marshal(gns)
	require.Nil(t, err)
	require.Nil(t, os.WriteFile(path, data, 0o644))
}

func utxoRef(hash crypto.Hash, index uint) string {
	return fmt.Sprintf("%s:%d", hash.String(), index)
}
