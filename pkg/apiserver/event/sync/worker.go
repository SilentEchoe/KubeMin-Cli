package sync

import (
	v1alpha1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	wf "KubeMin-Cli/pkg/apiserver/workflow"
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicInformer "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

const (
	APPNAME = "applications"
)

// ApplicationSync 用于从k8s集群中的APP信息同步到数据库中
type ApplicationSync struct {
	KubeClient         client.Client       `inject:"kubeClient"`
	KubeConfig         *rest.Config        `inject:"kubeConfig"`
	Store              datastore.DataStore `inject:"datastore"`
	ApplicationService service.ApplicationService
	Queue              workqueue.TypedRateLimitingInterface[any]
}

func (a *ApplicationSync) Start(ctx context.Context, errorChan chan error) {
	dynamicClient, err := dynamic.NewForConfig(a.KubeConfig)
	if err != nil {
		errorChan <- err
	}
	// 创建client-go inform
	factory := dynamicInformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 0, v1.NamespaceAll, nil)
	informer := factory.ForResource(wf.SchemeGroupVersion.WithResource("applications")).Informer()

	cu := &CR2UX{
		ds:                 a.Store,
		cli:                a.KubeClient,
		cache:              sync.Map{},
		applicationService: a.ApplicationService,
	}

	if err = cu.initCache(ctx); err != nil {
		errorChan <- err
	}

	go func() {
		for {
			item, down := a.Queue.Get()
			if down {
				break
			}
			app := item.(*v1alpha1.Applications)
			if err := cu.AddOrUpdate(ctx, app); err != nil {
				failTimes := a.Queue.NumRequeues(app)
				klog.Errorf("fail to add or update application %s: %s, requeue times: %d", app.Name, err.Error(), failTimes)
				// 如果重试小于五次，则重新加入队列
				if failTimes < 5 {
					a.Queue.AddRateLimited(app)
				} else {
					//如果重试超过五次，则从队列里面移除
					klog.Errorf("fail to add or update application %s: %s, requeue times reach the limit(%d), give up", app.Name, err.Error(), failTimes)
					a.Queue.Forget(app)
				}
			}
			a.Queue.Done(app)
		}
	}()

	addOrUpdateHandler := func(obj interface{}) {
		app := getApp(obj)
		if app.DeletionTimestamp == nil {
			a.Queue.Add(app)
			klog.V(4).Infof("watched update/add app event, namespace: %s, name: %s", app.Namespace, app.Name)
		}
	}

	// Inform
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addOrUpdateHandler(obj)
		},
		UpdateFunc: func(oldObj, obj interface{}) { //nolint:revive,unused
			addOrUpdateHandler(obj)
		},
		DeleteFunc: func(obj interface{}) {
			app := getApp(obj)
			klog.V(4).Infof("watched delete app event, namespace: %s, name: %s", app.Namespace, app.Name)
			a.Queue.Forget(app)
			a.Queue.Done(app)
			// 从数据库中删除这个APP
			//err = cu.DeleteApp(ctx, app)
			//if err != nil {
			//	klog.Errorf("Application %-30s Deleted Sync to db err %v", color.WhiteString(app.Namespace+"/"+app.Name), err)
			//}
			klog.Infof("delete the application (%s/%s) metadata successfully", app.Namespace, app.Name)
		},
	}

	_, err = informer.AddEventHandler(handlers)
	if err != nil {
		klog.ErrorS(err, "failed to add event handler for application sync")
		return
	}
	klog.Info("app syncing started")
	informer.Run(ctx.Done())
}

// 获取Pod的元数据
func getApp(obj interface{}) *v1alpha1.Applications {
	if app, ok := obj.(*v1alpha1.Applications); ok {
		return app
	}
	var app v1alpha1.Applications
	if object, ok := obj.(*unstructured.Unstructured); ok {
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &app); err != nil {
			klog.Errorf("decode the Pod failure %s", err.Error())
			return &app
		}
	}
	return &app
}
