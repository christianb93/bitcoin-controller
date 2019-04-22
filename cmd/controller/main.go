package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	clientset "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

//
// Create a channel that will be closed when a signal is received
//
func createSignalHandler() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-c
		fmt.Printf("Signal handler: received signal %s\n", sig)
		close(stop)
	}()
	return stop
}

func main() {
	var kubeconfig string
	// Initialize logging
	klog.InitFlags(nil)
	klog.Info("Starting up")
	// Establish a signal handler - this will return a channel which
	// will be closed if a signal is received
	stopCh := createSignalHandler()
	// Get command line arguments
	flag.StringVar(&kubeconfig, "kubeconfig", "", "The kubectl configuration file to use")
	flag.Parse()
	if kubeconfig != "" {
		klog.Infof("Trying stand-alone configuration with kubectl config file %s\n", kubeconfig)
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Error("Could not get config")
		os.Exit(1)
	}
	// Create clientset from config
	_, err = kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Could not create K8s clientset")
		os.Exit(1)
	}
	// Create BitcoinNetwork client set
	_, err = clientset.NewForConfig(config)
	if err != nil {
		klog.Error("Could not create BitcoinNetwork clientset")
		os.Exit(1)
	}
	klog.Info("Entering main loop")
	<-stopCh
}
