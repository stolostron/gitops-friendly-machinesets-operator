/*
Copyright 2021 Ales Nosek.

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

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	machineapi "github.com/openshift/api/machine/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/noseka1/gitops-friendly-machinesets-controller/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(machineapi.Install(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "123eec1d.redhat.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	machineSetInterface := getMachineSetInterface(mgr.GetConfig())

	if err = (&controllers.MachineSetReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		MachineSetInterface: machineSetInterface,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MachineSet")
		os.Exit(1)
	}
	if err = (&controllers.MachineReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Machine")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getMachineSetInterface(clientConfig *rest.Config) dynamic.NamespaceableResourceInterface {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "unable to create a discovery client")
		os.Exit(1)
	}

	machineSetGv := schema.GroupVersion{
		Group:   machineapi.SchemeGroupVersion.Group,
		Version: machineapi.SchemeGroupVersion.Version,
	}
	machineSetKind := "MachineSet"

	resourceList, err := discoveryClient.ServerResourcesForGroupVersion(machineSetGv.String())
	if err != nil {
		setupLog.Error(err, "unable to retrieve resouce list for "+machineSetGv.String())
		os.Exit(1)
	}

	var machineSetResource string
	for _, res := range resourceList.APIResources {
		if res.Kind == machineSetKind {
			machineSetResource = res.Name
			break
		}
	}

	if machineSetResource == "" {
		setupLog.Info("cannot find resource for kind " + machineSetKind)
		os.Exit(1)
	}

	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "unable to create a dynamic client")
		os.Exit(1)
	}

	machineSetGvr := schema.GroupVersionResource{
		Group:    machineapi.SchemeGroupVersion.Group,
		Version:  machineapi.SchemeGroupVersion.Version,
		Resource: machineSetResource,
	}

	return dynamicClient.Resource(machineSetGvr)
}
