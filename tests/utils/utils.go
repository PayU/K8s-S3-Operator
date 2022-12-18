package utils

import (
	s3operatorv1 "github.com/PayU/K8s-S3-Operator/api/v1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateK8SClient(logger logr.Logger) client.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(s3operatorv1.AddToScheme(scheme))


	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Port:             9443,
		LeaderElectionID: "e8727534.payu.com",
	})
	if err != nil {
		logger.Error(err, "error create mew manager")
		return nil
	} else {
		logger.Info("succseded create k8sclient")
		c,err := client.New(mgr.GetConfig(),client.Options{Scheme: scheme})
		if err != nil{
			logger.Error(err,"error to create new k8s client")
			panic("error to create new k8s client")
		}
		return c
	}
}
