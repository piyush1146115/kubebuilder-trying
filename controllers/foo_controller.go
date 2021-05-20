/*
Copyright 2021.

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batchv1 "github.com/piyush1146115/kubebuilder-trying/api/v1"
)

// FooReconciler reconciles a Foo object
type FooReconciler struct {
	client.Client
	Log    logr.Logger

	Recorder record.EventRecorder
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.piyush.kubebuilder.io,resources=foos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.piyush.kubebuilder.io,resources=foos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.piyush.kubebuilder.io,resources=foos/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch


// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Foo object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *FooReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("foo", req.NamespacedName)

	//ctx := context.Background()
	log := r.Log.WithValues("foo", req.NamespacedName)

	// your logic here
	log.Info("fetching MyKind resource")
	foo := batchv1.Foo{}
	if err := r.Client.Get(ctx, req.NamespacedName, &foo); err != nil {
		log.Error(err, "failed to get Foo resource")
		// Ignore NotFound errors as they will be retried automatically if the
		// resource is created in future.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.cleanupOwnedResources(ctx, log, &foo); err != nil {
		log.Error(err, "failed to clean up old Deployment resources for this foo")
		return ctrl.Result{}, err
	}

	log = log.WithValues("deployment_name", foo.Spec.Name)

	log.Info("checking if an existing Deployment exists for this resource")
	deployment := apps.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: foo.Namespace, Name: foo.Spec.Name}, &deployment)

	if apierrors.IsNotFound(err) {
		log.Info("could not find existing Deployment for MyKind, creating one...")

		deployment = *buildDeployment(foo)
		if err := r.Client.Create(ctx, &deployment); err != nil {
			log.Error(err, "failed to create Deployment resource")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(&foo, core.EventTypeNormal, "Created", "Created deployment %q", deployment.Name)
		log.Info("created Deployment resource for MyKind")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FooReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Foo{}).
		Complete(r)
}

var (
	deploymentOwnerKey = ".metadata.controller"
)



func buildDeployment(foo batchv1.Foo) *apps.Deployment {
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            foo.Spec.Name,
			Namespace:       foo.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&foo, batchv1.GroupVersion.WithKind("Foo"))},
		},
		Spec: apps.DeploymentSpec{
			Replicas: foo.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-controller.jetstack.io/deployment-name": foo.Spec.Name,
				},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"example-controller.jetstack.io/deployment-name": foo.Spec.Name,
					},
				},
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	return &deployment
}




// cleanupOwnedResources will Delete any existing Deployment resources that
// were created for the given MyKind that no longer match the
// myKind.spec.deploymentName field.
func (r *FooReconciler) cleanupOwnedResources(ctx context.Context, log logr.Logger, foo *batchv1.Foo) error {
	log.Info("finding existing Deployments for MyKind resource")

	// List all deployment resources owned by this MyKind
	var deployments apps.DeploymentList
	//if err := r.List(ctx, &deployments, client.InNamespace(foo.Namespace), client.MatchingField(deploymentOwnerKey, foo.Name)); err != nil {
	//	return err
	//}

	deleted := 0
	for _, depl := range deployments.Items {
		if depl.Name == foo.Spec.Name {
			// If this deployment's name matches the one on the MyKind resource
			// then do not delete it.
			continue
		}

		if err := r.Client.Delete(ctx, &depl); err != nil {
			log.Error(err, "failed to delete Deployment resource")
			return err
		}

		r.Recorder.Eventf(foo, core.EventTypeNormal, "Deleted", "Deleted deployment %q", depl.Name)
		deleted++
	}

	log.Info("finished cleaning up old Deployment resources", "number_deleted", deleted)

	return nil
}
