package wrapper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

// JobRunner triggers the keycloak-config-cli K8s Job when configuration changes.
type JobRunner struct {
	Client         client.Client
	APIReader      client.Reader
	URL            string
	User           string
	PasswordSecret *corev1.SecretKeySelector
	ConfigCLIImage string
}

const (
	labelApp = "keycloak-config-cli"
)

// discoverPullSecrets reads the operator's own Pod spec to discover imagePullSecrets.
// This is the Kubernetes-native way: the Helm chart sets imagePullSecrets on the
// operator Deployment, and the operator propagates them to any Jobs it creates.
func (r *JobRunner) discoverPullSecrets(ctx context.Context, namespace string) []corev1.LocalObjectReference {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		return nil
	}

	var pod corev1.Pod
	if r.APIReader != nil {
		if err := r.APIReader.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, &pod); err != nil {
			log.FromContext(ctx).V(1).Info("Could not read own pod via APIReader for imagePullSecrets discovery", "error", err)
			return nil
		}
	} else {
		if err := r.Client.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, &pod); err != nil {
			log.FromContext(ctx).V(1).Info("Could not read own pod via Client for imagePullSecrets discovery", "error", err)
			return nil
		}
	}

	return pod.Spec.ImagePullSecrets
}

// SyncRealm takes the generated Keycloak JSON representation and ensures it's applied
// by spawning the config-cli Job only if the configuration has drifted/changed.
func (r *JobRunner) SyncRealm(ctx context.Context, realm *v1alpha1.Realm, export *RealmExport, scheme *runtime.Scheme) error {
	logger := log.FromContext(ctx)
	realmName := realm.Spec.RealmName
	namespace := realm.Namespace

	payload, err := json.Marshal(export)
	if err != nil {
		return fmt.Errorf("failed to marshal realm export: %w", err)
	}

	secretName := fmt.Sprintf("kc-config-%s", realm.Name)
	jobName := fmt.Sprintf("kc-config-job-%s", realm.Name)

	var existingSecret corev1.Secret
	err = r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, &existingSecret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("getting config secret %q: %w", secretName, err)
	}
	secretExists := err == nil
	if secretExists {
		// Compare existing payload. If identical, check for periodic drift timeout
		if string(existingSecret.Data["realm.json"]) == string(payload) {
			var lastSyncTime time.Time
			if tStr := existingSecret.Annotations["last-sync"]; tStr != "" {
				lastSyncTime, _ = time.Parse(time.RFC3339, tStr)
			}
			if time.Since(lastSyncTime) < 5*time.Minute {
				logger.Info("Configuration up to date and recently synced, skipping execution", "realm", realmName)
				return nil
			}
			logger.Info("Periodic drift-healing triggered (payload unchanged but >5m elapsed)", "realm", realmName)
		}
	}

	logger.Info("Configuration drift detected, syncing...", "realm", realmName)

	if secretExists {
		if existingSecret.Labels == nil {
			existingSecret.Labels = make(map[string]string)
		}
		if existingSecret.Annotations == nil {
			existingSecret.Annotations = make(map[string]string)
		}
		existingSecret.Annotations["last-sync"] = time.Now().Format(time.RFC3339)
		existingSecret.Labels["app"] = labelApp
		if existingSecret.Data == nil {
			existingSecret.Data = make(map[string][]byte)
		}
		existingSecret.Data["realm.json"] = payload

		if err := controllerutil.SetControllerReference(realm, &existingSecret, scheme); err != nil {
			return fmt.Errorf("failed to set secret owner reference: %w", err)
		}
		if err := r.Client.Update(ctx, &existingSecret); err != nil {
			return fmt.Errorf("failed to update config secret: %w", err)
		}
	} else {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Labels:    map[string]string{"app": labelApp},
				Annotations: map[string]string{
					"last-sync": time.Now().Format(time.RFC3339),
				},
			},
			Data: map[string][]byte{
				"realm.json": payload,
			},
		}

		if err := controllerutil.SetControllerReference(realm, secret, scheme); err != nil {
			return fmt.Errorf("failed to set secret owner reference: %w", err)
		}
		if err := r.Client.Create(ctx, secret); err != nil {
			return fmt.Errorf("failed to create config secret: %w", err)
		}
	}

	// Delete old Job if exists
	var oldJob batchv1.Job
	if err := r.Client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, &oldJob); err == nil {
		propagation := metav1.DeletePropagationBackground
		if err := r.Client.Delete(ctx, &oldJob, &client.DeleteOptions{PropagationPolicy: &propagation}); err != nil {
			logger.Error(err, "Failed to delete old job")
		}
	}

	// Spawn new Job
	backoffLimit := int32(2)

	image := r.ConfigCLIImage
	if image == "" {
		return fmt.Errorf("config-cli image not configured (set CONFIG_CLI_IMAGE)")
	}

	// Discover imagePullSecrets from operator's own Pod spec (using OS namespace where the pod lives)
	operatorPodNamespace := os.Getenv("WATCH_NAMESPACE")
	if operatorPodNamespace == "" {
		operatorPodNamespace = "default"
	}
	pullSecrets := r.discoverPullSecrets(ctx, operatorPodNamespace)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    map[string]string{"app": labelApp},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": labelApp},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					AutomountServiceAccountToken: ptrBool(false),
					ImagePullSecrets:             pullSecrets,
					Containers: []corev1.Container{
						{
							Name:  "config-cli",
							Image: image,
							Env: []corev1.EnvVar{
								{Name: "KEYCLOAK_URL", Value: r.URL},
								{Name: "KEYCLOAK_USER", Value: r.User},
								{
									Name: "KEYCLOAK_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: r.PasswordSecret,
									},
								},
								{Name: "IMPORT_FILES_LOCATIONS", Value: "/config/realm.json"},
								{Name: "IMPORT_REMOTESTATE_ENABLED", Value: "true"},
								{Name: "IMPORT_MANAGED_IDENTITYPROVIDER", Value: "full"},
								{Name: "IMPORT_MANAGED_AUTHENTICATIONFLOW", Value: "full"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/config",
									ReadOnly:  true,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptrBool(false),
								RunAsNonRoot:             ptrBool(true),
								RunAsUser:                ptrInt64(1000),
								ReadOnlyRootFilesystem:   ptrBool(false), // config-cli writes temp files
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(realm, job, scheme); err != nil {
		return fmt.Errorf("failed to set job owner reference: %w", err)
	}

	if err := r.Client.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to spawn config-cli job: %w", err)
	}

	logger.Info("Spawned keycloak-config-cli Job", "job", jobName)
	return nil
}
func ptrBool(b bool) *bool {
	return &b
}

func ptrInt64(i int64) *int64 {
	return &i
}
