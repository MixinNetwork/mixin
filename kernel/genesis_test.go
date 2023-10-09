package kernel

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	require := require.New(t)

	root, err := os.MkdirTemp("", "mixin-genesis-test")
	require.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(require, root)
	require.NotNil(node)

	now, err := time.Parse(time.RFC3339, "2019-02-28T00:00:00Z")
	require.Nil(err)
	require.Equal(uint64(now.UnixNano()), node.Epoch)

	require.Equal("a1a2b0262a3b5bf0c88d03fd29867db83826a7d9648bb4fd79f0b1ba67f1d1e8", node.networkId.String())
	nodes := node.NodesListWithoutState(uint64(now.UnixNano())+1, false)
	require.Len(nodes, 15)
	for i, n := range nodes {
		require.Equal(node.Epoch, n.Timestamp)
		require.Equal(common.NodeStateAccepted, n.State)
		require.Equal(genesisNodes[i], n.IdForNetwork.String())
	}

	snapshots, err := node.persistStore.ReadSnapshotsSinceTopology(0, 100)
	require.Nil(err)
	require.Len(snapshots, 16)

	var genesisSnapshots []*SnapshotJSON
	err = json.Unmarshal([]byte(genesisSnapshotsData), &genesisSnapshots)
	require.Nil(err)
	for i, s := range snapshots {
		g := genesisSnapshots[i]
		require.Equal(g.Hash.String(), s.Hash.String())
		require.Equal(g.NodeId.String(), s.NodeId.String())
		require.Nil(g.References)
		require.Nil(s.References)
		require.Equal(g.RoundNumber, s.RoundNumber)
		require.Equal(g.Timestamp, s.Timestamp)
		require.Equal(g.TopologicalOrder, s.TopologicalOrder)
		require.Equal(g.Transactions[0].String(), s.SoleTransaction().String())
		require.Equal(g.Version, s.Version)
	}
}

type SnapshotJSON struct {
	Version      uint8         `json:"version"`
	NodeId       crypto.Hash   `json:"node"`
	Transactions []crypto.Hash `json:"transactions"`
	References   *struct {
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
		"01d2c24cb71f6c92ce6fc0615d3eae3d297b0ef4c578dc22f4795efbaf84523c",
		"044241f4f8381f1cf9fa913f17a934ad8eb323b53e89db917136133545ad6175",
		"252f3916000ffeddb70b28c014766f4b4b8852413b029264e9455f55c4081b4b",
		"347a0dcd3ad6c9a4d006d7c02095db97156305dc7e464fc69ae8143f020e38df",
		"38ac19845405a2e782e6005fbc1392ec97efd85183156cbf10ce2d9d0eb2bafd",
		"4755f0eba9d7318d6758380edbb5083535ead1fe03d5fdf7421bfad2c1ce78ba",
		"71eb92b9030a4c72db66b25bd96c078957b5431b3fc56493d85f687d91956109",
		"99b98cb6baf01710310c9d7c2a3ddea16459296f04d32729c30062edfd7748ab",
		"9c08ab01eeb09cdcd5ee7d7854fddbf9a046b308d944d557bec4a9a386aebd39",
		"a18c4bd994bbf245382b12912c348572ab238bfbac565dab590d091d4901ab94",
		"aba065c3ad175d7b1fd324e627d8e36223f9680c9923fd6c5b70f7a337025908",
		"b40af523ad71182d533243cf3d5eb02009fff993db51b586eefe1067714ea866",
		"c51e206f4246722cd574d9b631379fcddca01daa421978b917b411aba2f782ed",
		"ce3a5ad4ababdfa850113f0c6e5e00cdd983829f412588fe7309160c86c22074",
		"fb6497c90464a7160fe46b8975bbdbead6cc4ed7efd87f27c9dce319f6da9dc7",
	}
	genesisSnapshotsData = `[{"hash":"8d3b0cd47a17acaa7718c0ad5c507272d530228dd20e1ef1ef49cda6783a0058","hex":"77770002252f3916000ffeddb70b28c014766f4b4b8852413b029264e9455f55c4081b4b00000000000000000000000122cb2ed32fe762f135bea74ba858bb8dbccbb2cc1954c5fe41c98b3d51aec63615875e0b77cd000000000000000000000000000000000000","node":"252f3916000ffeddb70b28c014766f4b4b8852413b029264e9455f55c4081b4b","references":null,"round":0,"timestamp":1551312000000000000,"topology":0,"transactions":["22cb2ed32fe762f135bea74ba858bb8dbccbb2cc1954c5fe41c98b3d51aec636"],"version":2,"witness":{"signature":"85b42f518ee417a24b788ad516b739900e37d1c1a974fc93abd712af594defcf1539c263513764a9261d528344f213fc5585098168d37761572a169c37d99100","timestamp":1696861695862632198}},{"hash":"2f8e907fc531af931f130c8a69e444911fcc7e92af3b543a76535be014a61809","hex":"7777000238ac19845405a2e782e6005fbc1392ec97efd85183156cbf10ce2d9d0eb2bafd00000000000000000000000173776e1696b8caede8aa2f0d28f7bb8e6fa56a19b5f7c3b0a2bf81e7dc2c331615875e0b77cd000000000000000000000000000000000001","node":"38ac19845405a2e782e6005fbc1392ec97efd85183156cbf10ce2d9d0eb2bafd","references":null,"round":0,"timestamp":1551312000000000000,"topology":1,"transactions":["73776e1696b8caede8aa2f0d28f7bb8e6fa56a19b5f7c3b0a2bf81e7dc2c3316"],"version":2,"witness":{"signature":"f2d5ed345010df2c19d0cf864b08db2cb08576bcb6081e427cd153f0bb3a52a489b5519bb41099af07e5de42e7778afc71a925c70f8a440dbb6f019bd6904e08","timestamp":1696861695862665253}},{"hash":"0e48324c667ad7f66f7f14cb1040c8055ed84d06878f123928403d172102342e","hex":"777700029c08ab01eeb09cdcd5ee7d7854fddbf9a046b308d944d557bec4a9a386aebd390000000000000000000000019cbfebf96b8563253a7db417cbd5f896f83eab4d170159168e9926f2d9200a7115875e0b77cd000000000000000000000000000000000002","node":"9c08ab01eeb09cdcd5ee7d7854fddbf9a046b308d944d557bec4a9a386aebd39","references":null,"round":0,"timestamp":1551312000000000000,"topology":2,"transactions":["9cbfebf96b8563253a7db417cbd5f896f83eab4d170159168e9926f2d9200a71"],"version":2,"witness":{"signature":"e124ceabde1909d955e402f086ba57fbb899532530835e7a63bd5b14398e2adfeee0d13dbaa2f53fbfeb9e89bd6aa57782d082271b57bfc0dc66a5d007751f0a","timestamp":1696861695862695582}},{"hash":"55ac3eb12f076b36d75900fabf70c1cfebae2a46d99416a6d05850204a845ec6","hex":"777700024755f0eba9d7318d6758380edbb5083535ead1fe03d5fdf7421bfad2c1ce78ba0000000000000000000000011cf8878e94d1dad9f5a536c9baa92157c0269b3b93fec25e3090bcecaf9e393e15875e0b77cd000000000000000000000000000000000003","node":"4755f0eba9d7318d6758380edbb5083535ead1fe03d5fdf7421bfad2c1ce78ba","references":null,"round":0,"timestamp":1551312000000000000,"topology":3,"transactions":["1cf8878e94d1dad9f5a536c9baa92157c0269b3b93fec25e3090bcecaf9e393e"],"version":2,"witness":{"signature":"67331cdfe0d9f6c74c29a5768d1137224b3e108a694ada52e6af4dbe5933c9882ea1bfc5c1a8bd309157ffc7fe2d266863e0e4593191fc31d54176ed1a469e05","timestamp":1696861695862725156}},{"hash":"858c4f055e6d2ec53cae2aae347d4a1f428123cd2a90d438442b575277742d33","hex":"7777000299b98cb6baf01710310c9d7c2a3ddea16459296f04d32729c30062edfd7748ab000000000000000000000001a5769d9cb0a6ed56641ea1e71f33a6f5279f304207689f38e53dfaf923f4603c15875e0b77cd000000000000000000000000000000000004","node":"99b98cb6baf01710310c9d7c2a3ddea16459296f04d32729c30062edfd7748ab","references":null,"round":0,"timestamp":1551312000000000000,"topology":4,"transactions":["a5769d9cb0a6ed56641ea1e71f33a6f5279f304207689f38e53dfaf923f4603c"],"version":2,"witness":{"signature":"6ec113930f7f3d863aa45f1599adb8ab5008a5251eaafaed5e3fd465b2a1fddfff56ead5109b043b896912ad517cd355582e1f48570d3271bf781287071d3702","timestamp":1696861695862756407}},{"hash":"70cd37e2b9608e046008c1470ab38dcce526048c6fe88ab8ec92256d0b473eeb","hex":"77770002347a0dcd3ad6c9a4d006d7c02095db97156305dc7e464fc69ae8143f020e38df000000000000000000000001f622ab27ffb3f2f7fd6d1406a4dc8e4f8b12d42ad0a2c57b1e1fee31fba2df7615875e0b77cd000000000000000000000000000000000005","node":"347a0dcd3ad6c9a4d006d7c02095db97156305dc7e464fc69ae8143f020e38df","references":null,"round":0,"timestamp":1551312000000000000,"topology":5,"transactions":["f622ab27ffb3f2f7fd6d1406a4dc8e4f8b12d42ad0a2c57b1e1fee31fba2df76"],"version":2,"witness":{"signature":"028bfbaef6b05e331c61fa85bfe53d6f565c2d18de463ec541d1ca72cd42eebfb3e969ce024d6868e8b308b9e58bccc627ed5b23662b75845dcafc1a80ffd50f","timestamp":1696861695862782135}},{"hash":"85919756bc7b1d15bc0e7f905f2fee2b5228c9fa5647baa27a4074a090e56264","hex":"7777000271eb92b9030a4c72db66b25bd96c078957b5431b3fc56493d85f687d91956109000000000000000000000001514a9ca792f057089d30e399d15cdbed1e59771cbe70da7e6fa776871bd2dd0c15875e0b77cd000000000000000000000000000000000006","node":"71eb92b9030a4c72db66b25bd96c078957b5431b3fc56493d85f687d91956109","references":null,"round":0,"timestamp":1551312000000000000,"topology":6,"transactions":["514a9ca792f057089d30e399d15cdbed1e59771cbe70da7e6fa776871bd2dd0c"],"version":2,"witness":{"signature":"b908781bad39afee837cae859f3fede0037221e81d0cea3ffa4c51b1e011d5ada45e7f4e27940dac9e072b1dd3ff08bf0030f109c1941494d689f3a4a13b1002","timestamp":1696861695862807937}},{"hash":"98a81bc2580b562e1355cccd2d9011df5992a4240bc0a5f46dddfd8029debe37","hex":"77770002044241f4f8381f1cf9fa913f17a934ad8eb323b53e89db917136133545ad617500000000000000000000000106a1e302f59c3a3f3655f38ff57beee47957d4e682fe16ed5b18c439cfa8078115875e0b77cd000000000000000000000000000000000007","node":"044241f4f8381f1cf9fa913f17a934ad8eb323b53e89db917136133545ad6175","references":null,"round":0,"timestamp":1551312000000000000,"topology":7,"transactions":["06a1e302f59c3a3f3655f38ff57beee47957d4e682fe16ed5b18c439cfa80781"],"version":2,"witness":{"signature":"fac7fdad096ae6a17428aeb127a1f2c70bac220cdd41f4091cb1cc3a6c4aa5ae469f708dc8409d8ab8f84a6ce4adaad15f918a30e6ea83cd90ad1a4ab0f6fd0d","timestamp":1696861695862833581}},{"hash":"0f926daa68dfb675728723813e36c6914a4c1e2b8bcbfcf6cbf60a1b3db1c785","hex":"77770002b40af523ad71182d533243cf3d5eb02009fff993db51b586eefe1067714ea866000000000000000000000001e9fc1aca197bb1fa8ef9e663e6fa18dfc14160bfa6d219694a310eda082e26b515875e0b77cd000000000000000000000000000000000008","node":"b40af523ad71182d533243cf3d5eb02009fff993db51b586eefe1067714ea866","references":null,"round":0,"timestamp":1551312000000000000,"topology":8,"transactions":["e9fc1aca197bb1fa8ef9e663e6fa18dfc14160bfa6d219694a310eda082e26b5"],"version":2,"witness":{"signature":"ef16b129454293df00ef58045dab6718d90cf522625adfc86f30c9cb345d4803b55c06fd792acb72fbd301ed6767fbdd144f500fc8aebffefa79b8e7911ea100","timestamp":1696861695862859126}},{"hash":"4d24d64b46bcfb69373d0629837342cb5c440b79327947f3f8c209c2bb5d9185","hex":"77770002fb6497c90464a7160fe46b8975bbdbead6cc4ed7efd87f27c9dce319f6da9dc7000000000000000000000001e9f6941a540e2d90fc1cbda12bc7454593bb832878fdc7ade4904a2c16c3a53b15875e0b77cd000000000000000000000000000000000009","node":"fb6497c90464a7160fe46b8975bbdbead6cc4ed7efd87f27c9dce319f6da9dc7","references":null,"round":0,"timestamp":1551312000000000000,"topology":9,"transactions":["e9f6941a540e2d90fc1cbda12bc7454593bb832878fdc7ade4904a2c16c3a53b"],"version":2,"witness":{"signature":"46cde4599447f1b8601b355edb0f0614729c472a308be05ace5d5662b5feeb27384effd8bb5acc21df8d21f85c98d3a2522001e03f33ecbf34661bda9e43100f","timestamp":1696861695862884856}},{"hash":"47564ade3c95b7fd46ab6c6d951efc0893809bfd8a04c4316e9b20a25e3a6711","hex":"7777000201d2c24cb71f6c92ce6fc0615d3eae3d297b0ef4c578dc22f4795efbaf84523c0000000000000000000000019717ab81d20998c1bca3cfe2cb871bbe813b4f44a916160e5788935e482228ac15875e0b77cd00000000000000000000000000000000000a","node":"01d2c24cb71f6c92ce6fc0615d3eae3d297b0ef4c578dc22f4795efbaf84523c","references":null,"round":0,"timestamp":1551312000000000000,"topology":10,"transactions":["9717ab81d20998c1bca3cfe2cb871bbe813b4f44a916160e5788935e482228ac"],"version":2,"witness":{"signature":"c965b7786630f77ddac3824db980d573bdf93350ed72da90bcadd9385933da74a5871ae109754a0f1e6c447d2fd5a25361ff2eb4a8802e277b2ff04f40f54b0d","timestamp":1696861695862913290}},{"hash":"c8a01638af4bee02c393aeb2995e85378e838d7594d510e5f95c3695d2aaa590","hex":"77770002aba065c3ad175d7b1fd324e627d8e36223f9680c9923fd6c5b70f7a33702590800000000000000000000000142d9e20240dfc3874de79579b1599eb8f249c42b5af94e1ae67e407ff39ed8fe15875e0b77cd00000000000000000000000000000000000b","node":"aba065c3ad175d7b1fd324e627d8e36223f9680c9923fd6c5b70f7a337025908","references":null,"round":0,"timestamp":1551312000000000000,"topology":11,"transactions":["42d9e20240dfc3874de79579b1599eb8f249c42b5af94e1ae67e407ff39ed8fe"],"version":2,"witness":{"signature":"ff0a80e65df8605c809551e220095ffe4b5ec403778dc27574039cffb9dd4aa0ae58e81757f9a21a60957b3bd82b907e3066b6cb4f3a493e4cc1cef63acc6700","timestamp":1696861695862939539}},{"hash":"b70abc49f021b42712135e184efeddfdb722de95325e0fd326ccb52edfcb2c77","hex":"77770002a18c4bd994bbf245382b12912c348572ab238bfbac565dab590d091d4901ab9400000000000000000000000126fa15445491c71ed1727550011146ebb8eb691dc1cffab8d7b67fe1c0e821d415875e0b77cd00000000000000000000000000000000000c","node":"a18c4bd994bbf245382b12912c348572ab238bfbac565dab590d091d4901ab94","references":null,"round":0,"timestamp":1551312000000000000,"topology":12,"transactions":["26fa15445491c71ed1727550011146ebb8eb691dc1cffab8d7b67fe1c0e821d4"],"version":2,"witness":{"signature":"57ea1943fffdf854cffb5093d5902c76caaf97769281bb730650f1b361ff445fba5c0925dce77cbc3316dc186c021a2518517e79b5a81ca040e6e0e105de8507","timestamp":1696861695862965943}},{"hash":"5de098d04f3f213bc6dbfd085a693b597785f9e79a20be97ca987a9974a1d475","hex":"77770002ce3a5ad4ababdfa850113f0c6e5e00cdd983829f412588fe7309160c86c2207400000000000000000000000167f25c155d96aaa137e99d734c06d254a118913a21164d0fb2d5cb09583f74ea15875e0b77cd00000000000000000000000000000000000d","node":"ce3a5ad4ababdfa850113f0c6e5e00cdd983829f412588fe7309160c86c22074","references":null,"round":0,"timestamp":1551312000000000000,"topology":13,"transactions":["67f25c155d96aaa137e99d734c06d254a118913a21164d0fb2d5cb09583f74ea"],"version":2,"witness":{"signature":"c7640fa67c6b2dae4fac02c8daf04b7d374055b57214c2b2ca9f785e0373ccf781534f6cfd35e47b2966fdceb26f4a430bd2f2df22ada116fd4dc828bde1e002","timestamp":1696861695862991907}},{"hash":"5d51a156812d157cbcc63c9dd1dabd5f2f7ed46511366d23428f42e890b389bb","hex":"77770002c51e206f4246722cd574d9b631379fcddca01daa421978b917b411aba2f782ed000000000000000000000001105391d0641769b4755471719bc01542c7de0c765fe0b1a5747ef0a073a7f6ff15875e0b77cd00000000000000000000000000000000000e","node":"c51e206f4246722cd574d9b631379fcddca01daa421978b917b411aba2f782ed","references":null,"round":0,"timestamp":1551312000000000000,"topology":14,"transactions":["105391d0641769b4755471719bc01542c7de0c765fe0b1a5747ef0a073a7f6ff"],"version":2,"witness":{"signature":"52a67780b8eb9702d1e23d3b4c6780d5b61fc9847f3e972e800cf37f80c1d7f72b644d4f23a71f01be22d2716931a2b48b509d00b1fe80d2be910c9051a7a00b","timestamp":1696861695863018290}},{"hash":"c2502097daefb472dd25a2fb7c17b92bd237d0b65787ee41491d5749ac8f1e6d","hex":"77770002252f3916000ffeddb70b28c014766f4b4b8852413b029264e9455f55c4081b4b000000000000000000000001de180b5839c1e203fc5309e0ba2b555f880f93496f11cdcaa98c8a8cbfd2444415875e0b77cd00010000000000000000000000000000000f","node":"252f3916000ffeddb70b28c014766f4b4b8852413b029264e9455f55c4081b4b","references":null,"round":0,"timestamp":1551312000000000001,"topology":15,"transactions":["de180b5839c1e203fc5309e0ba2b555f880f93496f11cdcaa98c8a8cbfd24444"],"version":2,"witness":{"signature":"adb02704045b6e14f592731913485b8dea4d37c722c0e5490fb4906d8a968ec3543bed6222933d19b86edd9fee96a49aae675e6551d10979dbcde46207c7fe0b","timestamp":1696861695863046518}}]`
)
