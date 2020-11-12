package discovery

import "github.com/grandcat/zeroconf"

type DiscoveryData interface {
	Get(discovery *DiscoveryCtrl, entry *zeroconf.ServiceEntry) error
}
