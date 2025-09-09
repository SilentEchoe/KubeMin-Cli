package main

import (
	"log"

	"KubeMin-Cli/cmd/server/app"
	"KubeMin-Cli/pkg/apiserver/workflow/traits"
)

func main() {
	traits.RegisterAllProcessors()
	cmd := app.NewAPIServerCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
