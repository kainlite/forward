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
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	forwardv1beta1 "github.com/kainlite/forward/api/v1beta1"
)

// +kubebuilder:rbac:groups=maps.forward.techsquad.rocks,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=map.forward.techsquad.rocks,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=forward.techsquad.rocks,resources=maps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=forward.techsquad.rocks,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// MapReconciler reconciles a Map object
type MapReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func newPodForCR(cr *forwardv1beta1.Map) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	var command string
	if strings.EqualFold(cr.Spec.Protocol, "tcp") {
		command = fmt.Sprintf("socat -d -d tcp-listen:%s,fork,reuseaddr tcp-connect:%s:%s", strconv.Itoa(cr.Spec.Port), cr.Spec.Host, strconv.Itoa(cr.Spec.Port))
	} else if strings.EqualFold(cr.Spec.Protocol, "udp") {
		command = fmt.Sprintf("socat -d -d UDP4-RECVFROM:%s,fork,reuseaddr UDP4-SENDTO:%s:%s", strconv.Itoa(cr.Spec.Port), cr.Spec.Host, strconv.Itoa(cr.Spec.Port))
	} else {
		// TODO: Create a proper error here if the protocol doesn't match or is unsupported
		command = fmt.Sprintf("socat -V")
	}

	var livenessCommand string
	if cr.Spec.LivenessProbe == true {
		livenessCommand = fmt.Sprintf("nc -v -n -z %s %s", cr.Spec.Host, strconv.Itoa(cr.Spec.Port))
	} else {
		livenessCommand = fmt.Sprintf("echo")
	}

	// Learn more at: https://godoc.org/k8s.io/api/core/v1
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "forward-" + cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "map",
					Image:   "alpine/socat",
					Command: strings.Split(command, " "),
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							Exec: &corev1.ExecAction{
								Command: strings.Split(livenessCommand, " "),
							},
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
}

func (r *MapReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("namespace", req.Namespace, "MapForward", req.Name)
	reqLogger.Info("=== Reconciling Forward Map")
	// Fetch the Map instance
	instance := &forwardv1beta1.Map{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after
			// reconcile request—return and don't requeue:
			return reconcile.Result{}, nil
		}
		// Error reading the object—requeue the request:
		return reconcile.Result{}, err
	}

	// If no phase set, default to pending (the initial phase):
	if instance.Status.Phase == "" || instance.Status.Phase == "PENDING" {
		instance.Status.Phase = forwardv1beta1.PhaseRunning
	}

	// Now let's make the main case distinction: implementing
	// the state diagram PENDING -> RUNNING or PENDING -> FAILED
	switch instance.Status.Phase {
	case forwardv1beta1.PhasePending:
		reqLogger.Info("Phase: PENDING")
		reqLogger.Info("Waiting to forward", "Host", instance.Spec.Host, "Port", instance.Spec.Port)
		instance.Status.Phase = forwardv1beta1.PhaseRunning
		// If we ever get here, just requeue the request
		return reconcile.Result{}, err
	case forwardv1beta1.PhaseRunning:
		reqLogger.Info("Phase: RUNNING")
		pod := newPodForCR(instance)
		// Set Map instance as the owner and controller
		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			// requeue with error
			return reconcile.Result{}, err
		}
		found := &corev1.Pod{}
		nsName := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
		err = r.Get(context.TODO(), nsName, found)
		// Try to see if the pod already exists and if not
		// (which we expect) then create a one-shot pod as per spec:
		if err != nil && errors.IsNotFound(err) {
			err = r.Create(context.TODO(), pod)
			if err != nil {
				// requeue with error
				return reconcile.Result{}, err
			}
			reqLogger.Info("Pod launched", "name", pod.Name)
		} else if err != nil {
			// requeue with error
			return reconcile.Result{}, err
		} else if found.Status.Phase == corev1.PodFailed ||
			found.Status.Phase == corev1.PodSucceeded {
			reqLogger.Info("Container terminated", "reason",
				found.Status.Reason, "message", found.Status.Message)
			instance.Status.Phase = forwardv1beta1.PhaseFailed
		} else {
			// Don't requeue because it will happen automatically when the
			// pod status changes.
			return reconcile.Result{}, nil
		}
	case forwardv1beta1.PhaseFailed:
		reqLogger.Info("Phase: Failed, check that the host and port are reachable from the cluster and that there are no networks policies preventing this access or firewall rules...")
		return reconcile.Result{}, nil
	default:
		reqLogger.Info("NOP")
		return reconcile.Result{}, nil
	}

	// Update the At instance, setting the status to the respective phase:
	err = r.Status().Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Don't requeue. We should be reconcile because either the pod
	// or the CR changes.
	return reconcile.Result{}, nil
}

func (r *MapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&forwardv1beta1.Map{}).
		Complete(r)
}
