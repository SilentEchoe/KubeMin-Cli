package apiserver

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/utils/container"
	"context"
	"fmt"
	"github.com/kubevela/velaux/pkg/server/domain/service"
	"github.com/kubevela/velaux/pkg/server/infrastructure/clients"
	"github.com/kubevela/velaux/pkg/server/interfaces/api"
	"github.com/kubevela/velaux/pkg/server/utils"
	"github.com/oam-dev/kubevela/pkg/utils/apply"
	"sigs.k8s.io/controller-runtime/pkg/client"

	restfulSpec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"k8s.io/client-go/rest"
)

/*
1.

*/

// APIServer interface for call api server
type APIServer interface {
	Run(context.Context, chan error) error
	BuildRestfulConfig() (*restfulSpec.Config, error)
}

// restServer rest server
type restServer struct {
	webContainer  *restful.Container
	beanContainer *container.Container
	cfg           config.Config
	//dataStore     datastore.DataStore 第一版暂时使用缓存
	KubeClient client.Client `inject:"kubeClient"`
	KubeConfig *rest.Config  `inject:"kubeConfig"`
}

// New create api server with config data
func New(cfg config.Config) (a APIServer) {
	s := &restServer{
		webContainer:  restful.NewContainer(),
		beanContainer: container.NewContainer(),
		cfg:           cfg,
	}
	return s
}

func (s *restServer) buildIoCContainer() error {
	// infrastructure
	// 注入Rest服务
	if err := s.beanContainer.ProvideWithName("RestServer", s); err != nil {
		return fmt.Errorf("fail to provides the RestServer bean to the container: %w", err)
	}
	// 设置KubeConfig
	err := clients.SetKubeConfig(s.cfg)
	if err != nil {
		return err
	}
	// 获取k8s的配置文件
	kubeConfig, err := clients.GetKubeConfig()
	if err != nil {
		return err
	}
	// 获取k8s的连接
	kubeClient, err := clients.GetKubeClient()
	if err != nil {
		return err
	}
	// 将这个k8s的连接与用户信息绑定在一起
	authClient := utils.NewAuthClient(kubeClient)

	// 将操作k8s的权限全都注入到IOC中
	if err := s.beanContainer.ProvideWithName("kubeClient", authClient); err != nil {
		return fmt.Errorf("fail to provides the kubeClient bean to the container: %w", err)
	}
	if err := s.beanContainer.ProvideWithName("kubeConfig", kubeConfig); err != nil {
		return fmt.Errorf("fail to provides the kubeConfig bean to the container: %w", err)
	}
	if err := s.beanContainer.ProvideWithName("apply", apply.NewAPIApplicator(authClient)); err != nil {
		return fmt.Errorf("fail to provides the apply bean to the container: %w", err)
	}
	// 这里应该是启动一个list-watch 的even
	factory := pkgconfig.NewConfigFactory(authClient)
	if err := s.beanContainer.ProvideWithName("configFactory", factory); err != nil {
		return fmt.Errorf("fail to provides the config factory bean to the container: %w", err)
	}
	// 注册
	addonStore := pkgaddon.NewRegistryDataStore(authClient)
	if err := s.beanContainer.ProvideWithName("registryDatastore", addonStore); err != nil {
		return fmt.Errorf("fail to provides the registry datastore bean to the container: %w", err)
	}
	// domain
	if err := s.beanContainer.Provides(service.InitServiceBean(s.cfg)...); err != nil {
		return fmt.Errorf("fail to provides the service bean to the container: %w", err)
	}

	// interfaces
	if err := s.beanContainer.Provides(api.InitAPIBean()...); err != nil {
		return fmt.Errorf("fail to provides the api bean to the container: %w", err)
	}

	// event
	if err := s.beanContainer.Provides(event.InitEvent()...); err != nil {
		return fmt.Errorf("fail to provides the event bean to the container: %w", err)
	}

	if err := s.beanContainer.Populate(); err != nil {
		return fmt.Errorf("fail to populate the bean container: %w", err)
	}
	return nil
}

func (s *restServer) Run(ctx context.Context, errors chan error) error {

	// build the Ioc Container
	if err := s.buildIoCContainer(); err != nil {
		return err
	}

}

func (s *restServer) BuildRestfulConfig() (*restfulSpec.Config, error) {
	//TODO implement me
	panic("implement me")
}
