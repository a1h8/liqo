package storage

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"k8s.io/client-go/tools/cache"
)

type APICacheInterface interface {
	informer(apimgmt.ApiType) cache.SharedIndexInformer
	getApi(apimgmt.ApiType, string) (interface{}, error)
	listApi(apimgmt.ApiType) ([]interface{}, error)
	resyncListObjects(apimgmt.ApiType) ([]interface{}, error)
}

type CacheManagerAdder interface {
	AddNamespace(string, string) error
	StartNamespaces(string, string, chan struct{}) error
	RemoveNamespace(string)
	AddHomeEventHandlers(apimgmt.ApiType, string, *cache.ResourceEventHandlerFuncs) error
	AddForeignEventHandlers(apimgmt.ApiType, string, *cache.ResourceEventHandlerFuncs) error
}

type CacheManagerReader interface {
	GetHomeNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error)
	GetForeignNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error)
	ListHomeNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error)
	ListForeignNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error)
	ResyncListHomeNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error)
	ResyncListForeignNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error)
}

type CacheManagerReaderAdder interface {
	CacheManagerAdder
	CacheManagerReader
}
