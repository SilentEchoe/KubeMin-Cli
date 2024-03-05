package apiserver

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"k8s.io/client-go/kubernetes"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/infrastructure/clients"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/mysql"
)

// APIServer interface for call api server
type APIServer interface {
	Run(context.Context, chan error) error
	buildRestfulConfig() error //加载配置
}

type Server struct {
	cfg        *config.Config
	dataStore  datastore.DataStore
	KubeClient *kubernetes.Clientset
}

func New(cfg *config.Config) (a APIServer) {
	return &Server{
		cfg: cfg,
	}
}

func (s *Server) buildRestfulConfig() error {
	// 加载配置,有两种加载配置的方式，一种是读静态文件，一种是读ConfigMap

	return nil
}

// 构建Ioc
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
	// 注册datastore 至Ioc

	// init domain

	// interfaces
	// event

	return nil
}

func (s *Server) Run(context.Context, chan error) error {

	// 1. load configs
	if err := s.buildRestfulConfig(); err != nil {
		return fmt.Errorf("load config err %w", err)
	}
	// 3. registry gin router
	g := newKubeMinCliServer()
	g.router.Run()

	return nil
}

type KubeMinCliServer struct {
	router *gin.Engine //路由
}

func newKubeMinCliServer() *KubeMinCliServer {
	g := gin.Default()
	g.Use(cors.Default())
	return &KubeMinCliServer{
		router: g,
	}
}
