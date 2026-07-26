package main

import (
	"flag"
	"log"
	"os"

	"github.com/hamed/aistudio-api/internal/aistudio"
	"github.com/hamed/aistudio-api/internal/extensionbuild"
)

func main() {
	source := flag.String("source", "assets/extension", "extension source directory")
	config := flag.String("config", "config/upstream.yaml", "upstream config path")
	flag.Parse()

	if err := os.Setenv("AISTUDIO_UPSTREAM_CONFIG", *config); err != nil {
		log.Fatal(err)
	}
	upstream, err := aistudio.LoadUpstream()
	if err != nil {
		log.Fatal(err)
	}
	if err := extensionbuild.Build(*source, upstream); err != nil {
		log.Fatal(err)
	}
}
