package client

import (
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

var once sync.Once
var informerFactory informers.SharedInformerFactory

func NewSharedInformerFactory(
	client kubernetes.Interface,
	defaultResync time.Duration,
) informers.SharedInformerFactory {
	once.Do(func() {
		informerFactory = informers.NewSharedInformerFactory(client, defaultResync)
	})
	return informerFactory
}
