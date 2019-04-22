package main

import (
	"fmt"
	"path/filepath"
	"testing"

	bitcoinv1 "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"
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

func TestList(t *testing.T) {
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

func TestCreateGetDelete(t *testing.T) {
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
	// Create myNetwork
	myNetwork := &bitcoinv1.BitcoinNetwork{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BitcoinNetwork",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-test-network",
		},
		Spec: bitcoinv1.BitcoinNetworkSpec{
			Nodes: 1,
		},
	}
	_, err = client.BitcoinNetworks("default").Create(myNetwork)
	if err != nil {
		t.Errorf("Could not create test network, error is %s\n", err)
	}
	// Now get this network again
	check, err := client.BitcoinNetworks("default").Get("my-test-network", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Could not get test network, error is %s\n", err)
	}
	if check.Name != "my-test-network" {
		t.Errorf("Name is wrong, got %s, expected %s\n", check.Name, "my-test-network")
	}
	if check.Spec.Nodes != 1 {
		t.Errorf("Number of nodes is wrong, got %d, expected %d\n", check.Spec.Nodes, myNetwork.Spec.Nodes)
	}
	// And finally delete network again
	err = client.BitcoinNetworks("default").Delete("my-test-network", &metav1.DeleteOptions{})
	if err != nil {
		t.Errorf("Could not delete test network")
	}
}
