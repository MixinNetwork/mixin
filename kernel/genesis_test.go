package kernel

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestGenesis(t *testing.T) {
	assert := assert.New(t)

	root, err := os.MkdirTemp("", "mixin-genesis-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(assert, root)
	assert.NotNil(node)

	now, err := time.Parse(time.RFC3339, "2019-02-28T00:00:00Z")
	assert.Nil(err)
	assert.Equal(uint64(now.UnixNano()), node.Epoch)

	assert.Equal("6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997", node.networkId.String())
	nodes := node.NodesListWithoutState(uint64(now.UnixNano())+1, false)
	assert.Len(nodes, 15)
	for i, n := range nodes {
		assert.Equal(node.Epoch, n.Timestamp)
		assert.Equal(common.NodeStateAccepted, n.State)
		assert.Equal(genesisNodes[i], n.IdForNetwork.String())
	}

	snapshots, err := node.persistStore.ReadSnapshotsSinceTopology(0, 100)
	assert.Nil(err)
	assert.Len(snapshots, 16)

	var genesisSnapshots []*SnapshotJSON
	err = json.Unmarshal([]byte(genesisSnapshotsData), &genesisSnapshots)
	assert.Nil(err)
	for i, s := range snapshots {
		g := genesisSnapshots[i]
		assert.Equal(g.Hash.String(), s.Hash.String())
		assert.Equal(g.NodeId, s.NodeId)
		assert.Nil(g.References)
		assert.Nil(s.References)
		assert.Equal(g.RoundNumber, s.RoundNumber)
		assert.Equal(g.Timestamp, s.Timestamp)
		assert.Equal(g.TopologicalOrder, s.TopologicalOrder)
		assert.Equal(g.Transaction.String(), s.Transaction.String())
		assert.Equal(g.Version, s.Version)
	}
}

type SnapshotJSON struct {
	Version     uint8       `json:"version"`
	NodeId      crypto.Hash `json:"node"`
	Transaction crypto.Hash `json:"transaction"`
	References  *struct {
		Self     crypto.Hash `json:"self"`
		External crypto.Hash `json:"external"`
	} `json:"references"`
	RoundNumber      uint64      `json:"round"`
	Timestamp        uint64      `json:"timestamp"`
	Hash             crypto.Hash `json:"hash"`
	TopologicalOrder uint64      `json:"topology"`
}

var (
	genesisNodes = []string{
		"028d97996a0b78f48e43f90e82137dbca60199519453a8fbf6e04b1e4d11efc9",
		"1334081011398877b225a11a680440f8edbc2b3dd8b4a33cf90e571069d4c471",
		"2a3441d05c5974115b22e99506afb111e2d5f62845324cbcdd166af2b78b8076",
		"307ecfa84d100ecd6bc32743972083e5178e02db049ce16bfd743f3ae52fefc5",
		"5162c1d2bf0c515b033b3f00dcbc860fae2399e27fa446b835f4c3bf29ca0698",
		"54028e4cd6d088f83b95c25513843bf61f808898fd8a8fde47789214ff3d8e39",
		"65eb96be52e4bee3458dc3d5e72ee7c9c84d20e65bef2000802369adab968fe8",
		"83726bf919b63e1bdf8d0a7f59fd3cffa2be4400fa44939a4c0b9b6e67b063ec",
		"8a73e165fbc52ccc2a50e3971e15933efe5276cce9f8740abdc6518333c935d2",
		"a0bb55de68598e6133761c21ba1b05b5168ac7bc11993f6f2fa9ac5f3a157847",
		"a6d767d42e93644949e6a92ad9177f1a64dc991f83e6644a56d7d298ff41d22a",
		"a721a4fc0c667c4a1222c8d80350cbe07dab55c49942c8100a8c5e2f5bb4ec50",
		"b1ff822e0fc8e1510c0f5eeeb18d3cdc7513bc2142bc936efb2649f2178a6b0c",
		"e346266a3c9f8817dd699a1431f8712b8f3e81e43f7d56b7fad445c1c2a5b3de",
		"edc14960841f8d46a408b09c834a80f40a042fe6c4b632b6bc3b27195a2443e0",
	}
	genesisSnapshotsData = `[{"hash":"75eabab3b5e3fe0a811bc2969f32716cc58bac7260b112380be45a23fc839939","node":"a721a4fc0c667c4a1222c8d80350cbe07dab55c49942c8100a8c5e2f5bb4ec50","references":null,"round":0,"timestamp":1551312000000000000,"topology":0,"transaction":"f3a94f83f0a579d1a1b87f713d934df44e9b888216938667e7b2817aba71ef93","version":0},{"hash":"05b5e35f3fcef4895d35325a5c5fb9f95f71ec11f1b0f8cbd962581b6c89783e","node":"307ecfa84d100ecd6bc32743972083e5178e02db049ce16bfd743f3ae52fefc5","references":null,"round":0,"timestamp":1551312000000000000,"topology":1,"transaction":"e97aa8e4a9bf64f8c489cc696df05a8c8ab1f7a81cfe81a60ca66016c8c3b010","version":0},{"hash":"4c874bd7919149ef0bdae5d664dec2dbb9e1211be67de71e45131012aff3c115","node":"8a73e165fbc52ccc2a50e3971e15933efe5276cce9f8740abdc6518333c935d2","references":null,"round":0,"timestamp":1551312000000000000,"topology":2,"transaction":"2d259a9cbe49eccd7878112e291e378fee7c08af0b443c598b1fbc091d7345fc","version":0},{"hash":"cc8fd5f8454bb932d42cdd0d6327868ef1351e673714688e1abbb84ee98b37e7","node":"54028e4cd6d088f83b95c25513843bf61f808898fd8a8fde47789214ff3d8e39","references":null,"round":0,"timestamp":1551312000000000000,"topology":3,"transaction":"54aaa0e545a1e86d957d49a9b8901ade177d400bdc7d25292f647dea345a7757","version":0},{"hash":"16c7e9b96cc295bccaf7ad46543a04aa798d89fd328a8c31583c1b32a7cdeb56","node":"65eb96be52e4bee3458dc3d5e72ee7c9c84d20e65bef2000802369adab968fe8","references":null,"round":0,"timestamp":1551312000000000000,"topology":4,"transaction":"492bb359de0a40e0b71c6b26ecee7d7f48a8fdc3d1f7446942681b6c0dcae822","version":0},{"hash":"015d43f8232be2d86105aec38f03eb6e3ba4864cb2d5a743232b436d010c59ea","node":"028d97996a0b78f48e43f90e82137dbca60199519453a8fbf6e04b1e4d11efc9","references":null,"round":0,"timestamp":1551312000000000000,"topology":5,"transaction":"dfcfbccb6d36fd86024fde98040cd9abcd984c4b88f992c8503bfb28daf4d259","version":0},{"hash":"5dcaab6ebe213cc6c843ebce0490aa34f5d27eb0914e74cf001cb04ef74d65fb","node":"5162c1d2bf0c515b033b3f00dcbc860fae2399e27fa446b835f4c3bf29ca0698","references":null,"round":0,"timestamp":1551312000000000000,"topology":6,"transaction":"f8cd366f926c8f27f9b359392caf304325759bd21a1a1b1e0a479a00ec38b896","version":0},{"hash":"5744b0cd97fdb01d775054090443c54ec115829fbda33552a8d8a0c524ca411f","node":"1334081011398877b225a11a680440f8edbc2b3dd8b4a33cf90e571069d4c471","references":null,"round":0,"timestamp":1551312000000000000,"topology":7,"transaction":"ad9094b6024b5968ae189f3c9c63cb2a9d9cfbc3191994200e75fdaf09995085","version":0},{"hash":"9849466d022029e24f3ff6b1afb459f5736d86e5be9f794fa8c14048365afee3","node":"a6d767d42e93644949e6a92ad9177f1a64dc991f83e6644a56d7d298ff41d22a","references":null,"round":0,"timestamp":1551312000000000000,"topology":8,"transaction":"28304956c10f4a4e3358505ad784c4832a7ce484648d0aece5744b3b58334c02","version":0},{"hash":"91744feec431a8c54dce4f4329cfc2e2522b1d4af9ceb406c674392fa3bb552e","node":"b1ff822e0fc8e1510c0f5eeeb18d3cdc7513bc2142bc936efb2649f2178a6b0c","references":null,"round":0,"timestamp":1551312000000000000,"topology":9,"transaction":"b85a5cbb9c4f7ef75d5b346b91e0cdfc0b3b929503f94a47b28d5bf7e8a3ae98","version":0},{"hash":"a961fb2623c84de7a793d5cfd34274a6810e5a23f4639b9baf27713aea7df958","node":"e346266a3c9f8817dd699a1431f8712b8f3e81e43f7d56b7fad445c1c2a5b3de","references":null,"round":0,"timestamp":1551312000000000000,"topology":10,"transaction":"762549b76f3947d668da23a4fcb70e1f96ad725eab0c56fa48a05129ad03e491","version":0},{"hash":"949a9242d41654a473a1e34dc560788995f4453563bb44ef698e9a853d1b66a3","node":"edc14960841f8d46a408b09c834a80f40a042fe6c4b632b6bc3b27195a2443e0","references":null,"round":0,"timestamp":1551312000000000000,"topology":11,"transaction":"4c6b8e520cdaa328a47783e90c36301787279cc73960876d427c63871685af40","version":0},{"hash":"fb18373c3efec76633b3e6074a48b22ec2ec5445cfb118405ec62489fcf3003f","node":"83726bf919b63e1bdf8d0a7f59fd3cffa2be4400fa44939a4c0b9b6e67b063ec","references":null,"round":0,"timestamp":1551312000000000000,"topology":12,"transaction":"d845aa8280ce96bfbf239ead9f82b8a759a5776f09aa95a74387186523493b83","version":0},{"hash":"1888767a8095726fb8e151c5031ea175d099cc7942ba939570d685ab2ed630bd","node":"a0bb55de68598e6133761c21ba1b05b5168ac7bc11993f6f2fa9ac5f3a157847","references":null,"round":0,"timestamp":1551312000000000000,"topology":13,"transaction":"442c03d7ca8021cdb2037764fcd8e80e3aaa882373da2ad89db2ed0c62601288","version":0},{"hash":"6b2c64ba1a70de5b447ca9e203d1b82ed08d9392f56502c4e10c035907cb94e0","node":"2a3441d05c5974115b22e99506afb111e2d5f62845324cbcdd166af2b78b8076","references":null,"round":0,"timestamp":1551312000000000000,"topology":14,"transaction":"59c9398b48a8f91a5a298fd8d72ec77624ccb41311c25f07d7f126dfb9577e83","version":0},{"hash":"35882901dbeae376b01cf61d7ef0d58d3f9545878c0f9649c086628f1eaf9ab7","node":"a721a4fc0c667c4a1222c8d80350cbe07dab55c49942c8100a8c5e2f5bb4ec50","references":null,"round":0,"timestamp":1551312000000000001,"topology":15,"transaction":"4e24675df8a9d1592c82d6fa9ef86881fb2dfafe2a06b2a51134daf5a98f8411","version":0}]`
)
