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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// 1. 加载ConfigMap配置
	if s.cfg.ConfigMapName != "" {
		clients.KubeConfigLocal()
		s.KubeClient = clients.GetKubeClient()
		configMaps, err := s.KubeClient.CoreV1().ConfigMaps("").Get(context.Background(), s.cfg.ConfigMapName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get configmap error %w", err)
		}
		// 2. 解析Config配置
		if err := s.cfg.ParseConfigMap(configMaps); err != nil {
			return fmt.Errorf("parse configmap error %w", err)
		}
		return nil
	}
	//3. 如果没有configmap则加载默认配置
	s.cfg = config.NewConfig()

	return nil
}

// 构建Ioc
func (s *Server) buildIocContainer() error {
	if s.cfg.ConfigInfo.LocalCluster {
		clients.KubeConfigLocal()
		s.KubeClient = clients.GetKubeClient()
	}

	var err error
	switch s.cfg.ConfigInfo.DatastoreType {
	case config.Mysql:
		s.dataStore, err = mysql.New(context.Background(), s.cfg.ConfigInfo.Datastore)
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

	// 2. load all components
	if err := s.buildIocContainer(); err != nil {
		return fmt.Errorf("build ioc container err %w", err)
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
