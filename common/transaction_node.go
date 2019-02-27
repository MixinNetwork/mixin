package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

func (tx *Transaction) validateNodePledge(store DataStore) error {
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for pledge transaction", len(tx.Outputs))
	}
	if len(tx.Extra) != 2*len(crypto.Key{}) {
		return fmt.Errorf("invalid extra length %d for pledge transaction", len(tx.Extra))
	}

	o := tx.Outputs[0]
	if o.Amount.Cmp(NewInteger(10000)) != 0 {
		return fmt.Errorf("invalid pledge amount %s", o.Amount.String())
	}
	nodes := store.ReadConsensusNodes()
	for _, n := range nodes {
		if n.State != NodeStateAccepted {
			return fmt.Errorf("invalid node pending state %s %s", n.Signer.String(), n.State)
		}
	}

	var signer, payee Address
	copy(signer.PublicSpendKey[:], tx.Extra[:len(signer.PublicSpendKey)])
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicSpendKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicSpendKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()
	nodes = append(nodes, &Node{Signer: signer, Payee: payee})
	if len(nodes) != len(o.Keys) {
		return fmt.Errorf("invalid output keys count %d %d for pledge transaction", len(nodes), len(o.Keys))
	}

	if o.Script.VerifyFormat() != nil || int(o.Script[2]) != len(nodes)*2/3+1 {
		return fmt.Errorf("invalid output script %s %d", o.Script, len(nodes)*2/3+1)
	}

	filter := make(map[crypto.Key]bool)
	for _, n := range nodes {
		filter[n.Signer.PublicSpendKey] = true
	}
	for i, k := range o.Keys {
		for _, n := range nodes {
			ghost := crypto.ViewGhostOutputKey(&k, &n.Signer.PrivateViewKey, &o.Mask, 0)
			delete(filter, *ghost)
		}
		if len(filter) != len(nodes)-1-i {
			return fmt.Errorf("invalid output keys signatures %d", len(filter))
		}
	}
	return nil
}

func (tx *Transaction) validateNodeAccept(store DataStore, inputAmount Integer) error {
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for accept transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 2 {
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
	nodesAmount := NewInteger(uint64(10000 * len(nodes)))
	if inputAmount.Cmp(nodesAmount) != 0 {
		return fmt.Errorf("invalid accept input amount %s %s", inputAmount.String(), nodesAmount.String())
	}

	lastAccept, err := store.ReadTransaction(tx.Inputs[0].Hash)
	if err != nil {
		return err
	}
	ao := lastAccept.Outputs[0]
	if len(lastAccept.Outputs) != 1 {
		return fmt.Errorf("invalid accept utxo count %d", len(lastAccept.Outputs))
	}
	if ao.Type != OutputTypeNodeAccept {
		return fmt.Errorf("invalid accept utxo type %d", ao.Type)
	}
	var publicSpend crypto.Key
	copy(publicSpend[:], lastAccept.Extra)
	privateView := publicSpend.DeterministicHashDerive()
	acc := Address{
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
	if filter[acc.String()] != NodeStateAccepted {
		return fmt.Errorf("invalid accept utxo source %s", filter[acc.String()])
	}

	lastPledge, err := store.ReadTransaction(tx.Inputs[1].Hash)
	if err != nil {
		return err
	}
	po := lastPledge.Outputs[0]
	if len(lastPledge.Outputs) != 1 {
		return fmt.Errorf("invalid pledge utxo count %d", len(lastPledge.Outputs))
	}
	if po.Type != OutputTypeNodePledge {
		return fmt.Errorf("invalid pledge utxo type %d", po.Type)
	}
	copy(publicSpend[:], lastPledge.Extra)
	privateView = publicSpend.DeterministicHashDerive()
	acc = Address{
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
	if filter[acc.String()] != NodeStatePledging {
		return fmt.Errorf("invalid pledge utxo source %s", filter[acc.String()])
	}
	return nil
}
