package kernel

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
)

func (node *Node) validateCustodianUpdateNodes(s *common.Snapshot, tx *common.VersionedTransaction, finalized bool) error {
	timestamp := s.Timestamp
	if s.Timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}

	if timestamp < node.Epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Epoch, timestamp)
	}
	since := timestamp - node.Epoch
	hours := int(since / 3600000000000)
	kmb, kme := config.KernelMintTimeBegin, config.KernelMintTimeEnd
	if hours%24+1 >= kmb && hours%24 <= kme+1 {
		return fmt.Errorf("invalid custodian update hour %d", hours%24)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if !finalized && timestamp+threshold*2 < node.GraphTimestamp {
		return fmt.Errorf("invalid custodian update snapshot timestamp %d %d", node.GraphTimestamp, timestamp)
	}

	curs, err := common.ParseCustodianUpdateNodesExtra(tx.Extra)
	if err != nil {
		return err
	}
	if len(curs.Nodes) < 7 {
		return fmt.Errorf("invalid custodian nodes count %d", len(curs.Nodes))
	}

	prev, err := node.persistStore.ReadCustodian(timestamp)
	if err != nil {
		return err
	}
	if prev == nil {
		domains := node.persistStore.ReadDomains()
		if len(domains) != 1 {
			return fmt.Errorf("invalid domains count %d", len(domains))
		}
		prev = &common.CustodianUpdateRequest{Custodian: &domains[0].Account}
	}
	if !prev.Custodian.PublicSpendKey.Verify(tx.Extra[:len(tx.Extra)-64], *curs.Signature) {
		return fmt.Errorf("invalid custodian update approval signature %x", tx.Extra)
	}

	all := node.persistStore.ReadAllNodes(timestamp, false)
	filter := make(map[string]bool)
	for _, n := range all {
		filter[n.Payee.String()] = true
	}
	for _, n := range curs.Nodes {
		if filter[n.Payee.String()] {
			continue
		}
		return fmt.Errorf("invalid custodian node %v", n)
	}
	return nil
}
