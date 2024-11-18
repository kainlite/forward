/*
Copyright 2024.

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

package controller

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
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	forwardv1alpha1 "redbeard.team/forward/api/v1alpha1"
)

// MapPortReconciler reconciles a MapPort object
type MapPortReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=forward.redbeard.team,resources=mapports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=forward.redbeard.team,resources=mapports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=forward.redbeard.team,resources=mapports/finalizers,verbs=update

// +kubebuilder:rbac:groups=mapports.forward.redbeard.team,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mapports.forward.redbeard.team,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=forward.redbeard.team,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MapPort object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile

func newPodForCR(cr *forwardv1alpha1.MapPort) *corev1.Pod {
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
	if cr.Spec.LivenessProbe {
		livenessCommand = fmt.Sprintf("nc -v -n -z %s %s", cr.Spec.Host, strconv.Itoa(cr.Spec.Port))
	} else {
		livenessCommand = fmt.Sprintf("echo")
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "forward-" + cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "mapport",
					Image:   "alpine/socat",
					Command: strings.Split(command, " "),
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
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

func (r *MapPortReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	reqLogger := r.Log.WithValues("namespace", req.Namespace, "MapPortForward", req.Name)
	reqLogger.Info("=== Reconciling Forward MapPort")
	// Fetch the MapPort instance
	instance := &forwardv1alpha1.MapPort{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after
			// reconcile request—return and don't requeue:
			return ctrl.Result{}, nil
		}
		// Error reading the object—requeue the request:
		return ctrl.Result{}, err
	}

	// If no phase set, default to pending (the initial phase):
	if instance.Status.Phase == "" || instance.Status.Phase == "PENDING" {
		instance.Status.Phase = forwardv1alpha1.PhaseRunning
	}

	// Now let's make the main case distinction: implementing
	// the state diagram PENDING -> RUNNING or PENDING -> FAILED
	switch instance.Status.Phase {
	case forwardv1alpha1.PhasePending:
		reqLogger.Info("Phase: PENDING")
		reqLogger.Info("Waiting to forward", "Host", instance.Spec.Host, "Port", instance.Spec.Port)
		instance.Status.Phase = forwardv1alpha1.PhaseRunning

		// requeue the request
		return ctrl.Result{}, err
	case forwardv1alpha1.PhaseRunning:
		reqLogger.Info("Phase: RUNNING")
		pod := newPodForCR(instance)
		// Set MapPort instance as the owner and controller
		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			// requeue with error
			return ctrl.Result{}, err
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
				return ctrl.Result{}, err
			}
			reqLogger.Info("Pod launched", "name", pod.Name)
		} else if err != nil {
			// requeue with error
			return ctrl.Result{}, err
		} else if found.Status.Phase == corev1.PodFailed ||
			found.Status.Phase == corev1.PodSucceeded {
			reqLogger.Info("Container terminated", "reason",
				found.Status.Reason, "message", found.Status.Message)
			instance.Status.Phase = forwardv1alpha1.PhaseFailed
		} else {
			// Don't requeue because it will happen automatically when the
			// pod status changes.
			return ctrl.Result{}, nil
		}
	case forwardv1alpha1.PhaseFailed:
		reqLogger.Info("Phase: Failed, check that the host and port are reachable from the cluster and that there are no networks policies preventing this access or firewall rules...")
		return ctrl.Result{}, nil
	default:
		reqLogger.Info("NOP")
		return ctrl.Result{}, nil
	}

	// Update the At instance, setting the status to the respective phase:
	err = r.Status().Update(context.TODO(), instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Don't requeue. We should be reconcile because either the pod
	// or the CR changes.
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MapPortReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&forwardv1alpha1.MapPort{}).
		Named("mapport").
		Complete(r)
}
