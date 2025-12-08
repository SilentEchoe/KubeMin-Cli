package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/event"
	"KubeMin-Cli/pkg/apiserver/infrastructure/clients"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/mysql"
	msg "KubeMin-Cli/pkg/apiserver/infrastructure/messaging"
	"KubeMin-Cli/pkg/apiserver/interfaces/api"
	"KubeMin-Cli/pkg/apiserver/interfaces/api/middleware"
	"KubeMin-Cli/pkg/apiserver/utils/cache"
	"KubeMin-Cli/pkg/apiserver/utils/container"
	"KubeMin-Cli/pkg/apiserver/utils/kube"
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
	KubeClient     kubernetes.Interface `inject:"kubeClient"` //inject 是注入IOC的name，如果tag中包含inject 那么必须有对应的容器注入服务,必须大写，小写会无法访问
	KubeConfig     *rest.Config         `inject:"kubeConfig"`
	Queue          msg.Queue            `inject:"queue"`
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

	// Initialize cache implementation. Prefer redis if configured; fallback to memory.
	var iCache cache.ICache
	switch strings.ToLower(s.cfg.Cache.CacheType) {
	case string(cache.CacheTypeRedis):
		rcli, err := clients.EnsureRedis(s.cfg.Cache)
		if err != nil {
			klog.ErrorS(err, "init redis cache client failed; falling back to in-memory cache")
			iCache = cache.New(false, cache.CacheTypeMem)
		} else {
			cache.SetGlobalRedisClient(rcli)
			iCache = cache.NewRedisICache(rcli, false, s.cfg.Cache.CacheTTL, s.cfg.Cache.KeyPrefix)
		}
	default:
		iCache = cache.New(false, cache.CacheTypeMem)
	}

	// 将db 注入到IOC中
	if err := s.beanContainer.ProvideWithName("datastore", s.dataStore); err != nil {
		return fmt.Errorf("fail to provides the datastore bean to the container: %w", err)
	}

	if err := s.beanContainer.ProvideWithName("cache", iCache); err != nil {
		return fmt.Errorf("fail to provides the cache bean to the container: %w", err)
	}

	// Initialize work queue (Redis Streams if configured; noop otherwise)
	streamKey := s.dispatchTopic()
	q := s.buildQueue(streamKey)
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

	// domain - repository (注入 Repository，依赖 datastore)
	if err := s.beanContainer.Provides(repository.InitRepositoryBean()...); err != nil {
		return fmt.Errorf("fail to provides the repository bean to the container: %w", err)
	}

	// domain - service (注入 Service，可依赖 Repository)
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

// buildQueue constructs the messaging queue based on config.
// It returns a usable Queue in all cases, falling back to NoopQueue on failures.
func (s *restServer) buildQueue(streamKey string) msg.Queue {
	if strings.ToLower(s.cfg.Messaging.Type) != "redis" {
		return &msg.NoopQueue{}
	}

	// Reuse shared redis client from factory
	rcli, err := clients.EnsureRedis(s.cfg.Cache)
	if err != nil {
		klog.Warningf("init redis client failed, falling back to noop: %v", err)
		return &msg.NoopQueue{}
	}
	if cache.GetGlobalRedisClient() == nil {
		cache.SetGlobalRedisClient(rcli)
	}

	rq, err := msg.NewRedisStreamsWithClient(rcli, streamKey, s.cfg.Messaging.RedisStreamMaxLen)
	if err != nil {
		klog.Warningf("init redis streams with client failed, falling back to noop: %v", err)
		return &msg.NoopQueue{}
	}
	return rq
}

func (s *restServer) RegisterAPIRoute() {
	// 初始化中间件
	s.webContainer.Use(gin.Recovery())

	// Enable CORS for browser clients
	s.webContainer.Use(middleware.CORS(middleware.CORSOptions{
		AllowOrigins:     s.cfg.CORS.AllowedOrigins,
		AllowMethods:     s.cfg.CORS.AllowedMethods,
		AllowHeaders:     s.cfg.CORS.AllowedHeaders,
		ExposeHeaders:    s.cfg.CORS.ExposedHeaders,
		AllowCredentials: s.cfg.CORS.AllowCredentials,
		MaxAge:           s.cfg.CORS.MaxAge,
	}))

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
	server := &http.Server{
		Addr:              s.cfg.BindAddr,
		Handler:           s,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown handler
	shutdownComplete := make(chan struct{})
	go func() {
		<-ctx.Done()
		klog.Info("HTTP server shutdown initiated")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			klog.Errorf("HTTP server graceful shutdown error: %v", err)
			// Force close if graceful shutdown fails
			if closeErr := server.Close(); closeErr != nil {
				klog.Errorf("HTTP server force close error: %v", closeErr)
			}
		} else {
			klog.Info("HTTP server graceful shutdown completed")
		}
		close(shutdownComplete)
	}()

	err := server.ListenAndServe()
	<-shutdownComplete

	// Ignore normal shutdown error
	if err == http.ErrServerClosed {
		klog.Info("HTTP server closed normally")
		return nil
	}
	return err
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

	// TmpCreate cancelable context for graceful shutdown
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case sig := <-sigChan:
			klog.Infof("received signal %v, initiating graceful shutdown", sig)
			// Stop workers before canceling context
			s.stopWorkers()
			runCancel()
		case <-ctx.Done():
			// Parent context canceled
			s.stopWorkers()
		}
	}()

	go func() {
		leaderelection.RunOrDie(runCtx, *l)
	}()

	return s.startHTTP(runCtx)
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
