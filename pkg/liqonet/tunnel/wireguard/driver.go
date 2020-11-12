package wireguard

import (
	"fmt"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	// PublicKey is the key of publicKey entry in back-end map
	PublicKey = "publicKey"
	// EndpointIP is the key of the endpointIP entry in back-end map
	EndpointIP = "endpointIP"
	// ListeningPort is the key of the listeningPort entry in the back-end map
	ListeningPort = "port"
	//AllowedIPs is the key of the allowedIPs entry in the back-end map
	AllowedIPs      = "allowedIPs"
	defaultPort int = 5871
	deviceName      = "liqo-wg"
	//
	KeepAliveInterval = 10 * time.Second
)

type wgConfig struct {
	//listening port
	port int
	//private key
	priKey wgtypes.Key
	//public key
	pubKey wgtypes.Key
}

type wireguard struct {
	connections map[string]*netv1alpha1.Connection
	client      *wgctrl.Client
	link        netlink.Link
	conf        wgConfig
}

// NewDriver creates a new WireGuard driver
func NewDriver() (tunnel.Driver, error) {
	var err error
	// generate local keys
	var priv, pub wgtypes.Key
	if priv, err = wgtypes.GeneratePrivateKey(); err != nil {
		return nil, fmt.Errorf("error generating private key: %v", err)
	}
	pub = priv.PublicKey()
	w := wireguard{
		connections: make(map[string]*netv1alpha1.Connection),
		conf: wgConfig{
			port:   defaultPort,
			priKey: priv,
			pubKey: pub,
		},
	}

	if err = w.setWGLink(); err != nil {
		return nil, fmt.Errorf("failed to setup WireGuard link: %v", err)
	}

	// create controller
	if w.client, err = wgctrl.New(); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("wgctrl is not available on this system")
		}

		return nil, fmt.Errorf("failed to open wgctl client: %v", err)
	}

	defer func() {
		if err != nil {
			if e := w.client.Close(); e != nil {
				klog.Errorf("Failed to close client %v", e)
			}

			w.client = nil
		}
	}()

	port := defaultPort
	// configure the device. still not up
	peerConfigs := make([]wgtypes.PeerConfig, 0)
	cfg := wgtypes.Config{
		PrivateKey:   &priv,
		ListenPort:   &port,
		FirewallMark: nil,
		ReplacePeers: true,
		Peers:        peerConfigs,
	}
	if err = w.client.ConfigureDevice(deviceName, cfg); err != nil {
		return nil, fmt.Errorf("failed to configure WireGuard device: %v", err)
	}

	klog.Infof("Created WireGuard %s with publicKey %s", deviceName, pub)

	return &w, nil
}

//used to cleanup resources at exit time of the operator
//we do not check the errors at exit time
func closeDriver() {
	//it removes the wireguard interface
	if link, err := netlink.LinkByName(deviceName); err == nil {
		// delete existing device
		if err := netlink.LinkDel(link); err != nil {
			klog.Errorf("failed to delete existing WireGuard device: %v", err)
		}
	}
}

func (w *wireguard) Init() error {
	// ip link set $DefaultDeviceName up
	if err := netlink.LinkSetUp(w.link); err != nil {
		return fmt.Errorf("failed to bring up WireGuard device: %v", err)
	}

	klog.Infof("WireGuard device %s, is up on i/f number %d, listening on port :%d, with key %s",
		w.link.Attrs().Name, w.link.Attrs().Index, w.conf.port, w.conf.pubKey)

	return nil
}

func (w *wireguard) ConnectToEndpoint(tep *netv1alpha1.TunnelEndpoint) (*netv1alpha1.Connection, error) {
	// parse allowed IPs
	allowedIPs, err := getAllowedIPs(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// parse remote public key
	remoteKey, err := getKey(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// parse remote endpoint
	endpoint, err := getEndpoint(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// delete or update old peers for ClusterID
	oldCon, found := w.connections[tep.Spec.ClusterID]
	if found {
		//check if the peer configuration is updated
		if allowedIPs.String() == oldCon.PeerConfiguration[AllowedIPs] && remoteKey.String() == oldCon.PeerConfiguration[PublicKey] &&
			endpoint.IP.String() == oldCon.PeerConfiguration[EndpointIP] && strconv.Itoa(endpoint.Port) == oldCon.PeerConfiguration[ListeningPort] {
			return nil, nil
		}
		klog.Infof("updating peer configuration for cluster %s", tep.Spec.ClusterID)
	} else {
		klog.Infof("Connecting cluster %s endpoint %s with publicKey %s",
			tep.Spec.ClusterID, endpoint.IP.String(), remoteKey)
	}

	ka := KeepAliveInterval
	// configure peer
	peerCfg := []wgtypes.PeerConfig{{
		PublicKey:                   *remoteKey,
		Remove:                      false,
		UpdateOnly:                  false,
		Endpoint:                    endpoint,
		PersistentKeepaliveInterval: &ka,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  []net.IPNet{*allowedIPs},
	}}

	err = w.client.ConfigureDevice(deviceName, wgtypes.Config{
		ReplacePeers: false,
		Peers:        peerCfg,
	})
	if err != nil {
		return newConnectionOnError(err.Error()), fmt.Errorf("failed to configure peer with clusterID %s: %v", tep.Spec.ClusterID, err)
	}
	//
	c := &netv1alpha1.Connection{
		Status:        netv1alpha1.Connected,
		StatusMessage: "Cluster peer connected",
		PeerConfiguration: map[string]string{ListeningPort: strconv.Itoa(endpoint.Port), EndpointIP: endpoint.IP.String(),
			AllowedIPs: allowedIPs.String(), PublicKey: remoteKey.String()},
	}
	klog.Infof("Done connecting cluster peer %s@%s", tep.Spec.ClusterID, endpoint.String())
	return c, nil
}

func (w *wireguard) DisconnectFromEndpoint(tep *netv1alpha1.TunnelEndpoint) error {
	klog.Infof("Removing connection with cluster %s", tep.Spec.ClusterID)

	s, found := tep.Status.Connection.PeerConfiguration[PublicKey]
	if !found {
		klog.Infof("no tunnel configured for cluster %s, nothing to be removed", tep.Spec.ClusterID)
		return nil
	}

	key, err := wgtypes.ParseKey(s)
	if err != nil {
		return fmt.Errorf("failed to parse public key %s: %v", s, err)
	}

	peerCfg := []wgtypes.PeerConfig{
		{
			PublicKey: key,
			Remove:    true,
		},
	}
	err = w.client.ConfigureDevice(deviceName, wgtypes.Config{
		ReplacePeers: false,
		Peers:        peerCfg,
	})
	if err != nil {
		return fmt.Errorf("Failed to remove WireGuard peer with clusterID %s: %v", tep.Spec.ClusterID, err)
	}

	klog.Infof("Done removing WireGuard peer with clusterID %s", tep.Spec.ClusterID)

	return nil
}

// Create new wg link
func (w *wireguard) setWGLink() error {
	// delete existing wg device if needed
	if link, err := netlink.LinkByName(deviceName); err == nil {
		// delete existing device
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete existing WireGuard device: %v", err)
		}
	}
	// create the wg device (ip link add dev $DefaultDeviceName type wireguard)
	la := netlink.NewLinkAttrs()
	la.Name = deviceName
	link := &netlink.GenericLink{
		LinkAttrs: la,
		LinkType:  "wireguard",
	}
	if err := netlink.LinkAdd(link); err == nil {
		w.link = link
	} else {
		return fmt.Errorf("failed to add WireGuard device: %v", err)
	}
	return nil
}

func getAllowedIPs(tep *netv1alpha1.TunnelEndpoint) (*net.IPNet, error) {
	var remoteSubnet string
	//check if the remote podCIDR has been remapped
	if tep.Status.IncomingNAT {
		remoteSubnet = tep.Status.RemoteRemappedPodCIDR
	} else {
		remoteSubnet = tep.Spec.PodCIDR
	}

	_, cidr, err := net.ParseCIDR(remoteSubnet)
	if err != nil {
		return nil, fmt.Errorf("unable to parse podCIDR %s for cluster %s: %v", remoteSubnet, tep.Spec.ClusterID, err)
	}
	return cidr, nil
}

func getKey(tep *netv1alpha1.TunnelEndpoint) (*wgtypes.Key, error) {
	s, found := tep.Spec.BackendConfig[PublicKey]
	if !found {
		return nil, fmt.Errorf("endpoint is missing public key")
	}

	key, err := wgtypes.ParseKey(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key %s: %v", s, err)
	}

	return &key, nil
}

func getEndpoint(tep *netv1alpha1.TunnelEndpoint) (*net.UDPAddr, error) {
	//get port
	port, found := tep.Spec.BackendConfig[ListeningPort]
	if !found {
		return nil, fmt.Errorf("tunnelEndpoint is missing listening port")
	}
	//convert port from string to int
	listeningPort, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error while converting port %s to int: %v", port, err)
	}
	//get endpoint ip
	remoteIP := net.ParseIP(tep.Spec.EndpointIP)
	if remoteIP == nil {
		return nil, fmt.Errorf("failed to parse remote IP %s", tep.Spec.EndpointIP)
	}
	return &net.UDPAddr{
		IP:   remoteIP,
		Port: int(listeningPort),
	}, nil
}

func newConnectionOnError(msg string) *netv1alpha1.Connection {
	return &netv1alpha1.Connection{
		Status:            netv1alpha1.ConnectionError,
		StatusMessage:     msg,
		PeerConfiguration: nil,
	}
}
