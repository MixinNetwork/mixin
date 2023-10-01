package common

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestSnapshot(t *testing.T) {
	require := require.New(t)

	s := &SnapshotWithTopologicalOrder{Snapshot: &Snapshot{Version: SnapshotVersionCommonEncoding}}
	s.Transactions = []crypto.Hash{crypto.Blake3Hash([]byte("tx-test-id"))}
	s.References = &RoundLink{
		Self:     crypto.Blake3Hash([]byte("self-reference")),
		External: crypto.Blake3Hash([]byte("external-reference")),
	}
	require.Len(s.versionedPayload(), 160)
	require.Equal("77770002000000000000000000000000000000000000000000000000000000000000000000000000000000000002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc00000000000000000000000000000000", hex.EncodeToString(s.versionedPayload()))
	require.Equal("16daf334f9aa4e476218c2c8ccd705ad53e0d67eebb2ad2847dc984abb0aae5c", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	s, err := NewDecoder(s.versionedPayload()).DecodeSnapshotWithTopo()
	require.Nil(err)
	require.Equal("77770002000000000000000000000000000000000000000000000000000000000000000000000000000000000002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc00000000000000000000000000000000", hex.EncodeToString(s.versionedPayload()))
	require.Equal("16daf334f9aa4e476218c2c8ccd705ad53e0d67eebb2ad2847dc984abb0aae5c", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	require.Equal("b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a", s.References.Self.String())
	require.Equal("0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb1", s.References.External.String())
	require.Equal("d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc", s.Transactions[0].String())

	s.NodeId = crypto.Blake3Hash([]byte("node-test-id"))
	s.RoundNumber = uint64(123)
	s.Timestamp = 1663669260746463409
	require.Len(s.versionedPayload(), 160)
	require.Equal("77770002088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde000000000000007b0002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc17168a60ce8798b10000000000000000", hex.EncodeToString(s.versionedPayload()))
	require.Equal("e2819adf40b6c92e0155bdb2ac721c6eb14e442633fd59fb7cb7fb03917d02f8", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	s, err = NewDecoder(s.versionedPayload()).DecodeSnapshotWithTopo()
	require.Nil(err)
	require.Equal("77770002088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde000000000000007b0002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc17168a60ce8798b10000000000000000", hex.EncodeToString(s.versionedPayload()))
	require.Equal("e2819adf40b6c92e0155bdb2ac721c6eb14e442633fd59fb7cb7fb03917d02f8", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	require.Equal("b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a", s.References.Self.String())
	require.Equal("0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb1", s.References.External.String())
	require.Equal("088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde", s.NodeId.String())
	require.Equal(uint64(123), s.RoundNumber)
	require.Equal(uint64(1663669260746463409), s.Timestamp)

	var sig crypto.CosiSignature
	sig.Mask ^= (1 << uint64(0))
	copy(sig.Signature[:], bytes.Repeat([]byte{1, 2, 3, 4}, 16))
	s.Signature = &sig
	require.Len(s.versionedPayload(), 160)
	require.Equal("77770002088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde000000000000007b0002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc17168a60ce8798b10000000000000000", hex.EncodeToString(s.versionedPayload()))
	require.Equal("e2819adf40b6c92e0155bdb2ac721c6eb14e442633fd59fb7cb7fb03917d02f8", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	s, err = NewDecoder(s.versionedPayload()).DecodeSnapshotWithTopo()
	require.Nil(err)
	require.Equal("77770002088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde000000000000007b0002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc17168a60ce8798b10000000000000000", hex.EncodeToString(s.versionedPayload()))
	require.Equal("e2819adf40b6c92e0155bdb2ac721c6eb14e442633fd59fb7cb7fb03917d02f8", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	require.Equal("b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a", s.References.Self.String())
	require.Equal("0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb1", s.References.External.String())
	require.Equal("088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde", s.NodeId.String())
	require.Equal(uint64(123), s.RoundNumber)
	require.Equal(uint64(1663669260746463409), s.Timestamp)
	require.Equal("d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc", s.Transactions[0].String())
	require.Nil(s.Signature)

	s.Signature = &sig
	s.TopologicalOrder = 345
	require.Len(s.VersionedCompressMarshal(), 190)
	require.Equal("0000000028b52ffd0300c118533c6d0500040a77770002088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde007b0002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc17168a60ce8798b100010102030400000000000001590300d965b57618a60c8e0c", hex.EncodeToString(s.VersionedCompressMarshal()))
	require.Equal("e2819adf40b6c92e0155bdb2ac721c6eb14e442633fd59fb7cb7fb03917d02f8", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	_, err = NewDecoder(s.VersionedCompressMarshal()).DecodeSnapshotWithTopo()
	require.NotNil(err)
	s, err = NewDecoder(decompress(s.VersionedCompressMarshal())).DecodeSnapshotWithTopo()
	require.Nil(err)
	require.Equal("0000000028b52ffd0300c118533c6d0500040a77770002088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde007b0002b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb10001d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc17168a60ce8798b100010102030400000000000001590300d965b57618a60c8e0c", hex.EncodeToString(s.VersionedCompressMarshal()))
	require.Equal("e2819adf40b6c92e0155bdb2ac721c6eb14e442633fd59fb7cb7fb03917d02f8", s.PayloadHash().String())
	require.Equal(crypto.Blake3Hash(s.versionedPayload()).String(), s.PayloadHash().String())
	require.Equal("b7342ffb374824d69674054486e71bb8b575a4d961b65ffff647a8e1696f579a", s.References.Self.String())
	require.Equal("0552038ee8ce7c8b0efba019a7c36e86f1b70069553bbb187cfd8e3ca5f14fb1", s.References.External.String())
	require.Equal("088ca294310ed5529cf86b530c8d409d7cdef3c2e352ceeb3ff55b529431fdde", s.NodeId.String())
	require.Equal(uint64(123), s.RoundNumber)
	require.Equal(uint64(1663669260746463409), s.Timestamp)
	require.Equal("d694818d674f347b36b0efd75332eadfa73723cd0fb6152da778b91baf9719cc", s.Transactions[0].String())
	require.Equal(uint64(1), s.Signature.Mask)
	require.Equal([]int{0}, s.Signature.Keys())
	require.Equal("01020304010203040102030401020304010203040102030401020304010203040102030401020304010203040102030401020304010203040102030401020304", s.Signature.Signature.String())
	require.Equal("010203040102030401020304010203040102030401020304010203040102030401020304010203040102030401020304010203040102030401020304010203040000000000000001", s.Signature.String())
	require.Equal(uint64(345), s.TopologicalOrder)
}

func BenchmarkSnapshotMarshal(b *testing.B) {
	s := &SnapshotWithTopologicalOrder{Snapshot: &Snapshot{Version: SnapshotVersionCommonEncoding}}
	s.Transactions = []crypto.Hash{crypto.Blake3Hash([]byte("tx-test-id"))}

	s.NodeId = crypto.Blake3Hash([]byte("node-test-id"))
	s.RoundNumber = 123

	s.References = &RoundLink{
		Self:     crypto.Blake3Hash([]byte("self-reference")),
		External: crypto.Blake3Hash([]byte("external-reference")),
	}

	s.TopologicalOrder = 456

	var sig crypto.CosiSignature
	sig.Mask ^= (1 << uint64(0))
	copy(sig.Signature[:], bytes.Repeat([]byte{1, 2, 3, 4}, 16))
	s.Signature = &sig
	benchmarkSnapshot(b, s)
}

func benchmarkSnapshot(b *testing.B, s *SnapshotWithTopologicalOrder) {
	for _, n := range []int{1, 4, 16, 64, 256} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf := s.VersionedCompressMarshal()
				s, err := DecompressUnmarshalVersionedSnapshot(buf)
				if err != nil {
					b.Fatal("unmarshal snapshot")
				}
				s.PayloadHash()
			}
		})
	}
}
