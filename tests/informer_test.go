package main

import (
	"testing"
	"time"

	clientset "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"
	bcinformers "github.com/christianb93/bitcoin-controller/internal/generated/informers/externalversions"
	"k8s.io/client-go/tools/clientcmd"
)

func TestInformerCreationIntegration(t *testing.T) {
	kubeconfig := getKubeConfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic("Could not get configuration file")
	}
	// Create BitcoinNetwork client set
	client, err := clientset.NewForConfig(config)
	if err != nil {
		t.Errorf("Could not create BitcoinNetwork clientset\n")
	}
	// Create an informer for BitcoinNetwork objects
	myInformerFactory := bcinformers.NewSharedInformerFactory(client, time.Second*30)
	if myInformerFactory == nil {
		t.Errorf("Could not create BitcoinNetwork informer factory\n")
	}
	myInformer := myInformerFactory.Bitcoincontroller().V1().BitcoinNetworks()
	if myInformer == nil {
		t.Errorf("Could not create BitcoinNetwork informer\n")
	}
}
