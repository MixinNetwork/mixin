package logger

import "log"

const (
	ERROR   = 1
	INFO    = 2
	VERBOSE = 3
	DEBUG   = 7
)

var level int

func Init(l int) {
	level = l
}

func Println(v ...interface{}) {
	if level >= INFO {
		log.Println(v...)
	}
}

func Printf(format string, v ...interface{}) {
	if level >= INFO {
		log.Printf(format, v...)
	}
}

func Verbosef(format string, v ...interface{}) {
	if level >= VERBOSE {
		log.Printf(format, v...)
	}
}
