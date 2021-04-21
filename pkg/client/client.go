package client

import (
	route53v1 "github.com/takutakahashi/external-route53/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New() (client.Client, error) {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = route53v1.AddToScheme(s)

	kubeconfig := ctrl.GetConfigOrDie()
	kubeclient, err := client.New(kubeconfig, client.Options{Scheme: s})
	if err != nil {
		return nil, err
	}
	return kubeclient, nil
}
