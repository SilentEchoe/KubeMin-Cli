package options

import "KubeMin-Cli/pkg/apiserver/config"

// ServerRunOptions contains everything necessary to create and run api server
type ServerRunOptions struct {
	GenericServerRunOptions *config.Config
}

// NewServerRunOptions creates a new ServerRunOptions object with default parameters
func NewServerRunOptions() *ServerRunOptions {
	s := &ServerRunOptions{
		GenericServerRunOptions: config.NewConfig(),
	}
	return s
}
