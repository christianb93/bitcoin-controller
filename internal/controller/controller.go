package controller

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	bcv1 "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"
	bitcoinclient "github.com/christianb93/bitcoin-controller/internal/bitcoinclient"
	bcversioned "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"
	bitcoinscheme "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned/scheme"
	bcInformers "github.com/christianb93/bitcoin-controller/internal/generated/informers/externalversions/bitcoincontroller/v1"
	bcListers "github.com/christianb93/bitcoin-controller/internal/generated/listers/bitcoincontroller/v1"
	secrets "github.com/christianb93/bitcoin-controller/internal/secrets"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1Informers "k8s.io/client-go/informers/apps/v1"
	corev1Informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appsv1Listers "k8s.io/client-go/listers/apps/v1"
	corev1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

//
// These prefixes are added to the name of a bitcoin network to derive the name
// of the corresponding stateful set and headless service
//
const (
	stsPostfix = "-sts"
	svcPostfix = "-svc"
)

// Controller is a controller for bitcoin networks
type Controller struct {
	workqueue         workqueue.Interface
	bcInformerSynced  func() bool
	stsInformerSynced func() bool
	svcInformerSynced func() bool
	podInformerSynced func() bool
	bcLister          bcListers.BitcoinNetworkLister
	stsLister         appsv1Listers.StatefulSetLister
	svcLister         corev1Listers.ServiceLister
	podLister         corev1Listers.PodLister
	clientset         kubernetes.Interface
	bcClientset       bcversioned.Interface
	rpcClient         bitcoinclient.BitcoinClient
	recorder          record.EventRecorder
	broadcaster       record.EventBroadcaster
}

// AddBitcoinNetwork is the event handler for ADD
func (c *Controller) AddBitcoinNetwork(obj interface{}) {
	c.enqueue(obj)
}

// UpdateBitcoinNetwork is the event handler for MOD
func (c *Controller) UpdateBitcoinNetwork(old interface{}, new interface{}) {
	c.enqueue(new)
}

// UpdateObject is a generic update handler for all other objects
func (c *Controller) UpdateObject(old interface{}, new interface{}) {
	c.handleObject(new)
}

// SetBcInformerSynced sets the function to check whether the informer is synced
// should be used for tests only
func (c *Controller) SetBcInformerSynced(f func() bool) {
	c.bcInformerSynced = f
}

// SetStsInformerSynced sets the function to check whether the informer is synced
// should be used for tests only
func (c *Controller) SetStsInformerSynced(f func() bool) {
	c.stsInformerSynced = f
}

// SetSvcInformerSynced sets the function to check whether the informer is synced
// should be used for tests only
func (c *Controller) SetSvcInformerSynced(f func() bool) {
	c.svcInformerSynced = f
}

// SetPodInformerSynced sets the function to check whether the informer is synced
// should be used for tests only
func (c *Controller) SetPodInformerSynced(f func() bool) {
	c.podInformerSynced = f
}

// SetRPCClient can be used during testing to inject a fake bitcoin client
func (c *Controller) SetRPCClient(rpcClient bitcoinclient.BitcoinClient) {
	c.rpcClient = rpcClient
}

// IsQueueProcessed returns true if there are no unprocessed items in the
// queue. Mainly used for testing
func (c *Controller) IsQueueProcessed() bool {
	return c.workqueue.Len() == 0
}

// NewController creates a new bitcoin controller
func NewController(bitcoinNetworkInformer bcInformers.BitcoinNetworkInformer,
	stsInformer appsv1Informers.StatefulSetInformer,
	svcInformer corev1Informers.ServiceInformer,
	podInformer corev1Informers.PodInformer,
	clientset kubernetes.Interface,
	bcClientset bcversioned.Interface) *Controller {
	klog.Info("Creating event broadcaster")
	// Add bitcoin-controller types to the default Kubernetes Scheme so Events can be
	// logged
	runtime.Must(bitcoinscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "bitcoin-controller"})
	controller := &Controller{
		workqueue:         workqueue.NewNamed("controllerWorkQueue"),
		bcInformerSynced:  bitcoinNetworkInformer.Informer().HasSynced,
		stsInformerSynced: stsInformer.Informer().HasSynced,
		svcInformerSynced: svcInformer.Informer().HasSynced,
		podInformerSynced: podInformer.Informer().HasSynced,
		bcLister:          bitcoinNetworkInformer.Lister(),
		stsLister:         stsInformer.Lister(),
		svcLister:         svcInformer.Lister(),
		podLister:         podInformer.Lister(),
		clientset:         clientset,
		bcClientset:       bcClientset,
		rpcClient: bitcoinclient.NewClientForConfig(&bitcoinclient.Config{
			RPCUser:     secrets.DefaultRPCUser,
			RPCPassword: secrets.DefaultRPCPassword,
			ServerPort:  18332,
			ServerIP:    "127.0.0.1",
		}),
		recorder:    recorder,
		broadcaster: eventBroadcaster,
	}
	// Set up event handler
	bitcoinNetworkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.AddBitcoinNetwork,
		UpdateFunc: controller.UpdateBitcoinNetwork,
	})
	stsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.UpdateObject,
	})
	svcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.UpdateObject,
	})
	return controller
}

// AddEventSink adds an event sink to the controller. This allows us to inject
// a mock object as a workaround for 	// https://github.com/kubernetes/client-go/issues/493
// during testing
func (c *Controller) AddEventSink(eventSink record.EventSink) {
	// We create the Events interface in the "all" namespace, but the events will inherit
	// the namespace of the object to which they refer
	c.broadcaster.StartRecordingToSink(eventSink)
}

// enqueue converts a BitcoinNetwork into a namespace / name
// string and puts that into the queue
func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error("Could not extract name and namespace")
		return
	}
	c.workqueue.Add(key)
}

// This function is invoked when a dependent object (stateful set, service) changes.
// It will simply determine the owning network and enqueue this network
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		klog.Error("Could not convert object, giving up")
	}
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != "BitcoinNetwork" {
			return
		}
		bcNetwork, err := c.bcLister.BitcoinNetworks(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.Infof("ignoring orphaned object '%s' of bitcoin network '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}
		c.enqueue(bcNetwork)
		return
	}
}

// Run starts the controller main loop and all workers
func (c *Controller) Run(stopChan <-chan struct{}, threads int) error {
	klog.Info("Waiting for cache to sync")
	if ok := cache.WaitForCacheSync(stopChan,
		c.bcInformerSynced,
		c.stsInformerSynced,
		c.svcInformerSynced,
		c.podInformerSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	klog.Info("Starting workers")
	for i := 0; i < threads; i++ {
		// this will run workerMainLoop every second, until stopChan is closed
		go wait.Until(c.workerMainLoop, time.Second, stopChan)
	}
	klog.Info("Controller main loop - waiting for stop channel")
	<-stopChan
	return nil
}

//
// Update stateful set  with the given number of replicas
//
func (c *Controller) updateStatefulSet(sts *appsv1.StatefulSet, replicaCount int32) {
	// Create a deep clone first
	stsCopy := sts.DeepCopy()
	stsCopy.Spec.Replicas = &replicaCount
	_, err := c.clientset.AppsV1().StatefulSets(stsCopy.Namespace).Update(stsCopy)
	if err != nil {
		klog.Errorf("Could not update replica set, error is %s\n", err)
	}
}

//
// Create a stateful set for a given bitcoin network
//
func (c *Controller) createStatefulSet(bcNetwork *bcv1.BitcoinNetwork, stsName string, svcName string) *appsv1.StatefulSet {
	klog.Infof("Creating stateful set %s for bitcoin network %s\n", stsName, bcNetwork.Name)
	sts := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Statefulset",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      stsName,
			Namespace: bcNetwork.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bcNetwork, bcv1.SchemeGroupVersion.WithKind("BitcoinNetwork")),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &bcNetwork.Spec.Nodes,
			ServiceName: svcName,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": bcNetwork.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: bcNetwork.Namespace,
					Labels:    map[string]string{"app": bcNetwork.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:            stsName + "-ctr",
							Image:           "christianb93/bitcoind:v1.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"/usr/local/bin/bitcoin-cli",
											"-regtest",
											"-conf=/bitcoin.conf",
											"getnetworkinfo"}},
								},
							},
						},
					},
				},
			},
		},
	}
	// If the bitcoin network has a non-default secret, refer to this secret in the container
	// definition as well
	optional := true
	if bcNetwork.Spec.Secret != "" {
		klog.Infof("Using secret %s\n", bcNetwork.Spec.Secret)
		sts.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					Optional: &optional,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: bcNetwork.Spec.Secret,
					},
				},
			},
		}
	}
	stsResult, err := c.clientset.AppsV1().StatefulSets(bcNetwork.Namespace).Create(sts)
	if err != nil {
		// Ignore duplicates
		if errors.IsAlreadyExists(err) {
			klog.Infof("Stateless set %s does already exist, ignoring\n", stsName)
		} else {
			klog.Errorf("Could not create stateful set %s, error is %s\n", stsName, err)
			c.recorder.Event(bcNetwork, corev1.EventTypeWarning, "CreationError",
				fmt.Sprintf("Could not create stateful set %s, error is %s\n", stsName, err))
		}
		return nil
	}
	c.recorder.Event(bcNetwork, corev1.EventTypeNormal, "Info",
		fmt.Sprintf("Created stateful set for bitcoin network  %s\n", bcNetwork.Name))
	return stsResult
}

//
// Create a headless service for a given bitcoin network
//
func (c *Controller) createHeadlessService(bcNetwork *bcv1.BitcoinNetwork, svcName string) {
	klog.Infof("Creating headless service %s for bitcoin network %s\n", svcName, bcNetwork.Name)
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: bcNetwork.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bcNetwork, bcv1.SchemeGroupVersion.WithKind("BitcoinNetwork")),
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  map[string]string{"app": bcNetwork.Name},
		},
	}
	_, err := c.clientset.CoreV1().Services(bcNetwork.Namespace).Create(service)
	if err != nil {
		// Ignore duplicates
		if errors.IsAlreadyExists(err) {
			klog.Infof("Service %s does already exist, ignoring\n", svcName)
		} else {
			klog.Errorf("Could not create service %s, error is %s\n", svcName, err)
			c.recorder.Event(bcNetwork, corev1.EventTypeWarning, "CreationError",
				fmt.Sprintf("Could not service %s, error is %s\n", svcName, err))
		}
		return
	}
	c.recorder.Event(bcNetwork, corev1.EventTypeNormal, "Info",
		fmt.Sprintf("Created headless service for bitcoin network  %s\n", bcNetwork.Name))
}

// Helper function to determine whether a pod is ready
func podReady(pod *corev1.Pod) bool {
	result := false
	// We need to look at the pods conditions
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			if cond.Status == corev1.ConditionTrue {
				result = true
			}
		}
	}
	return result
}

// Update the status of the bitcoin network
func (c *Controller) updateNetworkStatus(bcNetwork *bcv1.BitcoinNetwork, stsName string, svcName string) []bcv1.BitcoinNetworkNode {
	var nodeName string
	var nodeList []bcv1.BitcoinNetworkNode
	var node bcv1.BitcoinNetworkNode
	// We need to get all pods that are part of this stateful set - we simply do this
	// by name
	podNamespaceLister := c.podLister.Pods(bcNetwork.Namespace)
	if podNamespaceLister == nil {
		klog.Errorf("Could not get pod namespace lister for namespace %s, giving up\n", bcNetwork.Namespace)
	}
	for i := 0; i < int(bcNetwork.Spec.Nodes); i++ {
		nodeName = stsName + "-" + strconv.FormatInt(int64(i), 10)
		pod, err := podNamespaceLister.Get(nodeName)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Unexpected error during GET of pod %s\n", nodeName)
				return nil
			}
		} else {
			// Have a pod. Find out more about it and add it to the map
			node.Ordinal = int32(i)
			node.Ready = podReady(pod)
			node.IP = pod.Status.PodIP
			node.NodeName = pod.Name
			node.DNSName = pod.Name + "." + svcName
			nodeList = append(nodeList, node)
		}
	}
	// Make sure to create a deep copy to update the status
	bcNetworkCopy := bcNetwork.DeepCopy()
	bcNetworkCopy.Status = bcv1.BitcoinNetworkStatus{
		Nodes: nodeList,
	}
	_, err := c.bcClientset.BitcoincontrollerV1().BitcoinNetworks(bcNetwork.Namespace).UpdateStatus(bcNetworkCopy)
	if err != nil {
		klog.Errorf("Error %s during UpdateStatus for bitcoin network %s\n", err, bcNetwork.Name)
	}
	return nodeList
}

// connectNode connects a node to all other nodes that appear as
// keys in the nodeList. The parameter config is expected to be
// a complete config to connect to the target node
func (c *Controller) connectNode(config *bitcoinclient.Config, nodeList map[string]struct{}) {
	// First we get a map of all nodes to which this node has been added
	addedNodes, err := c.rpcClient.GetAddedNodes(config)
	if err != nil {
		klog.Errorf("Could not get added node list from node %s, error message is %s\n", config.ServerIP, err)
		return
	}
	// put those IP addressses into a map as well
	addedNodesMap := make(map[string]struct{})
	for _, addedNode := range addedNodes {
		addedNodesMap[addedNode.NodeIP] = struct{}{}
	}
	// and add the target IP itself - this should now be in both maps
	addedNodesMap[config.ServerIP] = struct{}{}
	// First go through all nodes that appear in nodeList but
	// not in addedNodesMap and add them
	for node := range nodeList {
		_, exists := addedNodesMap[node]
		if !exists {
			klog.Infof("IP %s does not appear in added node list of %s, needs to be added\n", node, config.ServerIP)
			err = c.rpcClient.AddNode(node, config)
			if err != nil && !bitcoinclient.IsNodeAlreadyAdded(err) {
				klog.Errorf("Could not add node %s to %s, error is %s\n", node, config.ServerIP, err)
			}
		}
	}
	// Then go through all nodes that have been added before but do not
	// seem to be ready any more
	for node := range addedNodesMap {
		_, exists := nodeList[node]
		if !exists {
			klog.Infof("IP %s is in added node list of %s but not ready, needs to be removed\n", node, config.ServerIP)
			err = c.rpcClient.RemoveNode(node, config)
			if err != nil && !bitcoinclient.IsNodeNotAdded(err) {
				klog.Errorf("Could not remove node %s from %s, error is %s\n", node, config.ServerIP, err)
			}
		}
	}
}

// syncNode connects all nodes of the bitcoin network to each other
func (c *Controller) syncNodes(nodeList []bcv1.BitcoinNetworkNode, secretName string, secretNamespace string) {
	// We create a hash map that only contains the IP addresses of the
	// nodes which are ready
	readyNodes := make(map[string]struct{})
	for _, node := range nodeList {
		if node.Ready {
			readyNodes[node.IP] = struct{}{}
		}
	}
	// Create a configuration template. We make a copy
	// of the default configuration first which will
	// give us default credentials and the correct port
	config := c.rpcClient.GetConfig()
	// get credentials
	user, password, err := secrets.CredentialsForSecret(secretName, secretNamespace, c.clientset)
	if err != nil {
		klog.Infof("Could not get credentials, using defaults")
	} else {
		config.RPCUser = user
		config.RPCPassword = password
	}
	// We now go throught the list node by node
	for nodeIP := range readyNodes {
		config.ServerIP = nodeIP
		c.connectNode(&config, readyNodes)
	}
}

// Main reconciliation function of our controller. If this is called
// for a specific controller (referenced via the key), we will
// - verify that a headless service exists and create one if needed
// - verify that a stateful set exists and create one if needed
// - check for differences in the number of replicas between the stateful set
//   and the bitcoin network and adjust the stateful set if needed
func (c *Controller) doRecon(key string) bool {
	// Split object key into name and determine the name of the
	// headless service and the stateful set
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("Could not split key %s into name and namespace\n")
		return false
	}
	svcName := name + svcPostfix
	stsName := name + stsPostfix
	// Get the bitcoin network object in question
	bcNetwork, err := c.bcLister.BitcoinNetworks(namespace).Get(name)
	if err != nil {
		klog.Infof("Could not retrieve bitcoin network from cache - already deleted?")
		return true
	}
	// If the deletion timestamp is set, this bitcoin network is scheduled for
	// deletion and we ignore it
	if bcNetwork.DeletionTimestamp != nil {
		klog.Infof("Deletion timestamp already set for this bitcoin network, ignoring\n")
		return true
	}
	_, err = c.svcLister.Services(namespace).Get(svcName)
	if err != nil {
		if errors.IsNotFound(err) {
			// The service does not yet seem to exist, create it
			c.createHeadlessService(bcNetwork, svcName)
		} else {
			klog.Infof("Could not get headless service, error %s\n", err, err)
			return true
		}
	}
	// Now do the same for the stateless set
	sts, err := c.stsLister.StatefulSets(namespace).Get(stsName)
	if err != nil {
		if errors.IsNotFound(err) {
			sts = c.createStatefulSet(bcNetwork, stsName, svcName)
		} else {
			klog.Infof("Could not get stateful set, error %s\n", err, err)
			return true
		}
	}
	// Now check whether we have the same number of replicas in both entities
	if sts != nil {
		if *sts.Spec.Replicas != bcNetwork.Spec.Nodes {
			klog.Infof("Adjusting number of replicas to %d\n", bcNetwork.Spec.Nodes)
			c.recorder.Event(bcNetwork, corev1.EventTypeNormal, "Info",
				fmt.Sprintf("Adjusting number of replicas to %d\n", bcNetwork.Spec.Nodes))
			c.updateStatefulSet(sts, bcNetwork.Spec.Nodes)
		}
	}
	// Finally update the status. We first get the old node list to be able to
	// see whether there was a change
	oldNodeList := bcNetwork.Status.Nodes
	newNodeList := c.updateNetworkStatus(bcNetwork, stsName, svcName)
	if !reflect.DeepEqual(oldNodeList, newNodeList) {
		c.syncNodes(newNodeList, bcNetwork.Spec.Secret, bcNetwork.Namespace)
	}
	return true
}

// main worker loop
func (c *Controller) workerMainLoop() {
	for {
		obj, shutdown := c.workqueue.Get()
		if shutdown {
			return
		}
		// Convert obj to string
		key, ok := obj.(string)
		if !ok {
			klog.Error("Got something from the queue which is not a string")
			c.workqueue.Done(obj)
		} else {
			ok := c.doRecon(key)
			if ok {
				c.workqueue.Done(obj)
			}
		}
	}
}
