package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

var (
	KernelNodePledgeAmount = NewInteger(13439)
)

type Genesis struct {
	Epoch int64 `json:"epoch"`
	Nodes []*struct {
		Signer    *Address `json:"signer"`
		Payee     *Address `json:"payee"`
		Custodian *Address `json:"custodian"`
		Balance   Integer  `json:"balance"`
	} `json:"nodes"`
	Custodian *Address `json:"custodian"`
}

func (gns *Genesis) EpochTimestamp() uint64 {
	return uint64(time.Unix(gns.Epoch, 0).UnixNano())
}

func (gns *Genesis) NetworkId() crypto.Hash {
	data, err := json.Marshal(gns)
	if err != nil {
		panic(err)
	}
	return crypto.Blake3Hash(data)
}

func (gns *Genesis) BuildSnapshots() ([]*Round, []*SnapshotWithTopologicalOrder, []*VersionedTransaction, error) {
	var snapshots []*SnapshotWithTopologicalOrder
	var transactions []*VersionedTransaction
	cacheRounds := make(map[crypto.Hash]*cacheRound)
	networkId, epoch := gns.NetworkId(), gns.EpochTimestamp()
	for i, in := range gns.Nodes {
		si := crypto.Blake3Hash([]byte(in.Signer.String() + "NODEACCEPT"))
		seed := append(si[:], si[:]...)
		script := NewThresholdScript(uint8(len(gns.Nodes)*2/3 + 1))
		accounts := []*Address{}
		for _, d := range gns.Nodes {
			accounts = append(accounts, d.Signer)
		}

		tx := NewTransactionV5(XINAssetId)
		tx.Inputs = []*Input{{Genesis: networkId[:]}}
		tx.AddOutputWithType(OutputTypeNodeAccept, accounts, script, KernelNodePledgeAmount, seed)
		tx.Extra = append(in.Signer.PublicSpendKey[:], in.Payee.PublicSpendKey[:]...)

		nodeId := in.Signer.Hash().ForNetwork(networkId)
		snapshot := &Snapshot{
			Version:     SnapshotVersionCommonEncoding,
			NodeId:      nodeId,
			RoundNumber: 0,
			Timestamp:   epoch,
		}
		signed := tx.AsVersioned()
		snapshot.AddTransaction(signed.PayloadHash())
		snapshot.Hash = snapshot.PayloadHash()
		topo := &SnapshotWithTopologicalOrder{
			Snapshot:         snapshot,
			TopologicalOrder: uint64(i),
		}
		snapshots = append(snapshots, topo)
		transactions = append(transactions, signed)
		cacheRounds[snapshot.NodeId] = &cacheRound{
			NodeId:    snapshot.NodeId,
			Number:    0,
			Snapshots: []*Snapshot{snapshot},
		}
	}

	topo, signed := buildCustodianSnapshot(networkId, epoch, gns)
	snapshots = append(snapshots, topo)
	transactions = append(transactions, signed)
	snap := topo.Snapshot
	snap.Hash = snap.PayloadHash()
	cacheRounds[topo.NodeId].Snapshots = append(cacheRounds[topo.NodeId].Snapshots, snap)

	rounds := make([]*Round, 0)
	for i, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(networkId)
		external := gns.Nodes[0].Signer.Hash().ForNetwork(networkId)
		if i != len(gns.Nodes)-1 {
			external = gns.Nodes[i+1].Signer.Hash().ForNetwork(networkId)
		}
		selfFinal := cacheRounds[id].asFinal()
		externalFinal := cacheRounds[external].asFinal()
		rounds = append(rounds, &Round{
			Hash:      selfFinal.Hash,
			NodeId:    selfFinal.NodeId,
			Number:    selfFinal.Number,
			Timestamp: selfFinal.Start,
		})
		rounds = append(rounds, &Round{
			Hash:   selfFinal.NodeId,
			NodeId: selfFinal.NodeId,
			Number: selfFinal.Number + 1,
			References: &RoundLink{
				Self:     selfFinal.Hash,
				External: externalFinal.Hash,
			},
		})
	}

	return rounds, snapshots, transactions, nil
}

func buildCustodianSnapshot(networkId crypto.Hash, epoch uint64, gns *Genesis) (*SnapshotWithTopologicalOrder, *VersionedTransaction) {
	tx := NewTransactionV5(XINAssetId)
	si := crypto.Blake3Hash([]byte(gns.Custodian.String() + "CUSTODIANUPDATENODES"))
	seed := append(si[:], si[:]...)
	addr := NewAddressFromSeed(make([]byte, 64))
	script := NewThresholdScript(64)
	accounts := []*Address{&addr}
	amount := NewInteger(100).Mul(len(gns.Nodes))
	tx.Inputs = []*Input{{Genesis: networkId[:]}}
	tx.AddOutputWithType(OutputTypeCustodianUpdateNodes, accounts, script, amount, seed)

	tx.Extra = append(tx.Extra, gns.Custodian.PublicSpendKey[:]...)
	tx.Extra = append(tx.Extra, gns.Custodian.PublicViewKey[:]...)
	nodes := make([]*CustodianNode, len(gns.Nodes))
	spend := gns.Custodian.PublicSpendKey.DeterministicHashDerive()
	for i, node := range gns.Nodes {
		extra := encodeGenesisCustodianNode(node.Custodian, node.Payee, node.Signer, &spend, networkId)
		nodes[i] = &CustodianNode{
			Custodian: *node.Custodian,
			Payee:     *node.Payee,
			Extra:     extra,
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i].Custodian.PublicSpendKey[:], nodes[j].Custodian.PublicSpendKey[:]) < 0
	})
	for _, n := range nodes {
		tx.Extra = append(tx.Extra, n.Extra...)
	}
	eh := crypto.Blake3Hash(tx.Extra)
	sig := spend.Sign(eh)
	tx.Extra = append(tx.Extra, sig[:]...)

	snapshot := &Snapshot{
		Version:     SnapshotVersionCommonEncoding,
		NodeId:      gns.Nodes[0].Signer.Hash().ForNetwork(networkId),
		RoundNumber: 0,
		Timestamp:   epoch + 1,
	}
	signed := tx.AsVersioned()
	snapshot.AddTransaction(signed.PayloadHash())
	return &SnapshotWithTopologicalOrder{
		Snapshot:         snapshot,
		TopologicalOrder: uint64(len(gns.Nodes)),
	}, signed
}

func ReadGenesis(path string) (*Genesis, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var gns Genesis
	err = json.Unmarshal(f, &gns)
	if err != nil {
		return nil, err
	}
	if gns.Custodian == nil {
		return nil, fmt.Errorf("invalid genesis custodian %v", gns)
	}
	if len(gns.Nodes) < config.KernelMinimumNodesCount {
		return nil, fmt.Errorf("invalid genesis inputs number %d/%d", len(gns.Nodes), config.KernelMinimumNodesCount)
	}

	inputsFilter := make(map[string]bool)
	for _, in := range gns.Nodes {
		if in.Signer == nil || in.Payee == nil || in.Custodian == nil {
			return nil, fmt.Errorf("invalid genesis node keys %v", *in)
		}
		_, err := NewAddressFromString(in.Signer.String())
		if err != nil {
			return nil, err
		}
		if in.Balance.Cmp(KernelNodePledgeAmount) != 0 {
			return nil, fmt.Errorf("invalid genesis node input amount %s", in.Balance.String())
		}
		if inputsFilter[in.Signer.String()] || inputsFilter[in.Payee.String()] || inputsFilter[in.Custodian.String()] {
			return nil, fmt.Errorf("duplicated genesis node input %v", in)
		}
		inputsFilter[in.Signer.String()] = true
		inputsFilter[in.Payee.String()] = true
		inputsFilter[in.Custodian.String()] = true
		privateView := in.Signer.PublicSpendKey.DeterministicHashDerive()
		if privateView.Public() != in.Signer.PublicViewKey {
			return nil, fmt.Errorf("invalid node key format %s %s",
				privateView.Public().String(), in.Signer.PublicViewKey.String())
		}
		privateView = in.Payee.PublicSpendKey.DeterministicHashDerive()
		if privateView.Public() != in.Payee.PublicViewKey {
			return nil, fmt.Errorf("invalid node key format %s %s",
				privateView.Public().String(), in.Payee.PublicViewKey.String())
		}
	}

	return &gns, nil
}

func encodeGenesisCustodianNode(custodian, payee, signer *Address, spend *crypto.Key, networkId crypto.Hash) []byte {
	nodeId := signer.Hash().ForNetwork(networkId)

	extra := []byte{1}
	extra = append(extra, custodian.PublicSpendKey[:]...)
	extra = append(extra, custodian.PublicViewKey[:]...)
	extra = append(extra, payee.PublicSpendKey[:]...)
	extra = append(extra, payee.PublicViewKey[:]...)
	extra = append(extra, nodeId[:]...)

	eh := crypto.Blake3Hash(extra)
	signerSig := spend.Sign(eh)
	payeeSig := spend.Sign(eh)
	custodianSig := spend.Sign(eh)
	extra = append(extra, signerSig[:]...)
	extra = append(extra, payeeSig[:]...)
	extra = append(extra, custodianSig[:]...)
	return extra
}

type cacheRound struct {
	NodeId     crypto.Hash
	Number     uint64
	Timestamp  uint64
	References *RoundLink
	Snapshots  []*Snapshot
}

type finalRound struct {
	NodeId crypto.Hash
	Number uint64
	Start  uint64
	End    uint64
	Hash   crypto.Hash
}

func (c *cacheRound) asFinal() *finalRound {
	if len(c.Snapshots) == 0 {
		return nil
	}

	start, end, hash := ComputeRoundHash(c.NodeId, c.Number, c.Snapshots)
	round := &finalRound{
		NodeId: c.NodeId,
		Number: c.Number,
		Start:  start,
		End:    end,
		Hash:   hash,
	}
	return round
}
