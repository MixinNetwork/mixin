package util

import "io"

func CloseOrPanic(c io.Closer) {
	err := c.Close()
	if err != nil {
		panic(err)
	}
}
