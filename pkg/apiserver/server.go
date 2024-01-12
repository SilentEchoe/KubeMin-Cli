package apiserver

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/infrastructure/clients"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/mysql"
)

// APIServer interface for call api server
type APIServer interface {
	Run(context.Context, chan error) error
	BuildRestfulConfig() error //加载配置
}

type Server struct {
	cfg        config.Config
	dataStore  datastore.DataStore
	KubeClient *kubernetes.Clientset
}

func New(cfg config.Config) (a APIServer) {
	return &Server{
		cfg: cfg,
	}
}

func (s *Server) Run(context.Context, chan error) error {
	// 1. build the Ioc Container
	// 2. init database
	// 3. 注册服务路由

	return nil
}

func (s *Server) BuildRestfulConfig() error {
	// 加载配置,有两种加载配置的方式，一种是读静态文件，一种是读ConfigMap

	return nil
}

func (s *Server) buildIocContainer() error {
	if s.cfg.LocalCluster {
		clients.KubeConfigLocal()
		s.KubeClient = clients.GetKubeClient()
	}

	var err error
	switch s.cfg.DatastoreType {
	case config.Mysql:
		s.dataStore, err = mysql.New(context.Background(), s.cfg.Datastore)
		if err != nil {
			return fmt.Errorf("create mysql datastore instance failure %w", err)
		}
	}

	return nil
}
