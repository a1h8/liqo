package discovery

import "sync"

type discoveryData struct {
	txtData  *TxtData
	authData *AuthData
}

type discoveryCache struct {
	m    map[string]discoveryData
	lock sync.RWMutex
}

var resolvedData = discoveryCache{}

func (discoveryCache *discoveryCache) add(key string, data DiscoveryData) {
	discoveryCache.lock.Lock()
	defer discoveryCache.lock.Unlock()
	if _, ok := discoveryCache.m[key]; !ok {
		switch data.(type) {
		case *TxtData:
			discoveryCache.m[key] = discoveryData{
				txtData: data.(*TxtData),
			}
		case *AuthData:
			discoveryCache.m[key] = discoveryData{
				authData: data.(*AuthData),
			}
		}
	} else {
		oldData := discoveryCache.m[key]
		switch data.(type) {
		case *TxtData:
			oldData.txtData = data.(*TxtData)
		case *AuthData:
			oldData.authData = data.(*AuthData)
		}

		discoveryCache.m[key] = oldData
	}
}
