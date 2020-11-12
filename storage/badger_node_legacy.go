package storage

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v2"
)

// FIXME remove this legacy node state migration file

const (
	legacygraphPrefixNodePledge = "NODESTATEPLEDGE"
	legacygraphPrefixNodeAccept = "NODESTATEACCEPT"
	legacygraphPrefixNodeRemove = "NODESTATEREMOVE"
	legacygraphPrefixNodeCancel = "NODESTATECANCEL"
)

func (s *BadgerStore) TryToMigrateNodeStateQueue() error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	lnodes := readAllLegacyNodes(txn)
	if len(lnodes) == 0 {
		return nil
	}
	sort.Slice(lnodes, func(i, j int) bool {
		a, b := lnodes[i], lnodes[j]
		if a.Timestamp < b.Timestamp {
			return true
		}
		if a.Timestamp > b.Timestamp {
			return false
		}
		return bytes.Compare(a.Signer.PublicSpendKey[:], b.Signer.PublicSpendKey[:]) < 0
	})

	if nodes := readAllNodes(txn, uint64(time.Now().UnixNano()), true); len(nodes) == len(lnodes) {
		return nil
	} else if len(nodes) != 0 {
		return fmt.Errorf("malformed state with both legacy and new nodes %d %d", len(lnodes), len(nodes))
	}

	for _, n := range lnodes {
		key := nodeStateQueueKey(n.Signer.PublicSpendKey, n.Timestamp)
		val := nodeEntryValue(n.Payee.PublicSpendKey, n.Transaction, n.State)
		err := txn.Set(key, val)
		if err != nil {
			return err
		}
	}
	return txn.Commit()
}

func readAllLegacyNodes(txn *badger.Txn) []*common.Node {
	nodes := make([]*common.Node, 0)
	accepted := readLagacyNodesInState(txn, legacygraphPrefixNodeAccept)
	nodes = append(nodes, accepted...)
	pledging := readLagacyNodesInState(txn, legacygraphPrefixNodePledge)
	nodes = append(nodes, pledging...)
	removed := readLagacyNodesInState(txn, legacygraphPrefixNodeRemove)
	nodes = append(nodes, removed...)
	canceled := readLagacyNodesInState(txn, legacygraphPrefixNodeCancel)
	return append(nodes, canceled...)
}

func readLagacyNodesInState(txn *badger.Txn, nodeState string) []*common.Node {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(nodeState)
	nodes := make([]*common.Node, 0)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		ival, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		var nc []*common.Node
		tx := nodeTransaction(ival)
		switch nodeState {
		case legacygraphPrefixNodePledge:
			nc = readLegacyPledgeNodes(txn, tx)
		case legacygraphPrefixNodeAccept:
			nc = readLegacyAcceptNodes(txn, tx)
		case legacygraphPrefixNodeRemove:
			nc = readLegacyRemoveNodes(txn, tx)
		case legacygraphPrefixNodeCancel:
			nc = readLegacyCancelNodes(txn, tx)
		}
		nodes = append(nodes, nc...)
	}
	return nodes
}

var legacyNodeStateSnapshotMap = map[string]string{
	"30beeadffe3e5d4e374b4f83c6f4df77169f644d83582c1147482b953d67fa1e": "2188eb4b969037134c6d286dd76ff9e9ad1017c91f3821638bc1aa7eac06a12e",
	"4353e6d6ca0dba8ef05a42c68f9861946d858968be04af6eab3b6fff7eb2f3ea": "f35ddb1e3c09e88274dcb82c68825f1c97a4ad52537b7e34cf86b858c660ce41",
	"6d6039f00f66d4bd4e94ecec392d1122cd03e2b623436aeffa3812c23fcfcd1c": "c9d52df3fd2f3745675c9372a36a02e5597aeb33b6698d3b089d0ce9ce3bad04",
	"619ccdbe2cdaec281c59df29a97ee6102a8a9f13c21ff53e664a53ed400fc6ac": "f535a83f6f5030b1cf1d5cdb8fd440b4f5503e85fb5a74cd30f6eecbf66d9e53",
	"f7b6d7c1e18a2879a38aea33f57287e097585f8fd45b53c18eeb3c53616a504d": "d87a6880aa4509d070cef2325d1129ae4bd1a70403df89a7ca94adf8fdc2fa34",
	"f04d157cc8ea9bcddb533c1c7bedcb1f095bc388bb49ba494458531dd5deeb76": "ff5acadd1c3f8c9f86e04e921ee3f16ecb35dbb45569bb5d2548119c3888136c",
	"65d6052e3fd931eb9f903c003f559ad521f1f90f2015f4e93272ee92f1bbc8d2": "caf430bd1c22d58684f831c276468236e79cc13e73872f251b0e46cb6f9672c5",
	"2acdb1f0615c974e7665de499074b7a34c9ef3e8692f310a02a80251d24a2be0": "92200b16e796ac38d0d37811f01ecb0243f6b475434148f8ed1a08591ab6e831",
	"b3369b651f1d2612651c0501c377fe11a99f3327ed55d78f7c12f7b077201aef": "8713ca8f63d5c4e1a82b2806360211cb6c382cdcf2bcb25760fd8515c03d2ac0",
	"48beb50b0801ac1eea17d1a5ae781e4eff91bc5f705d5d5dbb0494b39f3e0246": "da41969e39cdaac4a3012d536fd562152f8918a4e63d64d3553f0e958fa576fd",
	"1d6a41935fe484e398c58cf55710a1e8fa77977bcad6ce8d9b3ad6400316b9d9": "72e526ace49662baf8fa94793bc50e2dc68b4d957315e6acee567c7edfd76c9c",
	"5e92290f79604f12431b1895b321974e036a0929e80c4cfaca33d4a1a00011fb": "0dd6ef344036c3faebf6d7b3a28f933331c24ccf8f19946eed1783051691d776",
	"472abd56231e8a6972ca4cffa692c7b6191e937c092caefb39e08bc6f858856f": "9e525af86cbc461c53c5cdd7d4afe44aea560d0aeefd820270fd9f8851baef20",
	"c6132bc1e5fda979d95785599ff04e3fde68349d85575ce8fe9a1aff9f741572": "00251197c6b8aa9be60e382583a37b33ffdde7e4219a05047104add9bd4c960d",
	"fc2931d8b690eb7edd24d858d2e9d40c02ab3c8df0e4eb3881e764547866298f": "f5ec29da8d4e5e7b9c7070301b2b10827908bd5068067bf2e9defa4445a3b5ca",
	"8a505e00be935a30be1f42d09d5f36aaa10c02188c694cb5a899d6d91f748bc2": "0413f4a47ea08bd1d920b420aa7e55721fc9c4f35eda34bf762bcdd484a21b71",
	"6a8ac90ef310a7e6f8e521b23cc98180924ac5947075f19aff1e71d58b27bb39": "0304b11eb78c94d144bb592b86575a86df463c91c87e7bbbf835eaea08300877",
	"6b267f884020ac9c270ec6d319625a57ce8464577e8c5330bf055c36802b4339": "893350c2f2ef55a259ad4ddd24d300a4a06b0eaa0624ba66e867e00dd9cc47bd",
	"f65fcce5da91c9dfb3507206b6790d563c1713750af649aa560de48e0f26af25": "972b3056fe4d360d757bc789ef7f6c54ddb52560ca24ae9538f434744345a48b",
	"351cdf2428da47df9e809564fa95da30b8224750af47db5e788736e327acd909": "9b769200eb943419049a10c61348759a3e27e25381bead642103e29ad2ddec8c",
	"1434f235c1234a405496fc7a266a43e55dee2433e3dea47af1eb21f4397ccfd3": "0a9ddb616f5adfe399db08afb4ac767599a88f5df9ffbb02e437520eeece0df0",
	"222c5caf252ae9d4ba3f9a443ad86a7ae1bfeccfb10ca50b7480c13c8878d011": "5fe43a775fbc97d6e0c80c6fba20c6bbf5a17b5da3c4194f76bc82e804622911",
	"18d8718a556c54ff112308e3ce8840eb368521babc35cc42c9c24a6e07691a4b": "ad54776d9e29d6a0298349a20bc76bcd81d5d3f65f9b39a9994735884b7a60e9",
	"cb703d55ac13913d2cd12756892ee3d277ff8c048f163e4539488e9c028a42a6": "1ccb3844e37a65014c3713f3aa50dcc277b7c2314feb746fb51a27e395955bb2",
	"ee033344d6a5d406564f8bdf9f877d7c7757dde9c4353d279b002e7ba8a13c69": "9826d67aa190921e2268ee9d0b3f15646c4d51d09f765fb13d0334425adff2f1",
	"f0143e37c4749b65ae8f2824cbb9553c825c34583d776307e9c96b1e877fd03d": "8c4cf111f9a879a09c04766e1c1365fc8e82873849fd79cd74c1064847224348",
	"bd3dd6998c5e752d64afbadab729053ddcc7deb9a758c7739c8c372d4828e10b": "da08a1ce2f8541f1671bfca5268e924952c1b172ac12824f88fba63f4180a3ac",
	"6923cea2e53e070d715f850da60a5eb1d7af0cacc75972fb7f7f096feb73395e": "abb7d587447c2f68a852ce7163925cb90ac30631fc643b6978cd0c63a71c8dbc",
	"67a84c91c4f67ff424abdbae4a9a93414959396ac37ea2bcf9b710fe6bd91e4c": "7a91c311e6db3443a8acff9ef8bda0d3633d477e8d1f04299360dcf48ba6ed14",
	"54ca188eefe53a1d855aa4f6e5bd7670f64ad6ebe9a5b709a6ee274c6ba0cdd1": "950602efeb6e326b98cdbf7f7d1335463de581a7804e89f4763f882a3cb87bdf",
	"44d7d696e8b9887088ef1f57c8e0f5d62aa5aa2ca39f1e1b43b37134939160b1": "42f1ba1567b11b7b689f6f6e00bce5c5d63030577f2b5f726a5e0b86972803d3",
	"2e9ea4bd205dac244e662c46f2fff47a5caa7f20fe2d6416e7c22e9a674c0d18": "3ef54b90e7b621fc9640c9471a99818ada1ee40066af27b81499d95f84d16137",
	"48f3d7b5ae6b03f251705cfc82c3b3c7413ec8a7e7b100de0cab4d8f3ec33bd5": "00a1b493d68a5421c3e7add72837549a8bce4fe02e2d689334b32d7187256a47",
	"012e3c2367d5c87dae07e5a174d82b2a8fa583b8ab93de247535c03a01270f79": "b6aa3b9bd0aa2dec699845fea5d4c1f87521495f1f7d903cda7dc58e840046d4",
	"f9bf58e09b08b5b108bdea717df9fa50fc4aa6abde8e39cf130e84e1fee1ff83": "cf17ebb894763f9d61fa6c2b176a36c718895559a66e522ba3cb097cd13e5104",
	"139f2af168f237f5650660bd4bb4ca9daf100b8cb6b06345953c33bf1d6e5f93": "23e0f4c9bd53dc781e89c39fe9c20a327052c334a8c1f511db4c1286d09ba09e",
	"9a905921e53c05606165180d166f021bca554236411424836c1dcfa4bd10f216": "cbc7463f651c8a14caf81057a57ea047baa8dbe5eca0862b20aae529748727bf",
	"3a83c4113cd0966f18c3bc4e542368b247e4cad7d81d99445e40051a220afc29": "55e065a5868b83d1b54e03d17fabf4a7fd30f337e92eb0d24704fb1cb6497472",
	"19530a8b80a2b970b870ad6845ae0832eaef856bb36cafb23c3eed2e11894919": "e7ab731ce05272dfc7b0786f5b3234929c872249a29b67bd207c728fa8cf66a6",
	"9f63063e0067ab30aa47db5e0cdfeeebaa1b96fa2a3a7720cb4f314e7adfc9e3": "73c6fd696147836884e3688830a5a1a90e4a38c278c3be49afc0ca296a99eccf",
	"08703de79060cb7aa03c38d63799997c192fc71324d0cc982b5f40b5c4aeca63": "d4cac6f6daf1b2037164a9b52bda5a16d804f3be7e67957652f0295833c93c5b",
	"9cf12f132d1c13746a44a31f8ee74d078d57889eb4e74b95e14ec4d8aa666137": "93f930e89bcd46944a3a013742265361a95a9ef9381f5688ec06bb448c6f6ed1",
	"4a0c7d8efc3def785d583d0c608cbaf01361156ecb6edcd3465de2215b20c828": "b573fbb5793868cedd18710ce7d74db628ed5fa4ee9363b27a64a8ec287fb55b",
	"c3f7de9fb8454992aff3cd0474e7dd45edef12da9dc3c3db309a46c5bd1ef5fc": "3bb03c558dd3df9d88ae1185183214c82296b84388877f081b07bd2be4de3c1f",
	"e58ac139610914c1a2a24e4072dd6eb45de93dc5f105450b65842bfedc720b2a": "bb966a257ff166d669a9233c292b20de12a93f10a2ea209d287d199172e7349d",
	"81abcd0924d36b18825803c6f647d1d8069094ecc4663714eb332a55d07993ec": "8174a538f22af7d65bc1c695bdd4e7cc043b67dad31ae07bf4fc522a4907d0f4",
	"3683fdef6faa8003805eb9e26d4f869382b3c98b27e3a70453005f2894ba6a12": "92db986433f27f2ed8c7eb041be473ae4fae52a0d82b27dae607ed7ec865ddda",
	"ae8be1677312240241a3a9c7426f0bafe5451381224297dc5d8e75776a309d0e": "656a28f31246a5ae3f504c432fdbc3b3b7228a3971eb44d72a26bf74239016d6",
	"85d6ba7a48c26d5c0dc064aa296aabb1145b2be806dafe00fa3574668ac80c44": "1827b47b2e7359656facba5fac3f078f37497f8a02144290f96820e5541d99e0",
	"a7d1306e484c898a6fe6a614c2f12dec742f7bc3317e71a64dc172c4be9603e0": "2cb678d622a764b4697751debdb1b844925a296a572bdecb3042bef9cee77ee9",
	"777d85cc5b6b25fb6701a5e428fe1914a92c78139a71c83885446b3e0cadef4d": "5fd78e424a946dec539602571abfde5d567fbd44c449d5abaff9223ee5e36b10",
	"f510f8e569039c4e07f71abdc7a52f3d1a49434dc21703e10b391de54457d6ab": "c15685efee45546e11553b49062cf0b9255dc7db1e7efd70264e0ef1c79cbb07",
	"13edad49f8cc5bf4a3dc5515e10c3a79a0d2292479ced51bced0745103e92c9e": "ab8ecb42e627766ce28cab92a50cbab32317634f3c949ad4e2e8b61f18b4f938",
	"b38330707cf73150a8b74817ba24d90e1f45a70c02893e038d3014480fa65283": "a752761151712a98ef35ce7e337c8507bd8fbeda3cb1473c42d46024b2e6317a",
	"9cfc6eaed39ffc554624064c13e62adcf24be81886575d6a6d0562572833dfd5": "cc0ec83f5588856b26879cfbbb12f057fddca13b91070c38d427bd136894844e",
	"09196840259bdafd5658ef1d190ad0d67a79ca6d95dddc56e2c0ad5a6c1cebe0": "5b212b45c7926ec0f4b4d9a0cc648adb18801b6379242ac0c460050da0d1d24a",
	"0c3e8d9b29a88da52b16d02aba6dfe91bda692fbedec400be06d7e2882747e0e": "b3c0b45e35a969e6b59f5145fc5c4a5788843236a09e95992c6adf92f36226c8",
	"27394bdf0dad22ba2ae28b09120f61bbf50f7236d6f19c191a5778bea27b3398": "2346d5745c38d343bfc5104d237a4726d1c05ea5a3cf1531205f9af18f817333",
	"3367fd99f3d70a39084162df2353553abacf4ea8200ed5dbdd512f434fc6975d": "658bc2f6a6966e0da34b59e424ca93bddb26cc444b68f8702e181732883c8e65",
	"14b92ac5e5ef9dc04ea91339dcea034f7b971771d8f28f34251f1d9802c9dbbf": "9c544e37c1a1ef3f92790299b4a69b1e069db10c504c31238ac9ef5cd0801f90",
	"dc25e20f3fb8ba3151e0d520faf52646e8507bf98bd261dec51f4019dc3fb67f": "b51d0ae6b72fdf0e20ac4e5946387f8990add9fce9fe8699ea967597ebba0daf",
	"e1c1d4a253b31b70480c3944b8c9d04d7f41fc8ee1fedf889bde68240fd05f0b": "5daf35bde2ce8e7613ae64d5a3d0cb1866798aaa8c4ba9b287b6672581595ec8",
	"2d4c8befe882b36feabe1445b3260110bb94e60baf7b4b859c21efd6e708f2da": "46ef555e0a7c9ef31deb7bd97258e368385c4bf92ee9f6275f46281726226c29",
	"0578bc1eb40c9bb0b5a6fca444f1b5a0ecc8d5cd91e1c26d6d7481880594f0f5": "904e812c1a65225966e0a94e0bb28e16b9f2eedc235e0ee9454efd430452d9fb",
	"7f976f279b7e41fab5d023a1bf88b7053652e6f29c6ecc05870ac678286b8571": "190d57ea181bbdc783f0a0921a24748a5e88f3cf98b501dc9cdc38def8542842",
	"9ea4d10d904a10ff07528c53b8c7464b8997c00cbd618cfc49b76c8f5c4edbdb": "d2c35ed68ec0e6d5d682bf26b2fce44f3defccd1e8f32dd1fef7adc91e496e39",
	"b2e7088b6f9855b870c149ed908d0b8fa63eefe2a68eeaef3ea9a8d367de89d1": "cdb09b09d3be6c4c3cf4146261f6473153dae2d7509b0becf6d05b5b7645de99",
	"d21684bf7fa88e8522641a48165f3ae3ef12f5e63a8838e0d24ccbacbe6d19aa": "8ef620367d4717e138b6162b9d072bfd9da2c59c6dbdae4044c73cd5fa28a24e",
	"d06664aee8bf8592195a649abad0df8fd2a75b75670eb899e24d101ed0ebe6b3": "9f4eebe253d21ae48107f55390a9ea1c50d08141800e473a779d2d12c60b0ecb",
	"4952fd5f62abc74f9e7f7b175b6d3f4c82a2062cac1f56f4979565892f8033ce": "69f5a3f9fc534a0fdee9e82944c114be4cde7366b58617d1742d606f07d0b9d0",
	"b26b3accf232512924087fc810a3ace700d8ccfd75a392e7403471465bc1a886": "abe3e91c47618e45047bf19d7258fe7af9e599ea18e1814dffc661391863d38f",
	"f3a94f83f0a579d1a1b87f713d934df44e9b888216938667e7b2817aba71ef93": "75eabab3b5e3fe0a811bc2969f32716cc58bac7260b112380be45a23fc839939",
	"246de39853bcffeb885aa27df9e6df0e19ddfcee1967b29c2c81e86b386affde": "dac0889438a0949d1e2a73a3fbe26e2b6f2a0fe16d09b7db60e0dce30162bdb3",
	"d845aa8280ce96bfbf239ead9f82b8a759a5776f09aa95a74387186523493b83": "fb18373c3efec76633b3e6074a48b22ec2ec5445cfb118405ec62489fcf3003f",
	"5447772e29a35487fc42e6d10ba2b7ea6a7d77f99181b8a6f7ae25e964ff0994": "68f9a256b26491124c4c7dfc2c9d1dbfbd2f10614173cd4155b8890ca1e8564b",
	"492bb359de0a40e0b71c6b26ecee7d7f48a8fdc3d1f7446942681b6c0dcae822": "16c7e9b96cc295bccaf7ad46543a04aa798d89fd328a8c31583c1b32a7cdeb56",
	"d598c36ed84b4318dffbeb81efac93be2bfd22a76f5099eef8e6a5b508628a8a": "e50dd11156f59454b607d06b547b54833f719971a5b91c908a8933b4c7eacd63",
	"ebbbf69e9e74e4070ef0685f8d9b4d7bc443922ac93445bc9bda1567984bdda8": "1f0512c68448c8e74f1a02b62de1f19132507500dd0a0e7bd99f5bda34b850ec",
	"11412d05823e363ebc9cc308e74ce10d1c4a747fb43775ee6c7199df0aaebf0c": "c9e716999f62773b8a90f413db8af482dd16b702bf5f968fe86466d5fa6d40c5",
	"1c5883bc30f0caec912cc94011aa4ade2131cd63d21e652fdc8e49d62d79d73f": "222e97cb2597fa84200b2b499c0d589e286345df65f9e420d29f83b4858ad126",
	"b85a5cbb9c4f7ef75d5b346b91e0cdfc0b3b929503f94a47b28d5bf7e8a3ae98": "91744feec431a8c54dce4f4329cfc2e2522b1d4af9ceb406c674392fa3bb552e",
	"0213977d3c00a91de68904fb03ce3982e139200a2ce2e6f5332c9c3fb83743c5": "06cce790e037e4237d504f20db3ebc0f7f2a633523f632ad8e7ec24611e17e02",
	"957b200fcaadfa08492e96c80212624c1c4809a4984ce4df487bd26abed64fbb": "55996d9fae14510cd5f00186239dd2d8f459cb206b7a5199142e8e5e33a1da5b",
	"27d7d0bba986da1f1138ac71f0d39bfb0365a0e5d6779760c6dede8a6ece433b": "4c6ed820475fd3bdd7541ec8e99056ae0c205ae55213aa1c2c9b40183e8d8e75",
	"23e5e0b13eec7413116011b78a1a2bac0bc2070f02a6999d69a5c604e555b9b1": "cf46e1b0d2921becda921b6c1524bbe95c9177bcca8b43d72fc2d0af297bf676",
	"839c5ac4d9c29b9f3f8bdcf6fbd937deb16a00ccc8b658fcb7b513507232047b": "41e589655ecf43b13906c6f7933fed1d9a8d2470235d3bfdcb5711ecc7a32ba7",
	"cf0ea1a97befb4b32449272a5ec7e9a415d548be731b1f515cd90bd795c16be7": "b90a685bf3b962298e1538fb539a0e03778468f96d314b0b7df9c2edc4a9661c",
	"494a9b4326ffb2d22e53cd62945a349b2c205a2a1f3288ca8bee47446e535af8": "4057059b78d94b1577600312212d0a3d60def90cb04ac539052a4af8f7566f26",
	"ecb897a71752e96c1aa00915ad33863a9e4982a8358206331b40880cb4c55812": "0eb8e05cd9945ea3b2b9e120ebdca468fdccab13f4746f6cf4d894d72d29085c",
	"0e3d0d4fb918ea296b66ca096c772df4e8636de8b2ec8220d0d708bc30f2f815": "2c79c2c044d82cb3d3129c3062616f073e7086b30309ff860e02ff57be02c2f4",
	"aef48f91a3d6ffebc2dd0178d47de66cee222e48827adbf339d4197d5eee8af9": "f622017f1e6a494bda246aff96093ac23507e68dd7e74b3b64517aefb07d6faf",
	"ad9094b6024b5968ae189f3c9c63cb2a9d9cfbc3191994200e75fdaf09995085": "5744b0cd97fdb01d775054090443c54ec115829fbda33552a8d8a0c524ca411f",
	"356c9511de0a621f87cb6c98be7bc8ace90a7c8021ea02ba7cfe71f94c8348c3": "6c335ad3f6dff4d54bd433af2a90c38410abf6de502d19e3406e1c15287e5fcd",
	"762549b76f3947d668da23a4fcb70e1f96ad725eab0c56fa48a05129ad03e491": "a961fb2623c84de7a793d5cfd34274a6810e5a23f4639b9baf27713aea7df958",
	"f77436fd09c2248b79a8f54321e0332d247af489b26d4a4216d8eeb3596e8d4b": "fb977337f4992c566ca425a5e04de950a76a9ad3286214eeb58da950935c606c",
	"59c9398b48a8f91a5a298fd8d72ec77624ccb41311c25f07d7f126dfb9577e83": "6b2c64ba1a70de5b447ca9e203d1b82ed08d9392f56502c4e10c035907cb94e0",
	"46ae3d3d5c173f0b691250d7a3b24ba02731d7b9eae1808c655c0ca031b70cb6": "416e4d15084b01381449806977ff05ab1a76753213dcfb7639cdde67b33726e4",
	"d9e858482fc800892fc953c287c24ac31b6eced5a1a1c92b8f314752ed99d5cf": "3ca7f5d74fc0a21167a571e83b5cd94f9003d0af0c3355232a86d2e2eb90fa82",
	"74644f0993944ab08e93463c76c73d014cefc7387fd665a7cf4b5ed18ec3c543": "f1f70a9a0a4258d240f4332625426d09552891b5ef453f05a19a632a31d1a5d8",
	"86cbbefd4b1a4ebf84fa6c7429c278032bf79cef0ce00ec0bb4c7bbb081dde72": "c8457b471d4fc4c7b7b3f6df5ecc12f86465c45242eebf493873ed1df6a36a7a",
	"4c6b8e520cdaa328a47783e90c36301787279cc73960876d427c63871685af40": "949a9242d41654a473a1e34dc560788995f4453563bb44ef698e9a853d1b66a3",
	"c3918ece3f938448e2a573ec88b0a5cedd2449d6fb2af21804a1dd24fa9b4c29": "7f02557383ce05daa956149e0089b6f00ead01f6637e7802c33e8f92e5f7849a",
	"70f5c696678d9d07fe57b4e8cec64acedb86e8829b4dc3fa9b9bf86b55fda144": "2ead07eb45a5a0d2eaa48f81d0d16379379300c0e0f27067b58f6c130346652d",
	"4db2a4037007dab4cde28693f48d380683e28f9e442961f54266222c26527146": "9e76a65c03b9e6c1e8734a795b9247dc32269a9dfb649c8e1bbb01abf2e7be52",
	"b99fb0d60318c48d793840700789009ff34ff4632e788a7e71138bdae4772d59": "9b01a8125a14fc18868381ddbbb92bc1cc1e60b372b4cdc82363795f1e7a9f6d",
	"9a6bf8042eba089156e442f9c0480b102d1eb17e14f38800bad9c4a767e41bd5": "a0bc13e6828a2cf44a0950f2cef32e92c37b61842d74f30405ccc31536d711c3",
	"eab9ad7685b5ea28b4f71353f7fdbf445d67bee3758b76fa1635264bfe7b667f": "b5202df9f4a53d6e5b2f7808a13d48e4ccdcc875f41a79ac163e1b13676c2b9f",
	"46001500b12a3247e4a00fb32ac42f865f8bf320e01f55eee76aefe898b1cbb6": "29320eb182450804eab45b01a5e4e52709512eb8d84f3563d16b7e0739ed8078",
	"2e1f3558ebf4f5d4de110edeae316bcff40f7cf487a3deaefa35c125109b182e": "b8855c19a38999f283d9be6daa45147aef47cc6d35007673f62390c2e137e4e1",
	"9eba1b7eab9fc5603a15761cc906d09c30836651e1bd5ee411a5cc0f2ec6444f": "d710c159e6b12ebbab3cc04e1b4c01cc4f9a29ac6bdc22cf4d5d30002a42c615",
	"04f7ba291b44f838e8e784e76561455e9f068c0dedd750870e16169cdfb6a660": "15b76dd84aa54bc02711e9fda4c28d956ad186471457661830d4c8d2050c156e",
	"2d259a9cbe49eccd7878112e291e378fee7c08af0b443c598b1fbc091d7345fc": "4c874bd7919149ef0bdae5d664dec2dbb9e1211be67de71e45131012aff3c115",
	"7fa19dbf5c014d37485412d90b2d60e14b4778c969c0b5da253d2538795cb0e3": "36071696df8f7cac96baa5ee08624b95ef35a512f62715ccdbc5f3e524b101e7",
	"1f07c0828c425f6b24aa52130f950353ec80de83256b0b7d4b3e3cd32a049d45": "4bc7309831096d12c57393877593e1984194abbd897374d94c3e48e6e416629b",
	"57a58fc36f4bac957b118c24bacc4f3a56d05cb6136c0230206362f97c684333": "7c1babe9599a7d4aec8e231f74055d7380b478e7db0fdbd95b20012be15f8d4e",
	"ad1d3884c9335580ccea6cfb2a66cfb95f9bb77431cf5fda80c66028d796963e": "4a60da83d35e30a77782a3965e14945e3c1e7f61cc4686537c9a289495696e40",
	"5427ccbdb99a7eadfe271be34afe3a8101e304be7a5d8a7e8be57d3990e7c270": "c8b4b7ac7466b5574e9f5300ed05253db1fcd0f0fc58c756e73a776983256884",
	"9d87a3085035bba4b58bdad03ef61958f980e0377271721bd0cb3ff2d21b3f08": "59648006b91ccd41c4677dd5ea3454fac6f3c57c2a222f59a746eaceb5485179",
	"3e85d0329530a04c0132cb69c50b59103e7db405865de3dce41854d203778184": "52447c3d9d5636a0559920862077f5c3757a70ea984aaa83692afc4c7a427b8c",
	"9d7ec2c42bc4408668a2dd0e66290ba78ade3cf65c66cc2407f0325ea70104bc": "5a5567b8192a0f8b45e3502cc02174ccfdb9c5428cc1fce3535d37a1f91a7830",
	"8faa08a6a82097f0a28d2b29b7f2d21c9faaca529aa350902cb2de504358eb1d": "51bc07aed7bed5598f3c6458eded97742d21d523c94b56b6806931b94ea515e1",
	"6182da6d3e7bcee9d7a215edc04015aac1c6a9d4a84cef34e6c4fcbbd8d6cadf": "aa0b9e56f76ecf1172b8e9740b417c074e7905b863f469363cbb11f130b25605",
	"54aaa0e545a1e86d957d49a9b8901ade177d400bdc7d25292f647dea345a7757": "cc8fd5f8454bb932d42cdd0d6327868ef1351e673714688e1abbb84ee98b37e7",
	"4a0ddc369fe4cf60118bb5dc58729841c356c807ca9cc6c2cc62516576d65fb2": "aa5c7db2cf7ee595566e7bda3c959856de85931f3e69017fac55fb5ce73ff104",
	"c527f8bc0af93dba8b855beaced392eb53cfbe6cd39f5ccbee420dcf0365f4b7": "92f6ab4cb8db87b9357a0114c894531b797eae3e4a2afc09aab30bae9f50ec3c",
	"12e3d4dbc8fe04888d080c6223f17e64886a7d8eb458704c74efb13cc6ce340f": "2b0b61cbbd9082c9725b1649a965d812bee4c6892a183da22b943d23f6453fc0",
	"98bcb9acfebcbd666a423f9f4628a2946ce1939e9f3ba5653270774686d6df1b": "fd792b0121f3f371cdd076ef22274d0cba724f242aff8e0732811c9bcd5d0430",
	"400dfe02953ae617e0bcbe82962227c4db6ef31b76c3cf5b399f749c5b7a433a": "5b2104fe4c2a2953bb122b9a66112fd2090104a3b38d802b17cc662b5f312051",
	"3a7f8e6f1c409765a430acacc73cd11f5b93e207d58e16fd04c396bd1557d52a": "8766ee114339c22c6491286e7c6be74111f61c46128aed8bb0ec7caaac9f0d10",
	"d5af53561d99eb52af2b98b57d3fb0cc8ae4c6449ec6c89d8427201051a947a2": "1e729dcbaf719e43e8e0cc7f91bb925a55a327848a9ae457ce2d6d89b0148cbd",
	"dfcfbccb6d36fd86024fde98040cd9abcd984c4b88f992c8503bfb28daf4d259": "015d43f8232be2d86105aec38f03eb6e3ba4864cb2d5a743232b436d010c59ea",
	"c3c46410adfd1ebf8a3753d5d685fffd31a3c72c62118a678731e6292b2a426d": "8330e34e63c923f5aa81432c1baec1fc53a1de0c608c2bd3018dbe893fd58efe",
	"28304956c10f4a4e3358505ad784c4832a7ce484648d0aece5744b3b58334c02": "9849466d022029e24f3ff6b1afb459f5736d86e5be9f794fa8c14048365afee3",
	"ba7c57177d12c7a598bb1ac5ffc1c0ac52926f170da6baf438098b607d15f5c1": "639a2ac1788fe1b7c63a7e17c284958715849f544a2f1e0d2b0e8af6359cd3ec",
	"1ba702546714824a5fded7230ad1b728d4d33734c61d64c57120420dcf80d88d": "b7b18a25862d30e7f5370949ba862af190dc1e35f2a1b31591a0516aca998a35",
	"d28b7f9fb84b78d85c9cdde958419cbb8f761062f72e0de75c9817efbf1f000d": "6023600d72aea258809d1fc1716a4d05e7fff1b7648c3ed89857df22b3b4d0c8",
	"65da3f839b795bd57a52638767621ec9bc764b929a23ca26ebfc5cf49686b28e": "94b11e3d61c12825a052e770f93db186af32ddc8c0935c8455d84e1629283497",
	"e6b22fbcaec078f57898886d84de68d692c8074832683a43c8a9f12632724e42": "1d7ca16e9fb962cf2999a27be5f3b3c098309ce830e86f62e209c985f5f7b426",
	"f55022a07489bbee703c0ff0fd387caba741a7735406eeebe4f4b2d2e9df8f82": "9b0d0072d5bfd5545f3f968d98b9ada74a300a97cc1ef56226d1b24a57bad69d",
	"a133c2b154e8103b39bca963acb7f545838e06f784dcfaa761fc6ef2163b850e": "19d83ce352d4f7512ec57640b676b1bd4da75aa90b701c6fb2e6e7fb8fd66ee9",
	"8c2463893dd4130430d06e69dbb57a9ebf406640468fbd58c0a5e7232d666025": "112bc74ec88fc04436086c564262b7b8916431a062de33f5a14c7569463f86e3",
	"c6627206886fdbe2df4f8b14545aa2fd96416c609724ca766944876bd40dec9e": "862a9a47bbf511de129e754a395e7a3b1e5829eabd522809f195dd75c7e07dac",
	"e369448593bdb04c1ab9dd9d6536ea6ac91a49e1b8fdb8374ea1d896034267bc": "4c69c9ea4965ff10c0e7782fda9d69739f93573e0ce0d9f4822c29d113d25b4a",
	"f8cd366f926c8f27f9b359392caf304325759bd21a1a1b1e0a479a00ec38b896": "5dcaab6ebe213cc6c843ebce0490aa34f5d27eb0914e74cf001cb04ef74d65fb",
	"35ba9f06bcf68ffab52d4fddfab6be11a7eddd8cf94baf10500a289ea97031af": "0fa90dccc338c7976cbae20effa1efd2f09b8562800a4bf7ce21d2f606bf2b83",
	"9517462947506ffa0fdd680cab8112f7aab81339209ab1f1fd65d24ec6997f23": "1afe129ce85423aec3fc869c6652f72f88e50bb62e2f71e4ceb6908a80e139f5",
	"fbcbe808c5d9e76a5783b87ae440fb531b8a91da736083655861736fca1e1602": "e210cce927a03c26a76f79fd4cafa04d6f8f7d3449b85c1a1c88c6e0e7718abe",
	"b1ccc15b4e6c97e1d41ccaffc2c933368c03c9d0ef80c45f9fa41d013b23be22": "9ede5af182fea965c736035331d2b92008d7d7d5ea7cfedb05d6a5945b45c02c",
	"e97aa8e4a9bf64f8c489cc696df05a8c8ab1f7a81cfe81a60ca66016c8c3b010": "05b5e35f3fcef4895d35325a5c5fb9f95f71ec11f1b0f8cbd962581b6c89783e",
	"3d3f223aaeab0bc54ded3420c9949b1c871d1b0e245c3f53362cf99ccaadf337": "a9d97cca3b4b7b7d853209a9c66eb8dd66a1a7b8f1380cc068b836db55ef8eb4",
	"442c03d7ca8021cdb2037764fcd8e80e3aaa882373da2ad89db2ed0c62601288": "1888767a8095726fb8e151c5031ea175d099cc7942ba939570d685ab2ed630bd",
	"1134ad1cd02c99c266804e04b59bb291360f81707cf056df097acbc73cd23ec7": "c4935de16dffd1084f5becb01dd2353532b0db30ab6c05184c6352bf33a8961d",
	"e337a71e428312d0e1a6f6ee0b584191ac2b238d5f46bfe9d75fdb7e1acf80ec": "691d1c92bd925aeec761fdfa63461285cc89e6d0f519ef49b335f3a587754d57",
}

func readSnapshotFromTx(txn *badger.Txn, h crypto.Hash) (*common.VersionedTransaction, *common.SnapshotWithTopologicalOrder) {
	tx, final, err := readTransactionAndFinalization(txn, h)
	if err != nil {
		panic(err)
	}
	if final == "MISSING" {
		final = legacyNodeStateSnapshotMap[h.String()]
	}
	hash, err := crypto.HashFromString(final)
	if err != nil {
		panic(err)
	}
	snap, err := readSnapshotWithTopo(txn, hash)
	if err != nil {
		panic(err)
	}
	return tx, snap
}

func readLegacyPledgeNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStatePledging,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	return []*common.Node{node}
}

func readLegacyAcceptNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStateAccepted,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	nodes := []*common.Node{node}

	if len(tx.Inputs[0].Genesis) > 0 {
		return nodes
	}

	pn := readLegacyPledgeNodes(txn, tx.Inputs[0].Hash)
	return append(nodes, pn...)
}

func readLegacyCancelNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStateCancelled,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	nodes := []*common.Node{node}

	pn := readLegacyPledgeNodes(txn, tx.Inputs[0].Hash)
	return append(nodes, pn...)
}

func readLegacyRemoveNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStateRemoved,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	nodes := []*common.Node{node}

	an := readLegacyAcceptNodes(txn, tx.Inputs[0].Hash)
	return append(nodes, an...)
}
