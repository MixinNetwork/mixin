package main

import (
	"log"
)

func main() {
	err := StartHTTP()
	if err != nil {
		log.Println(err)
	}
}
