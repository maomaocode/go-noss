package main

import (
	"log"
	"nostr/cmd"
)

func main() {

	if err := cmd.RootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
