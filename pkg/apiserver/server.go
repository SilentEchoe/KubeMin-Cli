package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/event"
	"KubeMin-Cli/pkg/apiserver/infrastructure/clients"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/mysql"
	"KubeMin-Cli/pkg/apiserver/interfaces/api"
	"KubeMin-Cli/pkg/apiserver/interfaces/api/middleware"
	qpkg "KubeMin-Cli/pkg/apiserver/queue"
	"KubeMin-Cli/pkg/apiserver/utils/cache"
	"KubeMin-Cli/pkg/apiserver/utils/container"
	"KubeMin-Cli/pkg/apiserver/utils/kube"
	"os"
)

// APIServer interface for call api server
type APIServer interface {
	Run(context.Context, chan error) error
}

// restServer rest server
type restServer struct {
	webContainer   *gin.Engine
	beanContainer  *container.Container
	cfg            config.Config
	dataStore      datastore.DataStore
	cache          cache.ICache
	KubeClient     *kubernetes.Clientset `inject:"kubeClient"` //inject 是注入IOC的name，如果tag中包含inject 那么必须有对应的容器注入服务,必须大写，小写会无法访问
	KubeConfig     *rest.Config          `inject:"kubeConfig"`
	Queue          qpkg.Queue            `inject:"queue"`
	workersStarted bool
	workersCancel  context.CancelFunc
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
	for _, pre := range api.GetAPIPrefix() {
		if strings.HasPrefix(req.URL.Path, pre) {
			s.webContainer.ServeHTTP(res, req)
			return
		}
	}
	req.URL.Path = "/"
	s.webContainer.ServeHTTP(res, req)
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

	// Initialize cache implementation. Default to in-memory cache.
	// Note: current ICache only supports memory; redis variant is used for messaging/locks.
	iCache := cache.New(false, cache.CacheTypeMem)

	// 将db 注入到IOC中
	if err := s.beanContainer.ProvideWithName("datastore", s.dataStore); err != nil {
		return fmt.Errorf("fail to provides the datastore bean to the container: %w", err)
	}

	if err := s.beanContainer.ProvideWithName("cache", iCache); err != nil {
		return fmt.Errorf("fail to provides the cache bean to the container: %w", err)
	}

	// messaging broker removed; we use unified Queue abstraction instead

	// Initialize work queue (Redis Streams if configured; noop otherwise)
	var q qpkg.Queue
	streamKey := s.dispatchTopic()
	switch s.cfg.Messaging.Type {
	case "redis":
		addr := fmt.Sprintf("%s:%d", s.cfg.Cache.CacheHost, s.cfg.Cache.CacheProt)
		db := int(s.cfg.Cache.CacheDB)
		user := s.cfg.Cache.UserName
		pass := s.cfg.Cache.Password
		if rq, err := qpkg.NewRedisStreams(addr, user, pass, db, streamKey); err != nil {
			klog.Warningf("init redis streams failed, falling back to noop: %v", err)
			q = &qpkg.NoopQueue{}
		} else {
			q = rq
		}
	default:
		q = &qpkg.NoopQueue{}
	}
	// 注入消息队列
	if err := s.beanContainer.ProvideWithName("queue", q); err != nil {
		return fmt.Errorf("fail to provides the queue bean to the container: %w", err)
	}

	// 将操作k8s的权限全都注入到IOC中
	if err := s.beanContainer.ProvideWithName("kubeClient", kubeClient); err != nil {
		return fmt.Errorf("fail to provides the kubeClient bean to the container: %w", err)
	}

	if err := s.beanContainer.ProvideWithName("kubeConfig", kubeConfig); err != nil {
		return fmt.Errorf("fail to provides the kubeConfig bean to the container: %w", err)
	}

	// provide config for downstream components that need it (inject by type)
	if err := s.beanContainer.Provides(&s.cfg); err != nil {
		return fmt.Errorf("fail to provides the config bean to the container: %w", err)
	}

	// domain
	services := service.InitServiceBean(s.cfg)
	for _, svc := range services {
		if err := s.beanContainer.Provides(svc); err != nil {
			return fmt.Errorf("fail to provides the service bean to the container: %w", err)
		}
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

// dispatchTopic 计算用于工作流分发的Redis Streams键
func (s *restServer) dispatchTopic() string {
	prefix := s.cfg.Messaging.ChannelPrefix
	if prefix == "" {
		prefix = "kubemin"
	}
	return fmt.Sprintf("%s.workflow.dispatch", prefix)
}

func (s *restServer) RegisterAPIRoute() {
	// 初始化中间件
	s.webContainer.Use(gin.Recovery())

	// Always enable request logging
	s.webContainer.Use(middleware.Logging())

	// Enable tracing middleware if configured
	if s.cfg.EnableTracing {
		s.webContainer.Use(otelgin.Middleware("kubemin-cli"))
	}

	// Enable gzip compression for responses
	s.webContainer.Use(middleware.Gzip())

	// 获取所有注册的API
	apis := api.GetRegisteredAPI()
	// 为每个API前缀创建路由组
	for _, prefix := range api.GetAPIPrefix() {
		group := s.webContainer.Group(prefix)
		for _, api := range apis {
			api.RegisterRoutes(group)
		}
	}

}

func (s *restServer) startHTTP(ctx context.Context) error {
	// Start HTTP appserver
	klog.Infof("HTTP APIs are being served on: %s, ctx: %s", s.cfg.BindAddr, ctx)
	server := &http.Server{Addr: s.cfg.BindAddr, Handler: s, ReadHeaderTimeout: 2 * time.Second}
	return server.ListenAndServe()
}

func (s *restServer) Run(ctx context.Context, errChan chan error) error {
	// build the Ioc Container
	if err := s.buildIoCContainer(); err != nil {
		return err
	}

	// Enforce odd replica count at startup: if even, exit so that last-started pod exits
	cnt := kube.DetectReplicaCount(ctx, s.KubeClient)
	if cnt%2 == 0 {
		klog.Errorf("replica count is even (%d); exiting to maintain odd replicas", cnt)
		os.Exit(0)
		return nil
	}

	s.RegisterAPIRoute()

	// 服务选举
	l, err := s.setupLeaderElection(errChan)
	if err != nil {
		return err
	}

	go func() {
		leaderelection.RunOrDie(ctx, *l)
	}()

	return s.startHTTP(ctx)
}

func (s *restServer) setupLeaderElection(errChan chan error) (*leaderelection.LeaderElectionConfig, error) {
	restCfg := ctrl.GetConfigOrDie()
	ns := s.cfg.LeaderConfig.Namespace
	if ns == "" {
		ns = config.NAMESPACE
	}
	rl, err := resourcelock.NewFromKubeconfig(resourcelock.LeasesResourceLock, ns, s.cfg.LeaderConfig.LockName, resourcelock.ResourceLockConfig{
		Identity: s.cfg.LeaderConfig.ID,
	}, restCfg, time.Second*10)
	if err != nil {
		klog.ErrorS(err, "Unable to setup the resource lock")
		return nil, err
	}
	return &leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: time.Second * 15,
		RenewDeadline: time.Second * 10,
		RetryPeriod:   time.Second * 2,
		Callbacks: leaderelection.LeaderCallbacks{
            OnStartedLeading: func(ctx context.Context) {
                s.onStartedLeading(ctx, errChan)
            },
			OnStoppedLeading: func() {
				if s.cfg.ExitOnLostLeader {
					errChan <- fmt.Errorf("leader lost %s", s.cfg.LeaderConfig.ID)
				}
			},
			OnNewLeader: func(identity string) {
				if identity == s.cfg.LeaderConfig.ID {
					return
				}
				klog.Infof("new leader elected: %s", identity)
				// we are follower now; if distributed (>=3), ensure workers started
				cnt := kube.DetectReplicaCount(context.Background(), s.KubeClient)
				if cnt >= 3 {
					s.startWorkers(context.Background(), errChan)
				}
			},
		},
		ReleaseOnCancel: true,
	}, nil
}

func (s *restServer) startWorkers(ctx context.Context, errChan chan error) {
	if s.workersStarted {
		return
	}
	s.workersStarted = true
	var wctx context.Context
	wctx, s.workersCancel = context.WithCancel(ctx)
	go event.StartWorkerSubscriber(wctx, errChan)
}

func (s *restServer) stopWorkers() {
    if !s.workersStarted {
        return
    }
    if s.workersCancel != nil {
        s.workersCancel()
    }
    s.workersStarted = false
}

// onStartedLeading encapsulates responsibilities when this instance becomes leader.
// It starts leader-scoped services, ensures queue readiness, reconciles worker role,
// and spawns watchers for ongoing adjustments.
func (s *restServer) onStartedLeading(ctx context.Context, errChan chan error) {
    // Start event service (leader lifecycle)
    go event.StartEventWorker(ctx, errChan)

    // Ensure consumer group exists (best-effort) and start queue metrics
    s.ensureQueueGroup(ctx)
    s.startQueueMetrics(ctx)

    // Initial reconcile of worker role based on current replica count
    s.reconcileWorkers(ctx, errChan)

    // Periodically re-evaluate topology and reconcile role
    s.startReplicaWatcher(ctx, errChan)
}

func (s *restServer) ensureQueueGroup(ctx context.Context) {
    if s.Queue == nil {
        return
    }
    _ = s.Queue.EnsureGroup(ctx, "workflow-workers")
}

func (s *restServer) startQueueMetrics(ctx context.Context) {
    if s.Queue == nil {
        return
    }
    go func() {
        t := time.NewTicker(30 * time.Second)
        defer t.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-t.C:
                if s.Queue == nil {
                    continue
                }
                if bl, pd, err := s.Queue.Stats(ctx, "workflow-workers"); err == nil {
                    klog.Infof("queue stats stream=%s backlog=%d pending=%d", s.dispatchTopic(), bl, pd)
                } else {
                    klog.V(4).Infof("queue stats error: %v", err)
                }
            }
        }
    }()
}

func (s *restServer) reconcileWorkers(ctx context.Context, errChan chan error) {
    count := kube.DetectReplicaCount(ctx, s.KubeClient)
    if count >= 3 {
        s.stopWorkers()
    } else {
        s.startWorkers(ctx, errChan)
    }
}

func (s *restServer) startReplicaWatcher(ctx context.Context, errChan chan error) {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                s.reconcileWorkers(ctx, errChan)
            }
        }
    }()
}
