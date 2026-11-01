package main

import (
	_ "embed"
	"log"
	"os"

	"github.com/eryajf/Peck/internal/app"
	"github.com/eryajf/Peck/internal/update"
	"github.com/eryajf/Peck/internal/version"
)

//go:embed viva.toml
var manifest []byte

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--apply-update" {
		if err := update.RunApply(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}
	currentVersion, err := version.FromManifest(string(manifest))
	if err != nil {
		currentVersion = version.Current
	}
	if err := app.Run(os.Args[1:], currentVersion); err != nil {
		log.Fatal(err)
	}
}
