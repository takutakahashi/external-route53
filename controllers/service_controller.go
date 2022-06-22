/*


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
	"time"

	"github.com/go-logr/logr"
	"github.com/takutakahashi/external-route53/pkg/dns"
	"github.com/takutakahashi/external-route53/pkg/healthcheck"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Dns    dns.Dns
}

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get;update;patch

func (r *ServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("service", req.NamespacedName)
	svc := corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
	if svc.DeletionTimestamp != nil {

		if err := r.reconcileDelete(svc.DeepCopy()); err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		return ctrl.Result{}, nil
	}
	if err := r.reconcile(svc.DeepCopy()); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) reconcileDelete(svc *corev1.Service) error {
	return r.Dns.Delete(svc)
}
func (r *ServiceReconciler) reconcile(svc *corev1.Service) error {
	if _, ok := svc.Annotations[dns.HostnameAnnotationKey]; !ok {
		return nil
	}
	if a, ok := svc.Annotations[dns.HealthCheckAnnotationKey]; ok && a == "true" && svc.Annotations[dns.HealthCheckIdAnnotationKey] == "" {
		hcsvc, err := healthcheck.EnsureResource(svc)
		if err != nil {
			return err
		}
		if hcsvc != nil {
			return r.Update(context.TODO(), hcsvc.DeepCopy(), &client.UpdateOptions{})
		}
	}
	return r.Dns.Ensure(svc)
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
