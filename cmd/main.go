package main

import (
	"KubeMin-Cli/cmd/server/app"
	"KubeMin-Cli/pkg/apiserver/workflow/traits"
	"log"
)

func main() {
	traits.RegisterAllProcessors()
	cmd := app.NewAPIServerCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
