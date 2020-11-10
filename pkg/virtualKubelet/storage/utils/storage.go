package utils

import (
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/cache"
)

func Keyer(namespace, name string) string {
	return fmt.Sprintf("%v/%v", namespace, name)
}

func GetObject(informer cache.SharedIndexInformer, key string) (interface{}, error) {
	if informer == nil {
		return nil, errors.New("informer not yet instantiated")
	}

	obj, exists, err := informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, errors.Wrap(err, "error while getting by key object from foreign cache")
	}
	if !exists {
		err = informer.GetStore().Resync()
		if err != nil {
			return nil, errors.Wrap(err, "error while resyncing foreign cache")
		}
		obj, exists, err = informer.GetStore().GetByKey(key)
		if err != nil {
			return nil, errors.Wrap(err, "error while retrieving object from foreign cache")
		}
		if !exists {
			return nil, errors.New("object not found after cache resync")
		}
	}

	return obj, nil
}

func ListObjects(informer cache.SharedIndexInformer) ([]interface{}, error) {
	if informer == nil {
		return nil, errors.New("informer not yet instantiated")
	}

	return informer.GetStore().List(), nil
}

func ResyncListObjects(informer cache.SharedIndexInformer) ([]interface{}, error) {
	if informer == nil {
		return nil, errors.New("informer not yet instantiated")
	}

	// resync for ensuring to be remotely aligned with the foreign cluster state
	err := informer.GetStore().Resync()
	if err != nil {
		return nil, err
	}

	return informer.GetStore().List(), nil
}
