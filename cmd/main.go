package main

import (
	"kubemin-cli/cmd/server/app"
	"kubemin-cli/pkg/apiserver/workflow/traits"
	"k8s.io/klog/v2"
)

func main() {
	traits.RegisterAllProcessors()
	cmd := app.NewAPIServerCommand()
	if err := cmd.Execute(); err != nil {
		klog.Fatalf("run command: %v", err)
	}
}
