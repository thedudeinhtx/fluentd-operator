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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/thedudeinhtx/fluentd-operator/api/v1"
)

// FluentdReconciler reconciles a Fluentd object
type FluentdReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitoring.thedude.cc,resources=fluentds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.thedude.cc,resources=fluentds/status,verbs=get;update;patch

func (r *FluentdReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx = context.Background()
	log = r.Log.WithValues("fluentd", req.NamespacedName)

	fluentd := &monitoringv1.Fluentd{}
	err := r.Get(ctx, req.NamespacedName, fluentd)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Fluentd not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Fluentd")
		return ctrl.Result{}, err
	}

	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: fluentd.Name, Namespace: fluentd.Namespace}, found)
	if err != nil  && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForFluentd(fluentd)
		log.Info("Creating a new deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(fluentd.Namespace),
		client.MatchingLabels(labelsForFluentd(fluentd.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Fluentd.Namespace", fluentd.Namespace, "Fluentd.Name", fluentd.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, fluentd.Status.Nodes) {
		fluentd.Status.Nodes = podNames
		err := r.Status().Update(ctx, fluentd)
		if err != nil {
			log.Error(err, "Failed to update Fluentd status")
			return ctrl.Result{}, err
		}
	}


	return ctrl.Result{}, nil
}

// deploymentForMemcached returns a memcached Deployment object
func (r *FluentdReconciler) deploymentForFluentd(m *monitoringv1.Fluentd) *appsv1.Deployment {
	ls := labelsForFluentd(m.Name)
	//version := m.Spec.FluentdVersion

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "fluentd:v1.11-1",
						Name:    "fluentd",
						Command: []string{"fluentd", "-m=64", "-o", "modern", "-v"},
						}},
					},
				},
			},
		}
	
	// Set Memcached instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// labelsForFluentd returns the labels for selecting the resources
// belonging to the given memcached CR name.
func labelsForFluentd(name string) map[string]string {
	return map[string]string{"app": "fluentd", "fluentd_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}


func (r *FluentdReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.Fluentd{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
