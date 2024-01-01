package apiserver

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/mysql"
	"context"
	"fmt"
	"kubevela/pkg/apiserver/infrastructure/clients"
)

// APIServer interface for call api server
type APIServer interface {
	Run(context.Context, chan error) error
	BuildRestfulConfig() (*restfulSpec.Config, error)
}

// restServer rest server
type restServer struct {
	cfg       config.Config
	dataStore datastore.DataStore
}

// New create api server with config data
func New(cfg config.Config) (a APIServer) {
	s := &restServer{
		cfg: cfg,
	}
	return s
}

func (s *restServer) buildIoCContainer() error {
	err := clients.SetKubeConfig(s.cfg)
	if err != nil {
		return err
	}

	ds, err := mysql.New(context.Background(), s.cfg.Datastore)
	if err != nil {
		return fmt.Errorf("create mysql datastore instance failure %w", err)
	}
	s.dataStore = ds
	return nil
}
