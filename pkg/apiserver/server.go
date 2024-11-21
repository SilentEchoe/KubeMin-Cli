package apiserver

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/event"
	"KubeMin-Cli/pkg/apiserver/interfaces/api"
	"KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/container"
	"context"
	"fmt"
	restfulSpec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	SwaggerConfigRoutePath = "/debug/apidocs.json"
)

type APIServer interface {
	Run(context.Context, chan error) error
	BuildRestfulConfig() (*restfulSpec.Config, error)
}

type restServer struct {
	webContainer  *restful.Container
	beanContainer *container.Container
	cfg           config.Config
	KubeClient    client.Client `inject:"kubeClient"`
	KubeConfig    *rest.Config  `inject:"kubeConfig"`
}

func (s *restServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var staticFilters []utils.FilterFunction
	// API 服务特性注册
	staticFilters = append(staticFilters, filters.Gzip)
}

func (s *restServer) BuildRestfulConfig() (*restfulSpec.Config, error) {
	//TODO implement me
	panic("implement me")
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
	if err := s.beanContainer.ProvideWithName("RestServer", s); err != nil {
		return fmt.Errorf("fail to provides the RestServer bean to the container: %w", err)
	}

	// domain
	if err := s.beanContainer.Provides(InitServiceBean(s.cfg)...); err != nil {
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

func (s *restServer) Run(ctx context.Context, errChan chan error) error {

	// 初始化IOC容器
	if err := s.buildIoCContainer(); err != nil {
		return err
	}

	// 注册API路由
	s.RegisterAPIRoute()

	return s.startHTTP(ctx)
}

// RegisterAPIRoute register the API route
func (s *restServer) RegisterAPIRoute() restfulSpec.Config {
	/* **************************************************************  */
	/* *************       Open API Route Group     *****************  */
	/* **************************************************************  */
	// Add container filter to enable CORS
	cors := restful.CrossOriginResourceSharing{
		ExposeHeaders:  []string{},
		AllowedHeaders: []string{"Content-Type", "Accept", "Authorization", "RefreshToken"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		CookiesAllowed: true,
		Container:      s.webContainer}
	s.webContainer.Filter(cors.Filter)

	// Add container filter to respond to OPTIONS
	s.webContainer.Filter(s.webContainer.OPTIONSFilter)
	s.webContainer.Filter(s.OPTIONSFilter)

	// Add request log
	s.webContainer.Filter(s.requestLog)

	// Register all custom api
	for _, handler := range api.GetRegisteredAPI() {
		s.webContainer.Add(handler.GetWebServiceRoute())
	}

	config := restfulSpec.Config{
		WebServices:                   s.webContainer.RegisteredWebServices(), // you control what services are visible
		APIPath:                       SwaggerConfigRoutePath,
		PostBuildSwaggerObjectHandler: enrichSwaggerObject}
	s.webContainer.Add(restfulSpec.NewOpenAPIService(config))
	return config
}

// InitServiceBean init all service instance
func InitServiceBean(c config.Config) []interface{} {
	return []interface{}{}
}

var needInitData []DataInit

// DataInit the service set that needs init data
type DataInit interface {
	Init(ctx context.Context) error
}

func (s *restServer) OPTIONSFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if req.Request.Method != "OPTIONS" {
		chain.ProcessFilter(req, resp)
		return
	}
	resp.AddHeader(restful.HEADER_AccessControlAllowCredentials, "true")
}

// TODO 请求日志
func (s *restServer) requestLog(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {

}

func enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "KubeMin-Cli api doc",
			Description: "KubeMin-Cli api doc",
			License: &spec.License{
				LicenseProps: spec.LicenseProps{
					Name: "MIT License 2.0",
					URL:  "https://github.com/SilentEchoe/KubeMin-Cli/blob/master/LICENSE",
				},
			},
			Version: "v1beta1",
		},
	}
}

func (s *restServer) startHTTP(ctx context.Context) error {
	// Start HTTP apiserver
	klog.Infof("HTTP APIs are being served on: %s, ctx: %s", s.cfg.BindAddr, ctx)
	server := &http.Server{Addr: s.cfg.BindAddr, Handler: s, ReadHeaderTimeout: 2 * time.Second}
	return server.ListenAndServe()
}
