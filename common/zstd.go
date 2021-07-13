package common

import (
	"runtime"

	"github.com/klauspost/compress/zstd"
)

func NewZstdDecoder(ccr int) *zstd.Decoder {
	if ccr > runtime.GOMAXPROCS(0) {
		ccr = runtime.GOMAXPROCS(0)
	}
	opts := []zstd.DOption{
		zstd.WithDecoderDicts(ZstdEmbed),
		zstd.WithDecoderConcurrency(ccr),
		zstd.WithDecoderLowmem(true),
		zstd.WithDecoderMaxMemory(1024 * 1024 * 16),
	}
	dec, err := zstd.NewReader(nil, opts...)
	if err != nil {
		panic(err)
	}
	return dec
}

func NewZstdEncoder(ccr int) *zstd.Encoder {
	if ccr > runtime.GOMAXPROCS(0) {
		ccr = runtime.GOMAXPROCS(0)
	}
	opts := []zstd.EOption{
		zstd.WithEncoderConcurrency(ccr),
		zstd.WithEncoderDict(ZstdEmbed),
		zstd.WithEncoderLevel(3),
		zstd.WithWindowSize(1024 * 32),
		zstd.WithEncoderCRC(false),
	}
	enc, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		panic(err)
	}
	return enc
}
