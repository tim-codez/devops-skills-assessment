package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/tim-codez/devops-skills-assessment/cmd/rollout"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Switch this to "nginx" if you have already ran "make deploy", that way you can see real resources get restarted
// otherwise there will be no pods to restart with the name "database", not as cool of a demonstration.
const podFilter = "database"

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	componentLogger := logger.WithField("component", "rollout")

	config, err := buildConfig()
	if err != nil {
		componentLogger.WithError(err).Fatal("Failed to build kubernetes config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		componentLogger.WithError(err).Fatal("failed to create clientset")
	}

	rc := rollout.NewRolloutClient(clientset, podFilter, componentLogger)
	err = rc.Run(context.Background())
	if err != nil {
		componentLogger.WithError(err).Fatal("Rollout failed")
	}
}

func buildConfig() (*rest.Config, error) {
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Check if KUBECONFIG env var is set
	if envKubeConfig := os.Getenv("KUBECONFIG"); envKubeConfig != "" {
		kubeconfig = envKubeConfig
	}

	// Use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return config, nil
}
