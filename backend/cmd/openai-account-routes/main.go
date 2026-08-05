package main

import (
	"log"
	"os"

	"github.com/Wei-Shaw/sub2api/internal/cli/openaiaccountroutes"
)

func main() {
	if err := openaiaccountroutes.Run(openaiaccountroutes.Name, os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}
