package main

import (
	"fmt"
	"path/filepath"
	"testing"

	clientset "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func TestClientsetCreation(t *testing.T) {
	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic("Could not get configuration file")
	}
	// Create BitcoinNetwork client set
	_, err = clientset.NewForConfig(config)
	if err != nil {
		panic("Could not create BitcoinNetwork clientset")
	}
}

func TestGet(t *testing.T) {
	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic("Could not get configuration file")
	}
	// Create BitcoinNetwork client set
	c, err := clientset.NewForConfig(config)
	if err != nil {
		panic("Could not create BitcoinNetwork clientset")
	}
	client := c.BitcoincontrollerV1()
	list, err := client.BitcoinNetworks("default").List(metav1.ListOptions{})
	if err != nil {
		t.Errorf("Could not get list of BitcoinNetworks")
		panic(err)
	}
	for _, item := range list.Items {
		fmt.Printf("Have item %s\n", item.Name)
	}
}
