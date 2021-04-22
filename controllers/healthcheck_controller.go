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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	route53v1 "github.com/takutakahashi/external-route53/api/v1"
	"github.com/takutakahashi/external-route53/pkg/healthcheck"
)

// HealthCheckReconciler reconciles a HealthCheck object
type HealthCheckReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const finalizer = "healthcheck.finalizer.external-route53.io"
const queriedGenerationAnnotationKey = "healthcheck.external-route53.io/queried-generation"

// +kubebuilder:rbac:groups=route53.takutakahashi.dev,resources=healthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route53.takutakahashi.dev,resources=healthchecks/status,verbs=get;update;patch

func (r *HealthCheckReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	// TODO: add Event
	ctx := context.Background()
	_ = r.Log.WithValues("healthcheck", req.NamespacedName)
	h := route53v1.HealthCheck{}
	err := r.Get(ctx, req.NamespacedName, &h)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}
	queriedGeneration, _ := strconv.ParseInt(h.Annotations[queriedGenerationAnnotationKey], 0, 64)
	if err == nil && h.Generation == queriedGeneration {
		return ctrl.Result{}, nil
	}
	if h.DeletionTimestamp != nil {
		err = r.reconcileDelete(h)
		if err != nil {
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
		return ctrl.Result{}, nil
	} else {
		err = r.reconcile(h)
	}
	if err != nil {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	return ctrl.Result{}, nil
}

func (r *HealthCheckReconciler) reconcile(h route53v1.HealthCheck) error {
	newHealthCheck, err := healthcheck.Ensure(h.DeepCopy())
	if err != nil {
		return err
	}
	newHealthCheck.Finalizers = append(newHealthCheck.Finalizers, finalizer)
	if newHealthCheck.Annotations == nil {
		newHealthCheck.Annotations = map[string]string{}
	}
	newHealthCheck.Annotations[queriedGenerationAnnotationKey] = strconv.Itoa(int(newHealthCheck.Generation))
	return r.Update(context.TODO(), newHealthCheck, &client.UpdateOptions{})
}
func (r *HealthCheckReconciler) reconcileDelete(h route53v1.HealthCheck) error {
	newHealthCheck, err := healthcheck.Delete(h.DeepCopy())
	if err != nil {
		return err
	}
	newHealthCheck.Finalizers = removeString(newHealthCheck.Finalizers, finalizer)
	return r.Update(context.TODO(), newHealthCheck, &client.UpdateOptions{})
}
func (r *HealthCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&route53v1.HealthCheck{}).
		Complete(r)
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
