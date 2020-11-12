package storage

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/pkg/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type Manager struct {
	homeInformers    *NamespacedAPICaches
	foreignInformers *NamespacedAPICaches
}

func NewManager(homeClient, foreignClient kubernetes.Interface) *Manager {
	homeInformers := &NamespacedAPICaches{
		apiInformers:      make(map[string]*APICaches),
		informerFactories: make(map[string]informers.SharedInformerFactory),
		client:            homeClient,
		mutex:             sync.RWMutex{},
	}

	foreignInformers := &NamespacedAPICaches{
		apiInformers:      make(map[string]*APICaches),
		informerFactories: make(map[string]informers.SharedInformerFactory),
		client:            foreignClient,
		mutex:             sync.RWMutex{},
	}

	manager := &Manager{
		homeInformers:    homeInformers,
		foreignInformers: foreignInformers,
	}

	return manager
}

func (cm *Manager) AddNamespace(homeNamespace, foreignNamespace string) error {
	if cm.homeInformers == nil {
		return errors.New("home informers set to nil")
	}
	if cm.foreignInformers == nil {
		return errors.New("foreign informers set to nil")
	}

	if err := cm.homeInformers.AddNamespace(homeNamespace); err != nil {
		return err
	}
	return cm.foreignInformers.AddNamespace(foreignNamespace)
}

func (cm *Manager) StartNamespaces(homeNamespace, foreignNamespace string, stop chan struct{}) error {
	if cm.homeInformers == nil {
		return errors.New("home informers set to nil")
	}
	if cm.foreignInformers == nil {
		return errors.New("foreign informers set to nil")
	}

	if err := cm.homeInformers.startNamespace(homeNamespace, stop); err != nil {
		return err
	}

	return cm.foreignInformers.startNamespace(foreignNamespace, stop)
}

func (cm *Manager) RemoveNamespace(namespace string) {
	cm.homeInformers.removeNamespace(namespace)
	cm.foreignInformers.removeNamespace(namespace)
}

func (cm *Manager) AddHomeEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) error {
	cm.homeInformers.mutex.Lock()
	defer cm.homeInformers.mutex.Unlock()

	if cm.homeInformers == nil {
		return errors.New("home informer set to nil")
	}

	informer := cm.homeInformers.Namespace(namespace).informer(api)
	if informer == nil {
		return errors.Errorf("cannot set handlers, home informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace)
	}

	informer.AddEventHandler(handlers)

	return nil
}

func (cm *Manager) AddForeignEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) error {
	cm.foreignInformers.mutex.Lock()
	defer cm.foreignInformers.mutex.Unlock()

	if cm.homeInformers == nil {
		return errors.New("foreign informer set to nil")
	}

	informer := cm.foreignInformers.Namespace(namespace).informer(api)
	if informer == nil {
		return errors.Errorf("cannot set handlers, foreign informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace)
	}

	informer.AddEventHandler(handlers)

	return nil
}

func (cm *Manager) GetHomeNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error) {
	if cm.homeInformers == nil {
		return nil, errors.New("home informers set to nil")
	}

	cm.homeInformers.mutex.RLock()
	defer cm.homeInformers.mutex.RUnlock()

	return cm.homeInformers.Namespace(namespace).getApi(api, key)
}

func (cm *Manager) GetForeignNamespacedObject(api apimgmt.ApiType, namespace, key string) (interface{}, error) {
	if cm.foreignInformers == nil {
		return nil, errors.New("foreign informers set to nil")
	}

	cm.foreignInformers.mutex.RLock()
	defer cm.foreignInformers.mutex.RUnlock()

	return cm.foreignInformers.Namespace(namespace).getApi(api, key)
}

func (cm *Manager) ListHomeNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.homeInformers == nil {
		return nil, errors.New("home informers set to nil")
	}

	cm.homeInformers.mutex.RLock()
	defer cm.homeInformers.mutex.RUnlock()

	objects, err := cm.homeInformers.Namespace(namespace).listApi(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ListForeignNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.foreignInformers == nil {
		return nil, errors.New("foreign informers set to nil")
	}

	cm.foreignInformers.mutex.RLock()
	defer cm.foreignInformers.mutex.RUnlock()

	objects, err := cm.foreignInformers.Namespace(namespace).listApi(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ResyncListHomeNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.homeInformers == nil {
		return nil, errors.New("home informers set to nil")
	}

	cm.homeInformers.mutex.RLock()
	defer cm.homeInformers.mutex.RLock()

	apiCache := cm.homeInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace)
	}

	objects, err := apiCache.resyncListObjects(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ResyncListForeignNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.foreignInformers == nil {
		return nil, errors.New("foreign informers set to nil")
	}

	cm.foreignInformers.mutex.RLock()
	defer cm.foreignInformers.mutex.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace)
	}

	objects, err := apiCache.resyncListObjects(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}
