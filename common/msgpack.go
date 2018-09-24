package common

import "github.com/vmihailenco/msgpack"

func MsgpackMarshalPanic(val interface{}) []byte {
	data, err := msgpack.Marshal(val)
	if err != nil {
		panic(err)
	}
	return data
}
