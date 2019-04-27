package controller

import (
	"fmt"
	"strconv"
	"time"

	bcv1 "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"
	bcversioned "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"
	bcInformers "github.com/christianb93/bitcoin-controller/internal/generated/informers/externalversions/bitcoincontroller/v1"
	bcListers "github.com/christianb93/bitcoin-controller/internal/generated/listers/bitcoincontroller/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1Informers "k8s.io/client-go/informers/apps/v1"
	corev1Informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	appsv1Listers "k8s.io/client-go/listers/apps/v1"
	corev1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
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
	clientset         *kubernetes.Clientset
	bcClientset       *bcversioned.Clientset
}

// AddBitcoinNetwork is the event handler for ADD
func (c *Controller) AddBitcoinNetwork(obj interface{}) {
	klog.Infof("ADD handler called\n")
	c.enqueue(obj)
}

// UpdateBitcoinNetwork is the event handler for MOD
func (c *Controller) UpdateBitcoinNetwork(old interface{}, new interface{}) {
	klog.Infof("UPDATE handler called\n")
	c.enqueue(new)
}

// UpdateObject is a generic update handler for all other objects
func (c *Controller) UpdateObject(old interface{}, new interface{}) {
	c.handleObject(new)
}

// NewController creates a new bitcoin controller
func NewController(bitcoinNetworkInformer bcInformers.BitcoinNetworkInformer,
	stsInformer appsv1Informers.StatefulSetInformer,
	svcInformer corev1Informers.ServiceInformer,
	podInformer corev1Informers.PodInformer,
	clientset *kubernetes.Clientset,
	bcClientset *bcversioned.Clientset) *Controller {
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
	klog.Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != "BitcoinNetwork" {
			return
		}
		bcNetwork, err := c.bcLister.BitcoinNetworks(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.Infof("ignoring orphaned object '%s' of bitcoin network '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}
		klog.Infof("Queuing owning object %s\n", bcNetwork.Name)
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
							Image:           "christianb93/bitcoind:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: 10,
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
	stsResult, err := c.clientset.AppsV1().StatefulSets(bcNetwork.Namespace).Create(sts)
	if err != nil {
		// Ignore duplicates
		if errors.IsAlreadyExists(err) {
			klog.Infof("Stateless set %s does already exist, ignoring\n", stsName)
		} else {
			klog.Errorf("Could not create stateful set %s, error is %s\n", stsName, err)
		}
		return nil
	}
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
		}
	}
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
func (c *Controller) updateNetworkStatus(bcNetwork *bcv1.BitcoinNetwork, stsName string, svcName string) {
	var nodeName string
	var nodeList []bcv1.BitcoinNetworkNode
	var node bcv1.BitcoinNetworkNode
	klog.Infof("Updating status information for bitcoin network %s\n", bcNetwork.Name)
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
				return
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
	klog.Infof("Doing reconciliation for bitcoin-controller %s in namespace %s\n", name, namespace)
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
			c.updateStatefulSet(sts, bcNetwork.Spec.Nodes)
		}
	}
	// Finally update the status
	c.updateNetworkStatus(bcNetwork, stsName, svcName)
	return true
}

// main worker loop
func (c *Controller) workerMainLoop() {
	for {
		obj, shutdown := c.workqueue.Get()
		if shutdown {
			return
		}
		klog.Infof("Got object %s from work queue\n", obj)
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
