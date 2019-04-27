package controller

import (
	"fmt"
	"time"

	bcv1 "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"
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
	bcLister          bcListers.BitcoinNetworkLister
	stsLister         appsv1Listers.StatefulSetLister
	svcLister         corev1Listers.ServiceLister
	clientset         *kubernetes.Clientset
}

// AddBitcoinNetwork is the event handler for ADD
func (c *Controller) AddBitcoinNetwork(obj interface{}) {
	klog.Infof("ADD handler called\n")
	c.enqueue(obj)
}

// DeleteBitcoinNetwork is the event handler for ADD
func (c *Controller) DeleteBitcoinNetwork(obj interface{}) {
	klog.Infof("DELETE handler called\n")
	c.enqueue(obj)
}

// UpdateBitcoinNetwork is the event handler for MOD
func (c *Controller) UpdateBitcoinNetwork(old interface{}, new interface{}) {
	klog.Infof("UPDATE handler called\n")
	c.enqueue(new)
}

// NewController creates a new bitcoin controller
func NewController(bitcoinNetworkInformer bcInformers.BitcoinNetworkInformer,
	stsInformer appsv1Informers.StatefulSetInformer,
	svcInformer corev1Informers.ServiceInformer,
	clientset *kubernetes.Clientset) *Controller {
	controller := &Controller{
		workqueue:         workqueue.NewNamed("controllerWorkQueue"),
		bcInformerSynced:  bitcoinNetworkInformer.Informer().HasSynced,
		stsInformerSynced: stsInformer.Informer().HasSynced,
		svcInformerSynced: svcInformer.Informer().HasSynced,
		bcLister:          bitcoinNetworkInformer.Lister(),
		stsLister:         stsInformer.Lister(),
		svcLister:         svcInformer.Lister(),
		clientset:         clientset,
	}
	// Set up event handler
	bitcoinNetworkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.AddBitcoinNetwork,
		DeleteFunc: controller.DeleteBitcoinNetwork,
		UpdateFunc: controller.UpdateBitcoinNetwork,
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

// Run starts the controller main loop and all workers
func (c *Controller) Run(stopChan <-chan struct{}, threads int) error {
	klog.Info("Waiting for cache to sync")
	if ok := cache.WaitForCacheSync(stopChan, c.bcInformerSynced, c.stsInformerSynced, c.svcInformerSynced); !ok {
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
	klog.Infof("Created deep copy of stateful set %s\n", stsCopy.Name)
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
