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
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kainlite/forward/controllers/apis"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"

	forwardv1beta1 "github.com/kainlite/forward/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = forwardv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

func TestForwardControllerPodCreate(t *testing.T) {
	var (
		name      = "forward-mapsample-pod"
		namespace = "default"
		host      = "localhost"
		port      = 8000
		protocol  = "tcp"
	)

	// --------------------------------------------------
	// 1- Create a Forward container object
	forwardContainer := &forwardv1beta1.Map{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: forwardv1beta1.MapSpec{
			Host:     host,
			Port:     port,
			Protocol: protocol,
		},
	}
	// Register the object in the fake client.
	objs := []runtime.Object{
		forwardContainer,
	}
	// --------------------------------------------------

	// --------------------------------------------------
	// 2- Create a fake Kubernetes API client
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(forwardv1beta1.GroupVersion, forwardContainer)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a MapReconciler object with the scheme and fake client.
	// --------------------------------------------------

	// --------------------------------------------------
	// 3- Create the context for the controller
	setupLog := ctrl.Log.WithName("setup")
	r := &MapReconciler{Client: cl, Scheme: s, Log: setupLog.WithName("controllers").WithName("Map")}
	// --------------------------------------------------

	// --------------------------------------------------
	// 4- Mock the event processing
	// Create a request, that should be in the work queue.
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	// Process the request
	res, err := r.Reconcile(req)

	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	if res != (reconcile.Result{}) {
		t.Error("reconcile did not return an empty Result")
	}
	// --------------------------------------------------

	// --------------------------------------------------
	// 5- Calculate how the expected ForwardContainer’s pod should look like
	expectedPod := newPodForCR(forwardContainer)
	// --------------------------------------------------

	// --------------------------------------------------
	// 6- Check that a pod matching the expected one was created
	pod := &corev1.Pod{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: expectedPod.Name, Namespace: expectedPod.Namespace}, pod)
	if err != nil {
		t.Fatalf("get pod: (%v)", err)
	}
	// --------------------------------------------------

	// --------------------------------------------------
	// 7- Check status is correctly updated
	updatedForwardContainer := &forwardv1beta1.Map{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: forwardContainer.Name, Namespace: forwardContainer.Namespace}, updatedForwardContainer)
	if err != nil {
		t.Fatalf("get forward container: (%v)", err)
	}
	if updatedForwardContainer.Status.Phase != "RUNNING" {
		t.Errorf("incorrect forward container Phase: (%v)", updatedForwardContainer.Status.Phase)
	}
	if updatedForwardContainer.Spec.Host != "localhost" {
		t.Errorf("incorrect host in container: (%v)", updatedForwardContainer.Spec.Host)
	}
	if updatedForwardContainer.Spec.Port != 8000 {
		t.Errorf("incorrect port in container: (%v)", updatedForwardContainer.Spec.Port)
	}
	if updatedForwardContainer.Spec.Protocol != "tcp" {
		t.Errorf("incorrect protocol in container: (%v)", updatedForwardContainer.Spec.Protocol)
	}
}

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func TestMain(m *testing.M) {
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}
	apis.AddToScheme(scheme.Scheme)

	var err error
	if cfg, err = t.Start(); err != nil {
		stdlog.Fatal(err)
	}

	code := m.Run()
	t.Stop()
	os.Exit(code)
}

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(req)
		requests <- req
		return result, err
	})
	return fn, requests
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager, g *gomega.GomegaWithT) (chan struct{}, *sync.WaitGroup) {
	stop := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.Expect(mgr.Start(stop)).NotTo(gomega.HaveOccurred())
	}()
	return stop, wg
}
