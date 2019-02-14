package kernel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	MinimumNodeCount = 7
	PledgeAmount     = 10000
)

type Genesis struct {
	Epoch int64 `json:"epoch"`
	Nodes []struct {
		Address common.Address `json:"address"`
		Balance common.Integer `json:"balance"`
	} `json:"nodes"`
	Domains []struct {
		Address common.Address `json:"address"`
		Balance common.Integer `json:"balance"`
	} `json:"domains"`
}

func (node *Node) LoadGenesis(configDir string) error {
	const stateKeyNetwork = "network"

	gns, err := readGenesis(configDir + "/genesis.json")
	if err != nil {
		return err
	}

	data, err := json.Marshal(gns)
	if err != nil {
		return err
	}
	node.networkId = crypto.NewHash(data)
	node.IdForNetwork = node.Account.Hash().ForNetwork(node.networkId)

	var state struct {
		Id crypto.Hash
	}
	found, err := node.store.StateGet(stateKeyNetwork, &state)
	if err != nil || state.Id == node.networkId {
		return err
	}
	if found {
		return fmt.Errorf("invalid genesis for network %s", state.Id.String())
	}

	var snapshots []*common.SnapshotWithTopologicalOrder
	cacheRounds := make(map[crypto.Hash]*CacheRound)
	for _, in := range gns.Nodes {
		seed := crypto.NewHash([]byte(in.Address.String() + "NODEACCEPT"))
		r := crypto.NewKeyFromSeed(append(seed[:], seed[:]...))
		R := r.Public()
		var keys []crypto.Key
		for _, d := range gns.Nodes {
			key := crypto.DeriveGhostPublicKey(&r, &d.Address.PublicViewKey, &d.Address.PublicSpendKey, 0)
			keys = append(keys, *key)
		}

		tx := common.Transaction{
			Version: common.TxVersion,
			Asset:   common.XINAssetId,
			Inputs: []*common.Input{
				{
					Genesis: node.networkId[:],
				},
			},
			Outputs: []*common.Output{
				{
					Type:   common.OutputTypeNodeAccept,
					Script: common.Script([]uint8{common.OperatorCmp, common.OperatorSum, uint8(len(gns.Nodes)*2/3 + 1)}),
					Amount: common.NewInteger(PledgeAmount),
					Keys:   keys,
					Mask:   R,
				},
			},
		}
		tx.Extra = make([]byte, len(in.Address.PublicSpendKey))
		copy(tx.Extra, in.Address.PublicSpendKey[:])

		signed := &common.SignedTransaction{Transaction: tx}
		nodeId := in.Address.Hash().ForNetwork(node.networkId)
		snapshot := common.Snapshot{
			NodeId:      nodeId,
			Transaction: signed,
			RoundNumber: 0,
			Timestamp:   uint64(time.Unix(gns.Epoch, 0).UnixNano()),
		}
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         snapshot,
			TopologicalOrder: node.TopoCounter.Next(),
		}
		snapshots = append(snapshots, topo)
		cacheRounds[snapshot.NodeId] = &CacheRound{
			NodeId:    snapshot.NodeId,
			Number:    0,
			Snapshots: []*common.Snapshot{&snapshot},
		}
	}

	domain := gns.Domains[0]
	if in := gns.Nodes[0]; domain.Address.String() != in.Address.String() {
		return fmt.Errorf("invalid genesis domain input account %s %s", domain.Address.String(), in.Address.String())
	}
	topo := node.buildDomainSnapshot(domain.Address, gns)
	snapshots = append(snapshots, topo)
	cacheRounds[topo.NodeId].Snapshots = append(cacheRounds[topo.NodeId].Snapshots, &topo.Snapshot)

	rounds := make([]*common.Round, 0)
	for i, in := range gns.Nodes {
		id := in.Address.Hash().ForNetwork(node.networkId)
		external := gns.Nodes[0].Address.Hash().ForNetwork(node.networkId)
		if i != len(gns.Nodes)-1 {
			external = gns.Nodes[i+1].Address.Hash().ForNetwork(node.networkId)
		}
		selfFinal := cacheRounds[id].asFinal()
		externalFinal := cacheRounds[external].asFinal()
		rounds = append(rounds, &common.Round{
			Hash:      selfFinal.Hash,
			NodeId:    selfFinal.NodeId,
			Number:    selfFinal.Number,
			Timestamp: selfFinal.Start,
		})
		rounds = append(rounds, &common.Round{
			Hash:       selfFinal.NodeId,
			NodeId:     selfFinal.NodeId,
			Number:     selfFinal.Number + 1,
			References: [2]crypto.Hash{selfFinal.Hash, externalFinal.Hash},
		})
	}

	err = node.store.LoadGenesis(rounds, snapshots)
	if err != nil {
		return err
	}

	state.Id = node.networkId
	return node.store.StateSet(stateKeyNetwork, state)
}

func (node *Node) buildDomainSnapshot(domain common.Address, gns *Genesis) *common.SnapshotWithTopologicalOrder {
	seed := crypto.NewHash([]byte(domain.String() + "DOMAINACCEPT"))
	r := crypto.NewKeyFromSeed(append(seed[:], seed[:]...))
	R := r.Public()
	keys := make([]crypto.Key, 0)
	for _, d := range gns.Nodes {
		key := crypto.DeriveGhostPublicKey(&r, &d.Address.PublicViewKey, &d.Address.PublicSpendKey, 0)
		keys = append(keys, *key)
	}

	tx := common.Transaction{
		Version: common.TxVersion,
		Asset:   common.XINAssetId,
		Inputs: []*common.Input{
			{
				Genesis: node.networkId[:],
			},
		},
		Outputs: []*common.Output{
			{
				Type:   common.OutputTypeDomainAccept,
				Script: common.Script([]uint8{common.OperatorCmp, common.OperatorSum, uint8(len(gns.Nodes)*2/3 + 1)}),
				Amount: common.NewInteger(50000),
				Keys:   keys,
				Mask:   R,
			},
		},
	}
	tx.Extra = make([]byte, len(domain.PublicSpendKey))
	copy(tx.Extra, domain.PublicSpendKey[:])

	signed := &common.SignedTransaction{Transaction: tx}
	nodeId := domain.Hash().ForNetwork(node.networkId)
	snapshot := common.Snapshot{
		NodeId:      nodeId,
		Transaction: signed,
		RoundNumber: 0,
		Timestamp:   uint64(time.Unix(gns.Epoch, 0).UnixNano() + 1),
	}
	return &common.SnapshotWithTopologicalOrder{
		Snapshot:         snapshot,
		TopologicalOrder: node.TopoCounter.Next(),
	}
}

func readGenesis(path string) (*Genesis, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var gns Genesis
	err = json.Unmarshal(f, &gns)
	if err != nil {
		return nil, err
	}
	if len(gns.Nodes) != MinimumNodeCount {
		return nil, fmt.Errorf("invalid genesis inputs number %d/%d", len(gns.Nodes), MinimumNodeCount)
	}

	inputsFilter := make(map[string]bool)
	for _, in := range gns.Nodes {
		_, err := common.NewAddressFromString(in.Address.String())
		if err != nil {
			return nil, err
		}
		if in.Balance.Cmp(common.NewInteger(PledgeAmount)) != 0 {
			return nil, fmt.Errorf("invalid genesis node input amount %s", in.Balance.String())
		}
		if inputsFilter[in.Address.String()] {
			return nil, fmt.Errorf("duplicated genesis node input %s", in.Address.String())
		}
		privateView := in.Address.PublicSpendKey.DeterministicHashDerive()
		if privateView.Public() != in.Address.PublicViewKey {
			return nil, fmt.Errorf("invalid node key format %s %s", privateView.Public().String(), in.Address.PublicViewKey.String())
		}
	}

	if len(gns.Domains) != 1 {
		return nil, fmt.Errorf("invalid genesis domain inputs count %d", len(gns.Domains))
	}
	domain := gns.Domains[0]
	if domain.Address.String() != gns.Nodes[0].Address.String() {
		return nil, fmt.Errorf("invalid genesis domain input account %s %s", domain.Address.String(), gns.Nodes[0].Address.String())
	}
	if domain.Balance.Cmp(common.NewInteger(50000)) != 0 {
		return nil, fmt.Errorf("invalid genesis domain input amount %s", domain.Balance.String())
	}
	return &gns, nil
}
