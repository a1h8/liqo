package v1

import (
	"github.com/netgroup-polito/dronev2/pkg/crdClient/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

func CreateClient(kubeconfig string) (*v1alpha1.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	config, err = v1alpha1.NewKubeconfig(kubeconfig, &GroupVersion)
	if err != nil {
		panic(err)
	}
	clientSet, err := v1alpha1.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	v1alpha1.AddToRegistry("namespacenattingtables", &NamespaceNattingTable{}, &NamespaceNattingTableList{})

	return clientSet, nil
}
