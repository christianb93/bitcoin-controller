// This file contains integration tests for the controller. It assumes that
// - you have a cluster up and running
// - the configuration of the cluster is in $HOME/.kube/config
// - there is a local kubectl installed
// - the CRD has been applied
// - a bitcoin controller is running
// - RBAC has been set up for the service account running the controller
// - a secret bitcoin-default-secret has been created in the default namespace
// The test cases cover the full lifecycle of a bitcoin network - create the network,
// verify that all nodes have been brought up and are connected, scale up, scale down,
// delete

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	bitcoinv1 "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"
	clientset "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"
	versionedv1 "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned/typed/bitcoincontroller/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	numberOfNodes   = 3
	testNetworkName = "my-test-network"
	secretName      = "bitcoin-default-secret"
)

type AddedNode struct {
	Node      string `json:"addednode"`
	Connected bool   `json:"connected"`
}

// wait until the given number of pods in the bitcoin network
// has reached the status ready
func waitForPodsReady(target int32, networkName string, client versionedv1.BitcoincontrollerV1Interface, t *testing.T, timeout int) {
	count := 0
	for {
		myNetwork, err := client.BitcoinNetworks("default").Get(networkName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Could not get test network, error is %s\n", err)
			t.FailNow()
		}
		// Get status
		nodeList := myNetwork.Status.Nodes
		readyNodes := int32(0)
		for _, node := range nodeList {
			if node.Ready {
				readyNodes = readyNodes + 1
			}
		}
		if readyNodes == target {
			break
		}
		time.Sleep(5 * time.Second)
		count = count + 1
		if count*5 > timeout {
			t.Errorf("Timeout while waiting for target state of network %s, readyNodes is now %d\n", networkName, readyNodes)
		}
	}
}

func getKubeConfig() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := homedir.HomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	return kubeconfig
}

// Use kubectl locally to run the bitcoin client on a node and get its JSON output
func runLocalClient(podName string, method string, t *testing.T) []byte {
	kubeconfig := getKubeConfig()
	cmd := exec.Command("kubectl", "--kubeconfig="+kubeconfig, "exec", podName, "--", "/usr/local/bin/bitcoin-cli", "-regtest", "-conf=/bitcoin.conf", method)
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("Execution of kubectl failed, reason %s (%T)\n", err, err)
		if ee, ok := err.(*exec.ExitError); ok {
			t.Logf("Output of Stderr: %s\n", string(ee.Stderr))
		}
	}
	return output
}

// run a client on a node to get its added node list
func getAddedNodeList(podName string, t *testing.T) []AddedNode {
	// Run kubectl to connect to node
	output := runLocalClient(podName, "getaddednodeinfo", t)
	// Now parse output
	v := make([]AddedNode, 0)
	json.Unmarshal(output, &v)
	return v
}

func TestClientsetCreationIntegration(t *testing.T) {
	kubeconfig := getKubeConfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Error("Could not get configuration file")
		t.FailNow()
	}
	// Create BitcoinNetwork client set
	_, err = clientset.NewForConfig(config)
	if err != nil {
		t.Error("Could not create BitcoinNetwork clientset")
		t.FailNow()
	}
}

func TestClientsetListIntegration(t *testing.T) {
	kubeconfig := getKubeConfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Errorf("Could not get configuration file\n")
		t.FailNow()
	}
	// Create BitcoinNetwork client set
	c, err := clientset.NewForConfig(config)
	if err != nil {
		t.Errorf("Could not create BitcoinNetwork clientset\n")
		t.FailNow()
	}
	client := c.BitcoincontrollerV1()
	list, err := client.BitcoinNetworks("default").List(metav1.ListOptions{})
	if err != nil {
		t.Errorf("Could not get list of BitcoinNetworks")
		t.FailNow()
	}
	if list == nil {
		t.Errorf("List is nil\n")
	}
}

func TestLifecycleCreateIntegration(t *testing.T) {
	kubeconfig := getKubeConfig()
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
			Name: testNetworkName,
		},
		Spec: bitcoinv1.BitcoinNetworkSpec{
			Nodes:  numberOfNodes,
			Secret: secretName,
		},
	}
	_, err = client.BitcoinNetworks("default").Create(myNetwork)
	if err != nil {
		t.Errorf("Could not create test network, error is %s\n", err)
	}
	// Now get this network again
	check, err := client.BitcoinNetworks("default").Get(testNetworkName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Could not get test network, error is %s\n", err)
	}
	if check.Name != "my-test-network" {
		t.Errorf("Name is wrong, got %s, expected %s\n", check.Name, "my-test-network")
	}
	if check.Spec.Nodes != numberOfNodes {
		t.Errorf("Number of nodes is wrong, got %d, expected %d\n", check.Spec.Nodes, myNetwork.Spec.Nodes)
	}
}

func TestLifecyclePodCreationIntegration(t *testing.T) {
	kubeconfig := getKubeConfig()
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
	myNetwork, err := client.BitcoinNetworks("default").Get(testNetworkName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Could not get network, error is %s\n", err)
		t.FailNow()
	}
	// Periodically get the network that we created in the last testcase
	// and wait until all nodes are up. As the initial pull in a newly established
	// cluster might take some time, we give up only after 10 minutes
	waitForPodsReady(myNetwork.Spec.Nodes, myNetwork.Name, client, t, 600)
}

// Test that connectivity between the nodes has been established
func TestLifecycleConnectivityIntegration(t *testing.T) {
	// Wait for five more seconds, just to make sure that the full
	// sync lifecycle could complete
	time.Sleep(5 * time.Second)
	for pod := 0; pod < numberOfNodes; pod++ {
		podName := testNetworkName + "-sts-" + strconv.Itoa(pod)
		addedNodeList := getAddedNodeList(podName, t)
		if len(addedNodeList) != (numberOfNodes - 1) {
			t.Errorf("Not all nodes appear in added node list, have %d but expected %d\n", len(addedNodeList), numberOfNodes-1)
		}
	}
}

// Test that an additional pod is brought up and the node is
// added when we scale our network
func TestLifecycleScaleUpIntegration(t *testing.T) {
	// First get access to network
	kubeconfig := getKubeConfig()
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
	myNetwork, err := client.BitcoinNetworks("default").Get(testNetworkName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Could not get network, error is %s\n", err)
		t.FailNow()
	}
	// Update network. As the network is updated by the controller in parallel, we need
	// to be prepared for retries
	retries := 0
	for {
		myNetworkCopy := myNetwork.DeepCopy()
		myNetworkCopy.Spec.Nodes++
		_, err := client.BitcoinNetworks("default").Update(myNetworkCopy)
		if err == nil {
			break
		}
		retries++
		if retries > 5 {
			t.Errorf("Exceeded five retries for updating network, giving up\n")
			t.FailNow()
		}
	}
	// wait until we reached the full pod count
	waitForPodsReady(numberOfNodes+1, myNetwork.Name, client, t, 60)
	// Now check that the new node has been added
	time.Sleep(5 * time.Second)
	for pod := 0; pod < numberOfNodes+1; pod++ {
		podName := testNetworkName + "-sts-" + strconv.Itoa(pod)
		addedNodeList := getAddedNodeList(podName, t)
		if len(addedNodeList) != numberOfNodes {
			t.Errorf("Not all nodes appear in added node list, have %d but expected %d\n", len(addedNodeList), numberOfNodes)
		}
	}
}

// Test scale down
func TestLifecycleScaleDownIntegration(t *testing.T) {
	// First get access to network
	kubeconfig := getKubeConfig()
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
	myNetwork, err := client.BitcoinNetworks("default").Get(testNetworkName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Could not get network, error is %s\n", err)
		t.FailNow()
	}
	// Should have numberOfNodes plus 1 nodes
	if myNetwork.Spec.Nodes != (numberOfNodes + 1) {
		t.Errorf("Number of nodes is %d, this appears to be wrong\n", myNetwork.Spec.Nodes)
	}
	// Update network. As the network is updated by the controller in parallel, we need
	// to be prepared for retries
	retries := 0
	for {
		myNetworkCopy := myNetwork.DeepCopy()
		myNetworkCopy.Spec.Nodes--
		_, err := client.BitcoinNetworks("default").Update(myNetworkCopy)
		if err == nil {
			break
		}
		retries++
		if retries > 5 {
			t.Errorf("Exceeded five retries for updating network, giving up\n")
			t.FailNow()
		}
	}
	// wait until we reached the reduced pod count
	waitForPodsReady(numberOfNodes, myNetwork.Name, client, t, 60)
	// Now check that a node has been removed again
	time.Sleep(2 * time.Second)
	for pod := 0; pod < numberOfNodes; pod++ {
		podName := testNetworkName + "-sts-" + strconv.Itoa(pod)
		addedNodeList := getAddedNodeList(podName, t)
		if len(addedNodeList) != (numberOfNodes - 1) {
			t.Errorf("Not all nodes appear in added node list, have %d but expected %d\n", len(addedNodeList), numberOfNodes-1)
		}
	}
}

// Test deletion of the entire network
func TestLifecycleDeleteIntegration(t *testing.T) {
	kubeconfig := getKubeConfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal("Could not get configuration file")
	}
	// Create BitcoinNetwork client set
	c, err := clientset.NewForConfig(config)
	if err != nil {
		t.Fatal("Could not create BitcoinNetwork clientset")
	}
	client := c.BitcoincontrollerV1()
	// Delete network again that we created in the previous test case
	err = client.BitcoinNetworks("default").Delete("my-test-network", &metav1.DeleteOptions{})
	if err != nil {
		t.Errorf("Could not delete test network")
	}
	// wait a few seconds
	time.Sleep(5 * time.Second)
	// make sure that the stateful set has been deleted and that the
	// service has been deleted
	kubeclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal("Could not create kubernetes clientset")
	}
	svcName := testNetworkName + "-svc"
	_, err = kubeclient.CoreV1().Services("default").Get(svcName, metav1.GetOptions{})
	if err == nil {
		t.Error("Did not expect this call to succeed, service should have been deleted")
	}
	stsName := testNetworkName + "-sts"
	_, err = kubeclient.AppsV1().StatefulSets("default").Get(stsName, metav1.GetOptions{})
	if err == nil {
		t.Error("Did not expect this call to succeed, service should have been deleted")
	}
	// To leave a clean state behind, wait until the last pod has been removed
	count := 0
	for {
		_, err = kubeclient.CoreV1().Pods("default").Get(stsName+"-0", metav1.GetOptions{})
		if err != nil {
			break
		}
		time.Sleep(1 * time.Second)
		count = count + 1
		if count > 60 {
			t.Error("Timeout while waiting for last pod to go down")
		}
	}
}
