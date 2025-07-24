package rollout

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RolloutMetadata struct {
	StartTime             time.Time
	DeploymentsRestarted  int
	StatefulSetsRestarted int
	DaemonSetsRestarted   int
	NamespacesProcessed   int
	Errors                []error
}

func (rm *RolloutMetadata) TotalRestarted() int {
	return rm.DeploymentsRestarted + rm.StatefulSetsRestarted + rm.DaemonSetsRestarted
}

func (rm *RolloutMetadata) Duration() time.Duration {
	return time.Since(rm.StartTime)
}

type RolloutClient struct {
	podFilter string

	cs       *kubernetes.Clientset
	log      logrus.FieldLogger
	metadata *RolloutMetadata
}

func NewRolloutClient(clientset *kubernetes.Clientset, podFilter string, logger logrus.FieldLogger) *RolloutClient {
	return &RolloutClient{
		podFilter: podFilter,
		cs:        clientset,
		log:       logger,
	}
}

func (rc *RolloutClient) Run(ctx context.Context) error {
	rc.metadata = &RolloutMetadata{
		StartTime: time.Now(),
		Errors:    []error{},
	}

	namespaces, err := rc.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Process each namespace
	for _, ns := range namespaces.Items {
		rc.metadata.NamespacesProcessed++
		rc.log.WithField("namespace", ns.Name).Info("Checking namespace")

		// Restart deployments with podFilter
		deploymentCount, err := rc.restartDeployments(ctx, ns.Name)
		if err != nil {
			rc.metadata.Errors = append(rc.metadata.Errors, fmt.Errorf("deployments in %s: %w", ns.Name, err))
			rc.log.WithFields(logrus.Fields{
				"namespace": ns.Name,
				"error":     err,
			}).Error("Failed to restart deployments")
		} else {
			rc.metadata.DeploymentsRestarted += deploymentCount
		}

		// Restart statefulsets with podFilter
		statefulSetCount, err := rc.restartStatefulSets(ctx, ns.Name)
		if err != nil {
			rc.metadata.Errors = append(rc.metadata.Errors, fmt.Errorf("statefulsets in %s: %w", ns.Name, err))
			rc.log.WithFields(logrus.Fields{
				"namespace": ns.Name,
				"error":     err,
			}).Error("Failed to restart statefulsets")
		} else {
			rc.metadata.StatefulSetsRestarted += statefulSetCount
		}

		// Restart daemonsets with podFilter
		daemonSetCount, err := rc.restartDaemonSets(ctx, ns.Name)
		if err != nil {
			rc.metadata.Errors = append(rc.metadata.Errors, fmt.Errorf("daemonsets in %s: %w", ns.Name, err))
			rc.log.WithFields(logrus.Fields{
				"namespace": ns.Name,
				"error":     err,
			}).Error("Failed to restart daemonsets")
		} else {
			rc.metadata.DaemonSetsRestarted += daemonSetCount
		}
	}

	// Log summary with metadata
	rc.log.WithFields(logrus.Fields{
		"total_restarted":    rc.metadata.TotalRestarted(),
		"deployments":        rc.metadata.DeploymentsRestarted,
		"statefulsets":       rc.metadata.StatefulSetsRestarted,
		"daemonsets":         rc.metadata.DaemonSetsRestarted,
		"namespaces_checked": rc.metadata.NamespacesProcessed,
		"errors_count":       len(rc.metadata.Errors),
		"duration":           rc.metadata.Duration().String(),
	}).Info("Rollout completed")
	return nil
}

func (rc *RolloutClient) restartDeployments(ctx context.Context, namespace string) (int, error) {
	deployments, err := rc.cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	count := 0
	for _, deployment := range deployments.Items {
		if strings.Contains(strings.ToLower(deployment.Name), rc.podFilter) {
			rc.log.WithFields(logrus.Fields{
				"namespace":  namespace,
				"deployment": deployment.Name,
			}).Info("Restarting deployment")

			// Update the deployment with a new annotation to trigger rollout
			if deployment.Spec.Template.ObjectMeta.Annotations == nil {
				deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
			}
			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

			_, err := rc.cs.AppsV1().Deployments(namespace).Update(ctx, &deployment, metav1.UpdateOptions{})
			if err != nil {
				rc.log.WithFields(logrus.Fields{
					"namespace":  namespace,
					"deployment": deployment.Name,
					"error":      err,
				}).Error("Failed to restart deployment")
				continue
			}

			count++
		}
	}
	return count, nil
}

func (rc *RolloutClient) restartStatefulSets(ctx context.Context, namespace string) (int, error) {
	statefulSets, err := rc.cs.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	count := 0
	for _, sts := range statefulSets.Items {
		if strings.Contains(strings.ToLower(sts.Name), rc.podFilter) {
			rc.log.WithFields(logrus.Fields{
				"namespace":   namespace,
				"statefulset": sts.Name,
			}).Info("Restarting statefulset")

			// Update the statefulset with a new annotation to trigger rollout
			if sts.Spec.Template.ObjectMeta.Annotations == nil {
				sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
			}
			sts.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

			_, err := rc.cs.AppsV1().StatefulSets(namespace).Update(ctx, &sts, metav1.UpdateOptions{})
			if err != nil {
				rc.log.WithFields(logrus.Fields{
					"namespace":   namespace,
					"statefulset": sts.Name,
					"error":       err,
				}).Error("Failed to restart statefulset")
				continue
			}

			count++
		}
	}
	return count, nil
}

func (rc *RolloutClient) restartDaemonSets(ctx context.Context, namespace string) (int, error) {
	daemonSets, err := rc.cs.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	count := 0
	for _, ds := range daemonSets.Items {
		if strings.Contains(strings.ToLower(ds.Name), rc.podFilter) {
			rc.log.WithFields(logrus.Fields{
				"namespace": namespace,
				"daemonset": ds.Name,
			}).Info("Restarting daemonset")

			// Update the daemonset with a new annotation to trigger rollout
			if ds.Spec.Template.ObjectMeta.Annotations == nil {
				ds.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
			}
			ds.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

			_, err := rc.cs.AppsV1().DaemonSets(namespace).Update(ctx, &ds, metav1.UpdateOptions{})
			if err != nil {
				rc.log.WithFields(logrus.Fields{
					"namespace": namespace,
					"daemonset": ds.Name,
					"error":     err,
				}).Error("Failed to restart daemonset")
				continue
			}

			count++
		}
	}
	return count, nil
}
