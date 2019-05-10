package common

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	NodeStatePledging  = "PLEDGING"
	NodeStateAccepted  = "ACCEPTED"
	NodeStateDeparting = "DEPARTING"
)

type Node struct {
	Signer      Address
	Payee       Address
	State       string
	Transaction crypto.Hash
	Timestamp   uint64
}

func (tx *Transaction) validateNodePledge(store DataStore, inputs map[string]*UTXO) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for pledge transaction", len(tx.Outputs))
	}
	if len(tx.Extra) != 2*len(crypto.Key{}) {
		return fmt.Errorf("invalid extra length %d for pledge transaction", len(tx.Extra))
	}
	for _, in := range inputs {
		if in.Type != OutputTypeScript {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}

	o := tx.Outputs[0]
	if o.Amount.Cmp(NewInteger(10000)) != 0 {
		return fmt.Errorf("invalid pledge amount %s", o.Amount.String())
	}
	for _, n := range store.ReadConsensusNodes() {
		if n.State != NodeStateAccepted {
			return fmt.Errorf("invalid node pending state %s %s", n.Signer.String(), n.State)
		}
	}

	return nil
}

func (tx *Transaction) validateNodeAccept(store DataStore) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for accept transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for accept transaction", len(tx.Inputs))
	}
	var pledging *Node
	filter := make(map[string]string)
	nodes := store.ReadConsensusNodes()
	for _, n := range nodes {
		filter[n.Signer.String()] = n.State
		if n.State == NodeStateDeparting {
			return fmt.Errorf("invalid node pending state %s %s", n.Signer.String(), n.State)
		}
		if n.State == NodeStateAccepted {
			continue
		}
		if n.State == NodeStatePledging && pledging == nil {
			pledging = n
		} else {
			return fmt.Errorf("invalid pledging nodes %s %s", pledging.Signer.String(), n.Signer.String())
		}
	}
	if pledging == nil {
		return fmt.Errorf("no pledging node needs to get accepted")
	}
	if pledging.Transaction != tx.Inputs[0].Hash {
		return fmt.Errorf("invalid plede utxo source %s %s", pledging.Transaction, tx.Inputs[0].Hash)
	}

	lastPledge, err := store.ReadTransaction(tx.Inputs[0].Hash)
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
	if bytes.Compare(lastPledge.Extra, tx.Extra) != 0 {
		return fmt.Errorf("invalid pledge and accpet key %s %s", hex.EncodeToString(lastPledge.Extra), hex.EncodeToString(tx.Extra))
	}
	return nil
}
