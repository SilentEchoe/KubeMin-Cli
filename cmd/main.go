package main

import (
	"KubeMin-Cli/cmd/server/app"
	"log"
)

func main() {
	cmd := app.NewAPIServerCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
