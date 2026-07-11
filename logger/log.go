package logger

import (
	"fmt"
	"log"
	"regexp"
	"sync/atomic"
)

const (
	ERROR   = 1
	INFO    = 2
	VERBOSE = 3
	DEBUG   = 7
)

// FIXME GLOBAL VARIABLES

var (
	level  atomic.Int32
	filter atomic.Pointer[regexp.Regexp]
)

func SetLevel(l int) {
	level.Store(int32(l))
}

func SetFilter(pattern string) error {
	if pattern == "" {
		filter.Store(nil)
		return nil
	}
	// https://github.com/google/re2/wiki/Syntax
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	filter.Store(reg)
	return nil
}

func Println(v ...any) {
	if level.Load() >= INFO {
		log.Println(v...)
	}
}

func Printf(format string, v ...any) {
	if level.Load() >= INFO {
		log.Printf(format, v...)
	}
}

func Verbosef(format string, v ...any) {
	printfAtLevel(VERBOSE, format, v...)
}

func Debugf(format string, v ...any) {
	printfAtLevel(DEBUG, format, v...)
}

func printfAtLevel(l int, format string, v ...any) {
	if level.Load() < int32(l) {
		return
	}
	out := filterOutput(format, v...)
	if out == "" {
		return
	}
	log.Print(out)
}

func filterOutput(format string, v ...any) string {
	out := fmt.Sprintf(format, v...)
	reg := filter.Load()
	if reg == nil || reg.MatchString(out) {
		return out
	}
	return ""
}
