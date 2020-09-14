package main

import (
	ctx "context"
	"flag"

	"github.com/nevercase/k8s-controller-custom-resource/pkg/signals"
	exec "github.com/nevercase/k8s-exec-pod"
	"k8s.io/klog"
)

var (
	masterUrl   string
	kubeconfig  string
	httpservice string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterUrl, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&httpservice, "httpservice", "0.0.0.0:9090", "The address of the http server.")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	_ = exec.InitServer(ctx.Background(), httpservice, kubeconfig, masterUrl)
	<-stopCh
}