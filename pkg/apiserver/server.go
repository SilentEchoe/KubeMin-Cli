package apiserver

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/event"
	"KubeMin-Cli/pkg/apiserver/infrastructure/clients"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/mysql"
	"KubeMin-Cli/pkg/apiserver/interfaces/api"
	pkgUtils "KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/container"
	"KubeMin-Cli/pkg/apiserver/utils/filters"
	"context"
	"fmt"
	restfulSpec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

/*

 */

// APIServer interface for call api server
type APIServer interface {
	Run(context.Context, chan error) error
	BuildRestfulConfig() (*restfulSpec.Config, error)
}

// restServer rest server
type restServer struct {
	//webContainer  *restful.Container
	webContainer  *gin.Engine
	beanContainer *container.Container
	cfg           config.Config
	dataStore     datastore.DataStore
	KubeClient    client.Client `inject:"kubeClient"` //inject 是注入IOC的name，如果tag中包含inject 那么必须有对应的容器注入服务,必须大写，小写会无法访问
}

// New create api server with config data
func New(cfg config.Config) (a APIServer) {
	s := &restServer{
		webContainer:  gin.New(),
		beanContainer: container.NewContainer(),
		cfg:           cfg,
	}
	return s
}

func (s *restServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	var staticFilters []pkgUtils.FilterFunction
	// 开启 GZIP
	staticFilters = append(staticFilters, filters.Gzip)
	for _, pre := range api.GetAPIPrefix() {
		if strings.HasPrefix(req.URL.Path, pre) {
			s.webContainer.ServeHTTP(res, req)
			return
		}
	}
	req.URL.Path = "/"
}

func (s *restServer) buildIoCContainer() error {
	// infrastructure
	if err := s.beanContainer.ProvideWithName("RestServer", s); err != nil {
		return fmt.Errorf("fail to provides the RestServer bean to the container: %w", err)
	}
	// 设置KubeConfig
	err := clients.SetKubeConfig(s.cfg)
	if err != nil {
		return err
	}
	// 获取k8s的连接
	kubeClient, err := clients.GetKubeClient()
	if err != nil {
		return err
	}
	// 将这个k8s的连接与用户信息绑定在一起
	authClient := pkgUtils.NewAuthClient(kubeClient)

	var ds datastore.DataStore
	switch s.cfg.Datastore.Type {
	case "mysql":
		ds, err = mysql.New(context.Background(), s.cfg.Datastore)
		if err != nil {
			return fmt.Errorf("create mysql datastore instance failure %w", err)
		}
	default:
		return fmt.Errorf("not support datastore type %s", s.cfg.Datastore.Type)
	}
	s.dataStore = ds

	// 将db 注入到IOC中
	if err := s.beanContainer.ProvideWithName("datastore", s.dataStore); err != nil {
		return fmt.Errorf("fail to provides the datastore bean to the container: %w", err)
	}

	// 将操作k8s的权限全都注入到IOC中
	if err := s.beanContainer.ProvideWithName("kubeClient", authClient); err != nil {
		return fmt.Errorf("fail to provides the kubeClient bean to the container: %w", err)
	}

	// domain
	if err := s.beanContainer.Provides(service.InitServiceBean(s.cfg)...); err != nil {
		return fmt.Errorf("fail to provides the service bean to the container: %w", err)
	}

	// interfaces
	//if err := s.beanContainer.Provides(api.InitAPIBean()...); err != nil {
	//	return fmt.Errorf("fail to provides the api bean to the container: %w", err)
	//}
	if err := s.beanContainer.Provides(api.NewInitAPIBean()...); err != nil {
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

	s.RegisterAPIRoute()

	return s.startHTTP(ctx)
}

func (s *restServer) RegisterAPIRoute() {
	// 初始化中间件
	s.webContainer.Use(gin.Recovery())
	// 获取所有注册的API
	apis := api.NewGetRegisteredAPI()
	// 为每个API前缀创建路由组
	for _, prefix := range api.GetAPIPrefix() {
		group := s.webContainer.Group(prefix)

		// 注册所有API到当前前缀组
		for _, api := range apis {
			api.RegisterRoutes(group)
		}
	}

}

func (s *restServer) requestLog(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if req.HeaderParameter("Upgrade") == "websocket" && req.HeaderParameter("Connection") == "Upgrade" {
		chain.ProcessFilter(req, resp)
		return
	}
	start := time.Now()
	c := pkgUtils.NewResponseCapture(resp.ResponseWriter)
	resp.ResponseWriter = c
	chain.ProcessFilter(req, resp)
	takeTime := time.Since(start)
	klog.InfoS("request log",
		"clientIP", pkgUtils.Sanitize(pkgUtils.ClientIP(req.Request)),
		"path", pkgUtils.Sanitize(req.Request.URL.Path),
		"method", req.Request.Method,
		"status", c.StatusCode(),
		"time", takeTime.String(),
		"responseSize", len(c.Bytes()),
	)
}

func (s *restServer) OPTIONSFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if req.Request.Method != "OPTIONS" {
		chain.ProcessFilter(req, resp)
		return
	}
	resp.AddHeader(restful.HEADER_AccessControlAllowCredentials, "true")
}

func (s *restServer) BuildRestfulConfig() (*restfulSpec.Config, error) {
	if err := s.buildIoCContainer(); err != nil {
		return nil, err
	}

	s.RegisterAPIRoute()
	return nil, nil
}

func (s *restServer) startHTTP(ctx context.Context) error {
	// Start HTTP apiserver
	klog.Infof("HTTP APIs are being served on: %s, ctx: %s", s.cfg.BindAddr, ctx)
	server := &http.Server{Addr: s.cfg.BindAddr, Handler: s, ReadHeaderTimeout: 2 * time.Second}
	return server.ListenAndServe()
}
