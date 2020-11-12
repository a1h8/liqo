package tunnel

import netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"

type Driver interface {

	Init() error

	ConnectToEndpoint(tep *netv1alpha1.TunnelEndpoint) (*netv1alpha1.Connection, error)

	DisconnectFromEndpoint(tep *netv1alpha1.TunnelEndpoint) error

}