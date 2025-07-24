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

// Run executes a graceful rolling restart of all Kubernetes workloads (Deployments, StatefulSets, and DaemonSets)
// that contain the podFilter string in their name across all namespaces in the cluster.
//
// The restart is performed by updating the pod template annotation with a timestamp, which triggers
// Kubernetes to perform a rolling update of the pods - similar to 'kubectl rollout restart'.
//
// The function will:
//   - List and iterate through all namespaces in the cluster
//   - For each namespace, identify Deployments, StatefulSets, and DaemonSets matching the podFilter
//   - Apply a restart annotation to trigger a graceful rollout
//   - Track success/failure metrics for each resource type
//   - Continue processing even if individual resources fail to restart
//
// Errors during restart of individual resources are logged but don't stop the overall process.
// Only critical errors (like inability to list namespaces) will cause the function to return early.
//
// On completion, a summary is logged showing:
//   - Total number of resources restarted by type
//   - Number of namespaces processed
//   - Any errors encountered
//   - Total execution time
//
// Example usage:
//
//	rc := rollout.NewRolloutClient(clientset, "database", logger)
//	err := rc.Run(context.Background())
func (rc *rolloutClient) Run(ctx context.Context) error {
	rc.metadata = &rolloutMetadata{
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
		"total_restarted":    rc.metadata.totalRestarted(),
		"deployments":        rc.metadata.DeploymentsRestarted,
		"statefulsets":       rc.metadata.StatefulSetsRestarted,
		"daemonsets":         rc.metadata.DaemonSetsRestarted,
		"namespaces_checked": rc.metadata.NamespacesProcessed,
		"errors_count":       len(rc.metadata.Errors),
		"duration":           rc.metadata.duration().String(),
	}).Info("Rollout completed")
	return nil
}

// NewRolloutClient creates a new rolloutClient instance for performing rolling restarts of Kubernetes workloads.
func NewRolloutClient(clientset *kubernetes.Clientset, podFilter string, logger logrus.FieldLogger) *rolloutClient {
	return &rolloutClient{
		podFilter: podFilter,
		cs:        clientset,
		log:       logger,
	}
}

type rolloutClient struct {
	podFilter string

	cs       *kubernetes.Clientset
	log      logrus.FieldLogger
	metadata *rolloutMetadata
}

type rolloutMetadata struct {
	StartTime             time.Time
	DeploymentsRestarted  int
	StatefulSetsRestarted int
	DaemonSetsRestarted   int
	NamespacesProcessed   int
	Errors                []error
}

func (rm *rolloutMetadata) totalRestarted() int {
	return rm.DeploymentsRestarted + rm.StatefulSetsRestarted + rm.DaemonSetsRestarted
}

func (rm *rolloutMetadata) duration() time.Duration {
	return time.Since(rm.StartTime)
}

func (rc *rolloutClient) restartDeployments(ctx context.Context, namespace string) (int, error) {
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

func (rc *rolloutClient) restartStatefulSets(ctx context.Context, namespace string) (int, error) {
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

func (rc *rolloutClient) restartDaemonSets(ctx context.Context, namespace string) (int, error) {
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
