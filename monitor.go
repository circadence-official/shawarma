package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// tracks the active status of each service
var activeMap = make(map[string]bool)

type monitorInfo struct {
	Namespace    string
	PodName      string
	ServiceName  string
	URL          string
	PathToConfig string
}

func processEndpoint(info *monitorInfo, endpoint *v1.Endpoints) {
	foundPod := false

	for _, subset := range endpoint.Subsets {
		for _, address := range subset.Addresses {
			if address.TargetRef != nil &&
				address.TargetRef.Kind == "Pod" &&
				address.TargetRef.Namespace == info.Namespace &&
				address.TargetRef.Name == info.PodName {
				foundPod = true
				break
			}
		}

		if foundPod {
			break
		}
	}

	// try to get active status of service, default to false if not found
	isActive, ok := activeMap[info.ServiceName]
	if !ok {
		isActive = false
	}

	if (foundPod && !isActive) || (!foundPod && isActive) {
		processStateChange(info, foundPod)
	}
}

func processStateChange(info *monitorInfo, newState bool) {
	activeMap[info.ServiceName] = newState

	logContext := log.WithFields(log.Fields{
		"svc": info.ServiceName,
		"pod": info.PodName,
		"ns":  info.Namespace,
	})

	if newState {
		logContext.Info("Activated")
	} else {
		logContext.Info("Deactivated")
	}

	go func() {
		err := notifyStateChange(info, newState)

		if err != nil {
			logContext.Error(err)
		}
	}()
}

func monitorService(info *monitorInfo) error {
	var config *rest.Config
	var err error
	if info.PathToConfig == "" {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
	} else {
		// creates from a kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", info.PathToConfig)
	}
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	for stopRequested := false; !stopRequested; {
		watchList := cache.NewFilteredListWatchFromClient(
			clientset.CoreV1().RESTClient(),
			"endpoints",
			info.Namespace,
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.SelectorFromSet(labels.Set(map[string]string{"shawarma": info.ServiceName})).String()
			},
		)

		_, controller := cache.NewInformer(
			watchList,
			&v1.Endpoints{},
			time.Second*0,
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					endpoint := obj.(*v1.Endpoints)
					notifierMonitorInfo := &monitorInfo{
						Namespace:    info.Namespace,
						PodName:      info.PodName,
						ServiceName:  endpoint.Name, // use endpoint name so that we get -preview/-canary instead of just base name
						URL:          info.URL,
						PathToConfig: info.PathToConfig,
					}

					log.Debugf("endpoint %s added", endpoint.Name)

					// default active status to false
					activeMap[endpoint.Name] = false

					processEndpoint(notifierMonitorInfo, endpoint)
				},
				DeleteFunc: func(obj interface{}) {
					endpoint := obj.(*v1.Endpoints)
					notifierMonitorInfo := &monitorInfo{
						Namespace:    info.Namespace,
						PodName:      info.PodName,
						ServiceName:  endpoint.Name, // use endpoint name so that we get -preview/-canary instead of just base name
						URL:          info.URL,
						PathToConfig: info.PathToConfig,
					}

					log.Debugf("endpoint %s deleted\n", endpoint.Name)

					if isActive, ok := activeMap[info.ServiceName]; ok {
						if isActive {
							processStateChange(notifierMonitorInfo, false)
						}
					}

					// delete from active map
					delete(activeMap, endpoint.Name)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					endpoint := newObj.(*v1.Endpoints)
					notifierMonitorInfo := &monitorInfo{
						Namespace:    info.Namespace,
						PodName:      info.PodName,
						ServiceName:  endpoint.Name, // use endpoint name so that we get -preview/-canary instead of just base name
						URL:          info.URL,
						PathToConfig: info.PathToConfig,
					}

					log.Debugf("endpoint %s changed\n", endpoint.Name)
					processEndpoint(notifierMonitorInfo, endpoint)
				},
			},
		)

		stop := make(chan struct{})

		term := make(chan os.Signal, 1)
		signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-term // wait for SIGINT or SIGTERM
			log.Debug("Shutdown signal received")
			stopRequested = true
			close(stop) // trigger the stop channel
		}()

		log.Debug("Starting controller")
		controller.Run(stop)
		log.Debug("Controller exited")

		if !stopRequested {
			log.Warn("Fail out of controller.Run, restarting...")
		}
	}

	return nil
}
