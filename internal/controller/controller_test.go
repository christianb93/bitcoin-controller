package controller_test

import (
	"fmt"
	"testing"
	"time"

	bitcoinv1 "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"
	bitcoinclient "github.com/christianb93/bitcoin-controller/internal/bitcoinclient"
	"github.com/christianb93/bitcoin-controller/internal/controller"
	bitcoinversioned "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"
	fakeBitcoin "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned/fake"
	bcinformers "github.com/christianb93/bitcoin-controller/internal/generated/informers/externalversions"
	secrets "github.com/christianb93/bitcoin-controller/internal/secrets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	fakeKubernetes "k8s.io/client-go/kubernetes/fake"
)

// This is a fake bitcoin client
type fakeBitcoinClient struct {
	nodeLists map[string][]bitcoinclient.AddedNode
}

func newFakeBitcoinClient() *fakeBitcoinClient {
	return &fakeBitcoinClient{
		nodeLists: make(map[string][]bitcoinclient.AddedNode),
	}
}

func (f *fakeBitcoinClient) RawRequest(method string, params []string, config *bitcoinclient.Config) (interface{}, error) {
	return nil, nil
}

func (f *fakeBitcoinClient) AddNode(nodeIP string, config *bitcoinclient.Config) error {
	_, ok := f.nodeLists[config.ServerIP]
	if !ok {
		return fmt.Errorf("Invalid target node IP %s\n", config.ServerIP)
	}
	f.nodeLists[config.ServerIP] = append(f.nodeLists[config.ServerIP], bitcoinclient.AddedNode{
		NodeIP:    nodeIP,
		Connected: true,
	})
	return nil
}

func (f *fakeBitcoinClient) RemoveNode(nodeIP string, config *bitcoinclient.Config) error {
	delete(f.nodeLists, nodeIP)
	return nil
}

func (f *fakeBitcoinClient) GetAddedNodes(config *bitcoinclient.Config) ([]bitcoinclient.AddedNode, error) {
	return f.nodeLists[config.ServerIP], nil
}

func (f *fakeBitcoinClient) GetConfig() bitcoinclient.Config {
	return *bitcoinclient.NewConfig("", 18332, "user", "password")
}

// a fake synced function
func alwaysReady() bool {
	return true
}

// This structure captures some data
// that we create during test setup
type testFixture struct {
	informerFactory informers.SharedInformerFactory
	controller      *controller.Controller
	stopCh          chan struct{}
	client          kubernetes.Interface
	t               *testing.T
}

// Create and populate a test network. This function will create a test
// bitcoin network, a matching secret and a controller
func basicSetup(client kubernetes.Interface, bcClient bitcoinversioned.Interface, t *testing.T) testFixture {
	// Create an informer factory for stateful sets
	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	if informerFactory == nil {
		t.Fatal("Could not create informer factory\n")
	}
	bcInformerFactory := bcinformers.NewSharedInformerFactory(bcClient, time.Second*30)
	bcInformer := bcInformerFactory.Bitcoincontroller().V1().BitcoinNetworks()
	controller := controller.NewController(
		bcInformer,
		informerFactory.Apps().V1().StatefulSets(),
		informerFactory.Core().V1().Services(),
		informerFactory.Core().V1().Pods(),
		client,
		bcClient,
	)
	if controller == nil {
		t.Fatal("Could not create controller")
	}
	myNetwork := &bitcoinv1.BitcoinNetwork{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BitcoinNetwork",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-network",
			Namespace: "test",
		},
		Spec: bitcoinv1.BitcoinNetworkSpec{
			Nodes:  4,
			Secret: "unit-test-secret",
		},
	}
	// create corresponding secret
	client.CoreV1().Secrets("test").Create(&corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-secret",
			Namespace: "test",
		},
		Data: map[string][]byte{
			secrets.UserKey:     []byte("user"),
			secrets.PasswordKey: []byte("password"),
		},
	})

	// create network in fake clientset
	bcClient.BitcoincontrollerV1().BitcoinNetworks("test").Create(myNetwork)
	// overwrite sycn functions in controller as our
	// informers do not do anything
	controller.SetBcInformerSynced(alwaysReady)
	controller.SetStsInformerSynced(alwaysReady)
	controller.SetSvcInformerSynced(alwaysReady)
	controller.SetPodInformerSynced(alwaysReady)
	// add our network to the lister so that it can be found
	// by the controllers recon function
	err := bcInformer.Informer().GetIndexer().Add(myNetwork)
	if err != nil {
		t.Fatalf("Could not add network to indexer, reason: %s\n", err)
	}
	return testFixture{
		informerFactory: informerFactory,
		controller:      controller,
		stopCh:          make(chan struct{}),
		client:          client,
		t:               t,
	}
}

// Run the controller
func (f *testFixture) runController() {
	// Start controller. This will block the current thread,
	// so we do this in a go-routine
	go f.controller.Run(f.stopCh, 1)
}

// Stop the controller
func (f *testFixture) stopController() {
	close(f.stopCh)
}

// Wait until work queue is empty
func (f *testFixture) Wait() {
	for {
		if f.controller.IsQueueProcessed() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Make sure that all objects known to the client are also
// in the informer cache
func (f *testFixture) pushObjectsToCache() {
	// Start with stateful sets. We first call Replace on the indexer so that it
	// will be empty
	f.informerFactory.Apps().V1().StatefulSets().Informer().GetIndexer().Replace(make([]interface{}, 0), "1")
	stsList, err := f.client.AppsV1().StatefulSets("test").List(metav1.ListOptions{})
	if err != nil {
		f.t.Fatalf("Unexpected error %s\n", err)
	}
	for _, sts := range stsList.Items {
		err = f.informerFactory.Apps().V1().StatefulSets().Informer().GetIndexer().Add(&sts)
		if err != nil {
			f.t.Fatalf("Unexpected error %s\n", err)
		}
	}
	// Do the same for services
	f.informerFactory.Core().V1().Services().Informer().GetIndexer().Replace(make([]interface{}, 0), "1")
	svcList, err := f.client.CoreV1().Services("test").List(metav1.ListOptions{})
	if err != nil {
		f.t.Fatalf("Unexpected error %s\n", err)
	}
	for _, svc := range svcList.Items {
		err = f.informerFactory.Core().V1().Services().Informer().GetIndexer().Add(&svc)
		if err != nil {
			f.t.Fatalf("Unexpected error %s\n", err)
		}
	}
	// and pods
	f.informerFactory.Core().V1().Pods().Informer().GetIndexer().Replace(make([]interface{}, 0), "1")
	podList, err := f.client.CoreV1().Pods("test").List(metav1.ListOptions{})
	if err != nil {
		f.t.Fatalf("Unexpected error %s\n", err)
	}
	for _, pod := range podList.Items {
		fmt.Printf("Adding pod %s in namespace %s to indexer\n", pod.Name, pod.Namespace)
		err = f.informerFactory.Core().V1().Pods().Informer().GetIndexer().Add(&pod)
		if err != nil {
			f.t.Fatalf("Unexpected error %s\n", err)
		}
		// Verify that the pod has arrived in the lister
		check, err := f.informerFactory.Core().V1().Pods().Lister().Pods(pod.Namespace).Get(pod.Name)
		if err != nil && check == nil {
			f.t.Fatalf("Could not retrieve pod that I have just added: %s\n", err)
		}

	}
}

// Helper function to create a pod and add it to a store
func (f *testFixture) createPod(name string, ip string, ready bool, t *testing.T) *corev1.Pod {
	condition := corev1.ConditionTrue
	if !ready {
		condition = corev1.ConditionFalse
	}
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test"},
		Spec: corev1.PodSpec{},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				corev1.PodCondition{
					Type:   corev1.PodReady,
					Status: condition,
				},
			},
			PodIP: ip,
		},
	}
	store := f.informerFactory.Core().V1().Pods().Informer().GetIndexer()
	err := store.Add(pod)
	if err != nil {
		t.Fatalf("Unexpected error %s\n", err)
	}
	// and add pod to client
	_, err = f.client.CoreV1().Pods("test").Create(pod)
	if err != nil {
		t.Fatalf("Unexpected error %s\n", err)
	}
	return pod
}

// Test creation of a new controller
func TestControllerCreationUnit(t *testing.T) {
	bcClient := fakeBitcoin.NewSimpleClientset()
	client := fakeKubernetes.NewSimpleClientset()
	bcInformerFactory := bcinformers.NewSharedInformerFactory(bcClient, time.Second*30)
	if bcInformerFactory == nil {
		t.Fatal("Could not create BitcoinNetwork informer factory\n")
	}
	// Create an informer factory for stateful sets
	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	if informerFactory == nil {
		t.Fatal("Could not create informer factory\n")
	}
	controller := controller.NewController(
		bcInformerFactory.Bitcoincontroller().V1().BitcoinNetworks(),
		informerFactory.Apps().V1().StatefulSets(),
		informerFactory.Core().V1().Services(),
		informerFactory.Core().V1().Pods(),
		client,
		bcClient,
	)
	if controller == nil {
		t.Fatal("Could not create controller")
	}
}

// Test add  handler. We invoke this handler to simulate the scenario that a
// new bitcoin controller has been added and verify that a new stateful set
// and a new service have been created
func TestAddHandlerUnit(t *testing.T) {
	bcClient := fakeBitcoin.NewSimpleClientset()
	client := fakeKubernetes.NewSimpleClientset()
	fixture := basicSetup(client, bcClient, t)
	// call the AddHandler as the informer would do it
	myNetwork, err := bcClient.BitcoincontrollerV1().BitcoinNetworks("test").Get("unit-test-network", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error %s\n", err)
	}
	fixture.controller.AddBitcoinNetwork(myNetwork)
	// Run controller
	fixture.runController()
	// and give the worker function some time to grab the update
	// from the queue
	fixture.Wait()
	// Now verify that a stateful set and a service have been created
	// in the namespace test
	sts, err := client.AppsV1().StatefulSets("test").Get("unit-test-network-sts", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Got unexpected error %s\n", err)
	}
	if *sts.Spec.Replicas != int32(4) {
		t.Errorf("Found %d replicas in stateful set, expected %d\n", sts.Spec.Replicas, 4)
	}
	// Verify correct labels
	if sts.Spec.Selector.MatchLabels["app"] != "unit-test-network" {
		t.Errorf("Incorrect label %s\n", sts.Spec.Selector.MatchLabels["app"])
	}
	// Verify correct image
	if len(sts.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("Expected one container in spec, found %d\n", len(sts.Spec.Template.Spec.Containers))
	}
	container := sts.Spec.Template.Spec.Containers[0]
	if container.Image != "christianb93/bitcoind:latest" {
		t.Errorf("Have image name %s, expected christianb93/bitcoind:latest\n", container.Image)
	}
	// Verify that the environment variables are mapped into this container as defined by
	// the secret
	if container.EnvFrom[0].SecretRef.LocalObjectReference.Name != "unit-test-secret" {
		t.Error("Expected reference to secret")
	}
	if *container.EnvFrom[0].SecretRef.Optional != true {
		t.Error("Expected secret mapping to be optional")
	}
	// Verify that there is a headless service which is referenced by the stateful set
	if sts.Spec.ServiceName != "unit-test-network-svc" {
		t.Errorf("Wrong service name %s in stateful set specification\n", sts.Spec.ServiceName)
	}
	svc, err := client.CoreV1().Services("test").Get("unit-test-network-svc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Got unexpected error %s\n", err)
	}
	if svc.Name != "unit-test-network-svc" {
		t.Errorf("Unexpected service name %s\n", svc.Name)
	}
	if svc.Spec.ClusterIP != "None" {
		t.Errorf("Expected headless service, but found clusterIP %s\n", svc.Spec.ClusterIP)
	}
	fixture.stopController()
}

// Test that the status field is correctly maintained
func TestUpdateStatusUnit(t *testing.T) {
	bcClient := fakeBitcoin.NewSimpleClientset()
	client := fakeKubernetes.NewSimpleClientset()
	fixture := basicSetup(client, bcClient, t)
	// Inject fake bitcoin controller
	myFakeBitcoinClient := newFakeBitcoinClient()
	fixture.controller.SetRPCClient(myFakeBitcoinClient)
	// Prepare bitcoin client to return an empty node list
	myFakeBitcoinClient.nodeLists["10.0.0.1"] = []bitcoinclient.AddedNode{}
	myFakeBitcoinClient.nodeLists["10.0.0.2"] = []bitcoinclient.AddedNode{}
	myFakeBitcoinClient.nodeLists["10.0.0.3"] = []bitcoinclient.AddedNode{}
	// add our network to the lister so that it can be found
	// by the controllers recon function
	myNetwork, err := bcClient.BitcoincontrollerV1().BitcoinNetworks("test").Get("unit-test-network", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Could not get test network: %s\n", err)
	}
	// Start controller
	fixture.runController()
	// now call the Add handler
	fixture.controller.AddBitcoinNetwork(myNetwork)
	// and give the worker function some time to grab the update
	// from the queue
	fixture.Wait()
	// make sure that the objects that the
	// controller has created are moved into the cache
	fixture.pushObjectsToCache()
	// at this point, there are no pods, so the status should still be empty
	bcNetwork, err := bcClient.BitcoincontrollerV1().BitcoinNetworks("test").Get("unit-test-network", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Got unexpected error %s\n", err)
	}
	if len(bcNetwork.Status.Nodes) != 0 {
		t.Errorf("Got unexpected number of nodes %d, should be zero as there are no pods yet\n", len(bcNetwork.Status.Nodes))
	}
	// Now create three pods - two being ready, one not
	fixture.createPod("unit-test-network-sts-0", "10.0.0.1", true, t)
	fixture.createPod("unit-test-network-sts-1", "10.0.0.2", true, t)
	fixture.createPod("unit-test-network-sts-2", "10.0.0.3", false, t)
	// and run update function
	client.ClearActions()
	bcClient.ClearActions()
	fixture.controller.UpdateBitcoinNetwork(bcNetwork, bcNetwork)
	fixture.Wait()
	// Now check status
	bcNetwork, err = bcClient.BitcoincontrollerV1().BitcoinNetworks("test").Get("unit-test-network", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error %s\n", err)
	}
	if len(bcNetwork.Status.Nodes) != 3 {
		t.Errorf("Unexpected length of status list\n")
	}
	// Turn node list into a map
	check := make(map[string]bool)
	for _, node := range bcNetwork.Status.Nodes {
		check[node.IP] = node.Ready
	}
	// Node 0 and Node 1 should be ready
	if !check["10.0.0.1"] || !check["10.0.0.2"] {
		t.Errorf("Node does not have expected status\n")
	}
	// Node 2 should not be ready
	if check["10.0.0.3"] {
		t.Errorf("Node does not have expected status\n")
	}
	// Now verify that the node lists have actually been updated
	if len(myFakeBitcoinClient.nodeLists["10.0.0.1"]) != 1 {
		t.Errorf("Node 10.0.0.1 has unexpected number %d of added nodes\n", len(myFakeBitcoinClient.nodeLists["10.0.0.1"]))
	}
	if myFakeBitcoinClient.nodeLists["10.0.0.1"][0].NodeIP != "10.0.0.2" {
		t.Errorf("Node 10.0.0.1 has incorrect node %s in its added node list\n", myFakeBitcoinClient.nodeLists["10.0.0.1"][0].NodeIP)
	}
	if len(myFakeBitcoinClient.nodeLists["10.0.0.2"]) != 1 {
		t.Errorf("Node 10.0.0.2 has unexpected number %d of added nodes\n", len(myFakeBitcoinClient.nodeLists["10.0.0.1"]))
	}
	if myFakeBitcoinClient.nodeLists["10.0.0.2"][0].NodeIP != "10.0.0.1" {
		t.Errorf("Node 10.0.0.2 has incorrect node %s in its added node list\n", myFakeBitcoinClient.nodeLists["10.0.0.1"][0].NodeIP)
	}
}

// Additional testcases TBD:
// test events
// test scale up and down
// test change of stateful set
// if deletion timestamp is set updates will be ignored
