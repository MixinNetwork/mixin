package common

import (
	"bytes"
	_ "embed"

	"github.com/klauspost/compress/zstd"
)

// FIXME have a cmd to make some random transactions, snapshots and
// or other data to compress, then make different dicts respectively
//
//go:embed data/zstd.dic
var zstdEmbed []byte

//go:embed data/snapshot.zstd
var snapshotDic []byte

func init() {
	zstdEncoder = NewZstdEncoder(2, zstdEmbed)
	zstdDecoder = NewZstdDecoder(2, zstdEmbed)

	zstdSnapshotEncoder = NewZstdEncoder(2, snapshotDic)
	zstdSnapshotDecoder = NewZstdDecoder(2, snapshotDic)
}

var (
	// zstd --train /tmp/zstd/* -o config/data/zstd.dic
	zstdEncoder         *zstd.Encoder
	zstdDecoder         *zstd.Decoder
	zstdSnapshotEncoder *zstd.Encoder
	zstdSnapshotDecoder *zstd.Decoder

	CompressionVersionZero   = []byte{0, 0, 0, 0}
	CompressionVersionLatest = CompressionVersionZero
)

func compressSnapshot(b []byte) []byte {
	return compressWith(b, zstdSnapshotEncoder)
}

func decompressSnapshot(b []byte) []byte {
	return decompressWith(b, zstdSnapshotDecoder)
}

func compressTransaction(b []byte) []byte {
	return compressWith(b, zstdEncoder)
}

func decompressTransaction(b []byte) []byte {
	return decompressWith(b, zstdDecoder)
}

func compressUTXO(b []byte) []byte {
	return compressWith(b, zstdEncoder)
}

func decompressUTXO(b []byte) []byte {
	return decompressWith(b, zstdDecoder)
}

func compressWith(b []byte, encoder *zstd.Encoder) []byte {
	b = encoder.EncodeAll(b, nil)
	return append(CompressionVersionLatest, b...)
}

func decompressWith(b []byte, decoder *zstd.Decoder) []byte {
	header := len(CompressionVersionLatest)
	if len(b) < header*2 {
		return nil
	}

	if !bytes.Equal(b[:header], CompressionVersionZero) {
		return nil
	}
	b, err := decoder.DecodeAll(b[header:], nil)
	if err != nil {
		return nil
	}
	return b
}
