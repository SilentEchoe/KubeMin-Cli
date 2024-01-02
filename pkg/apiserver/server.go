package apiserver

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/mysql"
	"context"
	"fmt"
	"k8s.io/klog"
)

// APIServer interface for call api server
type APIServer interface {
	Run(context.Context, chan error) error
}

// server rest server
type server struct {
	cfg       config.Config
	dataStore datastore.DataStore
}

// New create api server with config data
func New(cfg config.Config) APIServer {
	return nil
}

func (s *server) buildIoCContainer() error {
	ds, err := mysql.New(context.Background(), s.cfg.Datastore)
	if err != nil {
		return fmt.Errorf("create mysql datastore instance failure %w", err)
	}
	s.dataStore = ds
	return nil
}

func (s *server) Run(ctx context.Context, err chan error) error {

	// build the Ioc Container
	if err := s.buildIoCContainer(); err != nil {
		return err
	}

	return nil
}

func (s *server) startHTTP(ctx context.Context) error {
	// Start HTTP apiserver
	klog.Infof("HTTP APIs are being served on: %s, ctx: %s", s.cfg.BinAddr, ctx)

	return nil
}
