/*
Copyright 2023 kai.

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
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	appv1 "github.com/AnAnonymousFriend/KubeMin-Cli/api/v1"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=app.kubebuilder.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=app.kubebuilder.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.kubebuilder.io,resources=applications/finalizers,verbs=update

var CounterReconcileApplication = 0

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	<-time.NewTicker(100 * time.Millisecond).C
	logs := log.FromContext(ctx)

	CounterReconcileApplication += 1
	logs.Info("Starting a reconcile", "number", CounterReconcileApplication)
	app := &appv1.Application{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		if errors.IsNotFound(err) {
			logs.Info("Application not found.")
			return ctrl.Result{}, nil
		}
		logs.Error(err, "Failed to get the Application, will requeue after a short time.")
		return ctrl.Result{Requeue: false}, err
	}

	// reconcile sub-resources
	var result ctrl.Result
	var err error

	result, err = r.reconcileDeployment(ctx, app)
	if err != nil {
		logs.Error(err, "Failed to reconcile Deployment.")
		return result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1.Application{}).
		Complete(r)
}
