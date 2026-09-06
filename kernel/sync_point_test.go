package kernel

import (
	"fmt"
	"sync"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/p2p"
	"github.com/stretchr/testify/require"
)

func TestUpdateSyncPointAdmission(t *testing.T) {
	localID := crypto.Blake3Hash([]byte("graph recipient"))
	peerID := crypto.Blake3Hash([]byte("graph sender"))
	otherID := crypto.Blake3Hash([]byte("other graph peer"))
	points := []*p2p.SyncPoint{
		{NodeId: localID, Number: 2, Hash: crypto.Blake3Hash([]byte("new round"))},
		{NodeId: otherID, Number: 3, Hash: crypto.Blake3Hash([]byte("other round"))},
	}
	for _, test := range []struct {
		name       string
		peerState  string
		localState string
		observer   bool
		configured bool
		want       bool
	}{
		{name: "accepted peer", peerState: common.NodeStateAccepted, want: true},
		{name: "pledging peer", peerState: common.NodeStatePledging, want: true},
		{name: "unknown peer"},
		{name: "removed peer", peerState: common.NodeStateRemoved},
		{name: "configured relayer", configured: true, want: true},
		{name: "configured consensus peer", peerState: common.NodeStateAccepted, configured: true, want: true},
		{name: "pledging recipient rejects unknown peer", localState: common.NodeStatePledging},
		{name: "pledging recipient accepts configured relayer", localState: common.NodeStatePledging, configured: true, want: true},
		{name: "observer accepts client graph", observer: true, want: true},
	} {
		for _, isRelayer := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/relayer=%t", test.name, isRelayer), func(t *testing.T) {
				node := &Node{
					IdForNetwork: localID,
					isRelayer:    isRelayer,
					custom:       &config.Custom{},
					SyncPoints: &syncMap{mutex: new(sync.RWMutex), m: map[crypto.Hash]*p2p.SyncPoint{
						peerID:  {NodeId: localID, Number: 1},
						otherID: {NodeId: localID, Number: 1},
					}},
				}
				var nodes []*CNode
				if !test.observer {
					state := test.localState
					if state == "" {
						state = common.NodeStateAccepted
					}
					nodes = append(nodes, &CNode{IdForNetwork: localID, State: state})
				}
				if test.peerState != "" {
					nodes = append(nodes, &CNode{
						IdForNetwork: peerID,
						State:        test.peerState,
					})
				}
				node.nodeStateSequences = []*NodeStateSequence{{NodesWithoutState: nodes}}
				node.custom.P2P.Seeds = []string{otherID.String() + "@127.0.0.1:17001"}
				if test.configured {
					node.custom.P2P.Seeds = append(node.custom.P2P.Seeds, peerID.String()+"@127.0.0.1:17000")
				}
				before := node.SyncPoints.Map()
				node.SyncPointsMap = node.SyncPoints.Map()

				require.Equal(t, test.want, node.UpdateSyncPoint(peerID, points))
				if test.want {
					require.Equal(t, points[0], node.SyncPointsMap[peerID])
					require.Equal(t, before[otherID], node.SyncPointsMap[otherID])
					require.Len(t, node.SyncPointsMap, len(before))
				} else {
					require.Equal(t, before, node.SyncPointsMap)
				}
				require.Equal(t, node.SyncPointsMap, node.SyncPoints.Map())
			})
		}
	}
}
