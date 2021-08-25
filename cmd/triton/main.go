/*
Copyright 2021 The Triton Authors.

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
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	kubeclient "github.com/triton-io/triton/pkg/kube/client"
	"github.com/triton-io/triton/pkg/log"
	"github.com/triton-io/triton/pkg/routes"
	"github.com/triton-io/triton/pkg/server/grpc"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/kube/controller"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/capabilities"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	_ "go.uber.org/automaxprocs"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	_ "net/http/pprof"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = kruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	restConfigQPS   = flag.Int("rest-config-qps", 30, "QPS of rest config.")
	restConfigBurst = flag.Int("rest-config-burst", 50, "Burst of rest config.")
)

func init() {

	_ = clientgoscheme.AddToScheme(scheme)

	_ = tritonappsv1alpha1.AddToScheme(scheme)
	_ = kruiseappsv1alpha1.AddToScheme(scheme)

	//+kubebuilder:scaffold:builder
}
func main() {
	var metricsAddr, restAddr, grpcAddr, pprofAddr string
	var healthProbeAddr string
	var enableLeaderElection, enablePprof, allowPrivileged bool
	var leaderElectionNamespace string
	var namespace string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&restAddr, "rest-addr", ":8088", "The address the RESTAPI endpoint binds to.")
	flag.StringVar(&grpcAddr, "grpc-addr", ":8099", "The address the GRPCAPI endpoint binds to.")
	flag.StringVar(&healthProbeAddr, "health-probe-addr", ":8000", "The address the healthz/readyz endpoint binds to.")
	flag.BoolVar(&allowPrivileged, "allow-privileged", true, "If true, allow privileged containers. It will only work if api-server is also"+
		"started with --allow-privileged=true.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", true, "Whether you need to enable leader election.")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "triton-system",
		"This determines the namespace in which the leader election configmap will be created, it will use in-cluster namespace if empty.")
	flag.StringVar(&namespace, "namespace", "",
		"Namespace if specified restricts the manager's cache to watch objects in the desired namespace. Defaults to all namespaces.")
	flag.BoolVar(&enablePprof, "enable-pprof", false, "Enable pprof for controller manager.")
	flag.StringVar(&pprofAddr, "pprof-addr", ":8090", "The address the pprof binds to.")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		panic(err)
	}

	log.InitLog()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if enablePprof {
		go func() {
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				setupLog.Error(err, "unable to start pprof")
			}
		}()
	}
	var stopCh chan struct{}
	defer func() {
		stopCh <- struct{}{}
	}()

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	if allowPrivileged {
		capabilities.Initialize(capabilities.Capabilities{
			AllowPrivileged: allowPrivileged,
		})
	}

	cfg := ctrl.GetConfigOrDie()
	setRestConfig(cfg)
	cfg.UserAgent = "triton-manager"

	mgr := kubeclient.NewManager()

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	go func() {
		setupLog.Info("setup controllers")
		if err := controllers.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup controllers")
			os.Exit(1)
		}
	}()

	go func() {
		setupLog.Info("starting manager")
		if err := mgr.Start(stopCh); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()

	// start grpc server
	go grpc.Serve()
	log.Infof("NumCPU: %d, GOMAXPROCS: %d\n", runtime.NumCPU(), runtime.GOMAXPROCS(-1))

	// 运行服务器
	server := routes.NewServer().SetupRouters()
	if err := server.Run(restAddr); err != nil {
		log.Error("run server error:", err)
		panic(err)
	}
}

func setRestConfig(c *rest.Config) {
	if *restConfigQPS > 0 {
		c.QPS = float32(*restConfigQPS)
	}
	if *restConfigBurst > 0 {
		c.Burst = *restConfigBurst
	}
}
