/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	batchv1 "KubeMin-Cli/example/api/v1"
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CronJobReconciler reconciles a CronJob object
type CronJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Clock
}

// 虚拟一个时钟,以便在测试中方便地来回调节时间，调用 time.Now 获取真实时间

type realClock struct{}

func (_ realClock) Now() time.Time {
	return time.Now()
}

type Clock interface {
	Now() time.Time
}

// 需要获得RBAC权限，需要额外权限去创建或

//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	_ = r.Log.WithValues("cronjob", req.NamespacedName)

	fmt.Println("test")

	////1.根据名称加载定时任务
	//var cronJob batchv1.CronJob
	//if err := r.Get(ctx, req.NamespacedName, &cronJob); err != nil {
	//	fmt.Println(err, "unable to fetch CronJob")
	//	//忽略掉 not-found 错误，它们不能通过重新排队修复（要等待新的通知）
	//	//在删除一个不存在的对象时，可能会报这个错误。
	//	return ctrl.Result{}, client.IgnoreNotFound(err)
	//}
	//
	////2.列出所有有效Job,更新它们的状态
	//var childJobs kbatch.JobList
	//if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{}); err != nil {
	//	fmt.Println(err, "unable to list child Jobs")
	//
	//	return ctrl.Result{}, err
	//}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Complete(r)
}
