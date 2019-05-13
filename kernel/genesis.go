package kernel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	MinimumNodeCount = 7
	PledgeAmount     = 10000
)

type Genesis struct {
	Epoch int64 `json:"epoch"`
	Nodes []struct {
		Signer  common.Address `json:"signer"`
		Payee   common.Address `json:"payee"`
		Balance common.Integer `json:"balance"`
	} `json:"nodes"`
	Domains []struct {
		Signer  common.Address `json:"signer"`
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
	node.epoch = uint64(time.Unix(gns.Epoch, 0).UnixNano())
	node.networkId = crypto.NewHash(data)
	node.IdForNetwork = node.Signer.Hash().ForNetwork(node.networkId)
	for _, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(node.networkId)
		node.genesisNodes[id] = true
	}

	var state struct {
		Id crypto.Hash
	}
	found, err := node.store.StateGet(stateKeyNetwork, &state)
	if err != nil {
		return err
	}
	if found && state.Id != node.networkId {
		return fmt.Errorf("invalid genesis for network %s", state.Id.String())
	}
	loaded, err := node.store.CheckGenesisLoad()
	if err != nil || loaded {
		return err
	}

	var snapshots []*common.SnapshotWithTopologicalOrder
	var transactions []*common.VersionedTransaction
	cacheRounds := make(map[crypto.Hash]*CacheRound)
	for _, in := range gns.Nodes {
		si := crypto.NewHash([]byte(in.Signer.String() + "NODEACCEPT"))
		seed := append(si[:], si[:]...)
		script := common.NewThresholdScript(uint8(len(gns.Nodes)*2/3 + 1))
		accounts := []common.Address{}
		for _, d := range gns.Nodes {
			accounts = append(accounts, d.Signer)
		}

		tx := common.NewTransaction(common.XINAssetId)
		tx.Inputs = []*common.Input{{Genesis: node.networkId[:]}}
		tx.AddOutputWithType(common.OutputTypeNodeAccept, accounts, script, common.NewInteger(PledgeAmount), seed)
		tx.Extra = append(in.Signer.PublicSpendKey[:], in.Payee.PublicSpendKey[:]...)

		signed := tx.AsLatestVersion()
		if node.networkId.String() == config.MainnetId {
			signed, _ = common.UnmarshalVersionedTransaction(signed.Marshal())
		}
		nodeId := in.Signer.Hash().ForNetwork(node.networkId)
		snapshot := common.Snapshot{
			NodeId:      nodeId,
			Transaction: signed.PayloadHash(),
			RoundNumber: 0,
			Timestamp:   node.epoch,
		}
		snapshot.Hash = snapshot.PayloadHash()
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         snapshot,
			TopologicalOrder: node.TopoCounter.Next(),
		}
		snapshots = append(snapshots, topo)
		transactions = append(transactions, signed)
		cacheRounds[snapshot.NodeId] = &CacheRound{
			NodeId:    snapshot.NodeId,
			Number:    0,
			Snapshots: []*common.Snapshot{&snapshot},
		}
	}

	domain := gns.Domains[0]
	if in := gns.Nodes[0]; domain.Signer.String() != in.Signer.String() {
		return fmt.Errorf("invalid genesis domain input account %s %s", domain.Signer.String(), in.Signer.String())
	}
	topo, signed := node.buildDomainSnapshot(domain.Signer, gns)
	snapshots = append(snapshots, topo)
	transactions = append(transactions, signed)
	snap := &topo.Snapshot
	snap.Hash = snap.PayloadHash()
	cacheRounds[topo.NodeId].Snapshots = append(cacheRounds[topo.NodeId].Snapshots, snap)

	rounds := make([]*common.Round, 0)
	for i, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(node.networkId)
		external := gns.Nodes[0].Signer.Hash().ForNetwork(node.networkId)
		if i != len(gns.Nodes)-1 {
			external = gns.Nodes[i+1].Signer.Hash().ForNetwork(node.networkId)
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
			Hash:   selfFinal.NodeId,
			NodeId: selfFinal.NodeId,
			Number: selfFinal.Number + 1,
			References: &common.RoundLink{
				Self:     selfFinal.Hash,
				External: externalFinal.Hash,
			},
		})
	}

	err = node.store.LoadGenesis(rounds, snapshots, transactions)
	if err != nil {
		return err
	}

	state.Id = node.networkId
	return node.store.StateSet(stateKeyNetwork, state)
}

func (node *Node) buildDomainSnapshot(domain common.Address, gns *Genesis) (*common.SnapshotWithTopologicalOrder, *common.VersionedTransaction) {
	si := crypto.NewHash([]byte(domain.String() + "DOMAINACCEPT"))
	seed := append(si[:], si[:]...)
	script := common.NewThresholdScript(uint8(len(gns.Nodes)*2/3 + 1))
	accounts := []common.Address{}
	for _, d := range gns.Nodes {
		accounts = append(accounts, d.Signer)
	}
	tx := common.NewTransaction(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: node.networkId[:]}}
	tx.AddOutputWithType(common.OutputTypeDomainAccept, accounts, script, common.NewInteger(50000), seed)
	tx.Extra = make([]byte, len(domain.PublicSpendKey))
	copy(tx.Extra, domain.PublicSpendKey[:])

	signed := tx.AsLatestVersion()
	if node.networkId.String() == config.MainnetId {
		signed, _ = common.UnmarshalVersionedTransaction(signed.Marshal())
	}
	nodeId := domain.Hash().ForNetwork(node.networkId)
	snapshot := common.Snapshot{
		NodeId:      nodeId,
		Transaction: signed.PayloadHash(),
		RoundNumber: 0,
		Timestamp:   node.epoch + 1,
	}
	return &common.SnapshotWithTopologicalOrder{
		Snapshot:         snapshot,
		TopologicalOrder: node.TopoCounter.Next(),
	}, signed
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
	if len(gns.Nodes) < MinimumNodeCount {
		return nil, fmt.Errorf("invalid genesis inputs number %d/%d", len(gns.Nodes), MinimumNodeCount)
	}

	inputsFilter := make(map[string]bool)
	for _, in := range gns.Nodes {
		_, err := common.NewAddressFromString(in.Signer.String())
		if err != nil {
			return nil, err
		}
		if in.Balance.Cmp(common.NewInteger(PledgeAmount)) != 0 {
			return nil, fmt.Errorf("invalid genesis node input amount %s", in.Balance.String())
		}
		if inputsFilter[in.Signer.String()] {
			return nil, fmt.Errorf("duplicated genesis node input %s", in.Signer.String())
		}
		privateView := in.Signer.PublicSpendKey.DeterministicHashDerive()
		if privateView.Public() != in.Signer.PublicViewKey {
			return nil, fmt.Errorf("invalid node key format %s %s", privateView.Public().String(), in.Signer.PublicViewKey.String())
		}
		privateView = in.Payee.PublicSpendKey.DeterministicHashDerive()
		if privateView.Public() != in.Payee.PublicViewKey {
			return nil, fmt.Errorf("invalid node key format %s %s", privateView.Public().String(), in.Payee.PublicViewKey.String())
		}
	}

	if len(gns.Domains) != 1 {
		return nil, fmt.Errorf("invalid genesis domain inputs count %d", len(gns.Domains))
	}
	domain := gns.Domains[0]
	if domain.Signer.String() != gns.Nodes[0].Signer.String() {
		return nil, fmt.Errorf("invalid genesis domain input account %s %s", domain.Signer.String(), gns.Nodes[0].Signer.String())
	}
	if domain.Balance.Cmp(common.NewInteger(50000)) != 0 {
		return nil, fmt.Errorf("invalid genesis domain input amount %s", domain.Balance.String())
	}
	return &gns, nil
}
