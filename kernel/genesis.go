package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	MinimumNodeCount = 7
)

type Genesis struct {
	Epoch int64 `json:"epoch"`
	Nodes []*struct {
		Signer  common.Address `json:"signer"`
		Payee   common.Address `json:"payee"`
		Balance common.Integer `json:"balance"`
	} `json:"nodes"`
	Domains []*struct {
		Signer  common.Address `json:"signer"`
		Balance common.Integer `json:"balance"`
	} `json:"domains"`
}

func (node *Node) LoadGenesis(configDir string) error {
	gns, err := readGenesis(configDir + "/genesis.json")
	if err != nil {
		return err
	}

	data, err := json.Marshal(gns)
	if err != nil {
		return err
	}
	node.Epoch = uint64(time.Unix(gns.Epoch, 0).UnixNano())
	node.networkId = crypto.NewHash(data)
	node.IdForNetwork = node.Signer.Hash().ForNetwork(node.networkId)
	for _, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(node.networkId)
		node.genesisNodesMap[id] = true
		node.genesisNodes = append(node.genesisNodes, id)
	}

	rounds, snapshots, transactions, err := buildGenesisSnapshots(node.networkId, node.Epoch, gns)
	if err != nil {
		return err
	}

	loaded, err := node.persistStore.CheckGenesisLoad(snapshots)
	if err != nil || loaded {
		return err
	}

	return node.persistStore.LoadGenesis(rounds, snapshots, transactions)
}

func buildGenesisSnapshots(networkId crypto.Hash, epoch uint64, gns *Genesis) ([]*common.Round, []*common.SnapshotWithTopologicalOrder, []*common.VersionedTransaction, error) {
	var snapshots []*common.SnapshotWithTopologicalOrder
	var transactions []*common.VersionedTransaction
	cacheRounds := make(map[crypto.Hash]*CacheRound)
	for i, in := range gns.Nodes {
		si := crypto.NewHash([]byte(in.Signer.String() + "NODEACCEPT"))
		seed := append(si[:], si[:]...)
		script := common.NewThresholdScript(uint8(len(gns.Nodes)*2/3 + 1))
		accounts := []*common.Address{}
		for _, d := range gns.Nodes {
			accounts = append(accounts, &d.Signer)
		}

		tx := common.NewTransaction(common.XINAssetId)
		tx.Inputs = []*common.Input{{Genesis: networkId[:]}}
		tx.AddOutputWithType(common.OutputTypeNodeAccept, accounts, script, pledgeAmount(0), seed)
		tx.Extra = append(in.Signer.PublicSpendKey[:], in.Payee.PublicSpendKey[:]...)

		nodeId := in.Signer.Hash().ForNetwork(networkId)
		snapshot := common.Snapshot{
			Version:     common.SnapshotVersion,
			NodeId:      nodeId,
			RoundNumber: 0,
			Timestamp:   epoch,
		}
		signed := tx.AsLatestVersion()
		if networkId.String() == config.MainnetId {
			snapshot.Version = 0
			signed.Version = 1
			signed, _ = common.UnmarshalVersionedTransaction(signed.Marshal())
		}
		snapshot.Transaction = signed.PayloadHash()
		snapshot.Hash = snapshot.PayloadHash()
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         snapshot,
			TopologicalOrder: uint64(i),
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
		err := fmt.Errorf("invalid genesis domain input account %s %s", domain.Signer.String(), in.Signer.String())
		return nil, nil, nil, err
	}
	topo, signed := buildDomainSnapshot(networkId, epoch, domain.Signer, gns)
	snapshots = append(snapshots, topo)
	transactions = append(transactions, signed)
	snap := &topo.Snapshot
	snap.Hash = snap.PayloadHash()
	cacheRounds[topo.NodeId].Snapshots = append(cacheRounds[topo.NodeId].Snapshots, snap)

	rounds := make([]*common.Round, 0)
	for i, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(networkId)
		external := gns.Nodes[0].Signer.Hash().ForNetwork(networkId)
		if i != len(gns.Nodes)-1 {
			external = gns.Nodes[i+1].Signer.Hash().ForNetwork(networkId)
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

	return rounds, snapshots, transactions, nil
}

func buildDomainSnapshot(networkId crypto.Hash, epoch uint64, domain common.Address, gns *Genesis) (*common.SnapshotWithTopologicalOrder, *common.VersionedTransaction) {
	si := crypto.NewHash([]byte(domain.String() + "DOMAINACCEPT"))
	seed := append(si[:], si[:]...)
	script := common.NewThresholdScript(uint8(len(gns.Nodes)*2/3 + 1))
	accounts := []*common.Address{}
	for _, d := range gns.Nodes {
		accounts = append(accounts, &d.Signer)
	}
	tx := common.NewTransaction(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: networkId[:]}}
	tx.AddOutputWithType(common.OutputTypeDomainAccept, accounts, script, common.NewInteger(50000), seed)
	tx.Extra = make([]byte, len(domain.PublicSpendKey))
	copy(tx.Extra, domain.PublicSpendKey[:])

	signed := tx.AsLatestVersion()
	if networkId.String() == config.MainnetId {
		signed.Version = 1
		signed, _ = common.UnmarshalVersionedTransaction(signed.Marshal())
	}
	nodeId := domain.Hash().ForNetwork(networkId)
	snapshot := common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      nodeId,
		Transaction: signed.PayloadHash(),
		RoundNumber: 0,
		Timestamp:   epoch + 1,
	}
	if networkId.String() == config.MainnetId {
		snapshot.Version = 0
	}
	return &common.SnapshotWithTopologicalOrder{
		Snapshot:         snapshot,
		TopologicalOrder: uint64(len(gns.Nodes)),
	}, signed
}

func readGenesis(path string) (*Genesis, error) {
	f, err := os.ReadFile(path)
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
		if in.Balance.Cmp(pledgeAmount(0)) != 0 {
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
