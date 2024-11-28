package apiserver

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/service"
)

const (
	// BuildPublicRoutePath the route prefix to request the build static files.
	BuildPublicRoutePath = "/public/build"
	// BuildPublicPath the route prefix to request the build static files.
	BuildPublicPath = "public/build"
)

type APIServer interface {
	Run(context.Context, chan error) error
}

// restServer rest server
type restServer struct {
	cfg        config.Config
	KubeClient client.Client `inject:"kubeClient"`
}

// New create api server with config data
func New(cfg config.Config) (a APIServer) {
	s := &restServer{
		cfg: cfg,
	}
	return s
}

func (s restServer) Run(ctx context.Context, errors chan error) error {

	// TODO init cache

	// TODO init workflow

	// TODO

	return s.startHTTP(ctx)
}

func (s *restServer) startHTTP(ctx context.Context) error {
	// Start HTTP apiserver
	klog.Infof("HTTP APIs are being served on: %s, ctx: %s", s.cfg.BindAddr, ctx)

	//Initialize the gin framework service
	g := gin.Default()
	g.Use(cors.Default())
	// 注册各类服务
	service.NewApplicationService()

	// Inject the router to the server
	v1 := g.Group("/api/v1")
	v1.Handle(http.MethodPost, "/cluster")

	//server := &http.Server{Addr: s.cfg.BindAddr, Handler: s, ReadHeaderTimeout: 2 * time.Second}
	return g.Run(s.cfg.BindAddr)

}
