package controller

import (
	"fmt"
	"time"

	v1 "github.com/christianb93/bitcoin-controller/internal/generated/informers/externalversions/bitcoincontroller/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

// Controller is a controller for bitcoin networks
type Controller struct {
	workqueue        workqueue.Interface
	bcInformerSynced func() bool
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
func NewController(bitcoinNetworkInformer v1.BitcoinNetworkInformer) *Controller {
	controller := &Controller{
		workqueue:        workqueue.NewNamed("controllerWorkQueue"),
		bcInformerSynced: bitcoinNetworkInformer.Informer().HasSynced,
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
	if ok := cache.WaitForCacheSync(stopChan, c.bcInformerSynced); !ok {
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

// main worker loop
func (c *Controller) workerMainLoop() {
	for {
		obj, shutdown := c.workqueue.Get()
		if shutdown {
			return
		}
		klog.Infof("Got object %s from work queue\n", obj)
		c.workqueue.Done(obj)
	}
}
