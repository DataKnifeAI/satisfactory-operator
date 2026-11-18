/*
Copyright 2026 DataKnifeAI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provisioner

import (
	"strconv"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	"github.com/DataKnifeAI/satisfactory-operator/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BuildConfigMap stores non-secret settings for observability / config-hash rolls.
func BuildConfigMap(server *satisfactoryv1alpha1.SatisfactoryServer) *corev1.ConfigMap {
	settings := config.SettingsFromSpec(server)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(server),
			Namespace: server.Namespace,
			Labels:    labelsFor(server),
			Annotations: map[string]string{
				AnnotationConfigHash: config.ConfigHash(settings),
			},
		},
		Data: map[string]string{
			"settings.env": config.RenderSettingsEnv(settings),
		},
	}
}

// BuildPVC creates the data volume for /config (gamefiles + saves + backups).
func BuildPVC(server *satisfactoryv1alpha1.SatisfactoryServer) *corev1.PersistentVolumeClaim {
	size := config.PersistenceSize(server.Spec)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName(server),
			Namespace: server.Namespace,
			Labels:    labelsFor(server),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
	if server.Spec.Persistence.StorageClassName != "" {
		sc := server.Spec.Persistence.StorageClassName
		pvc.Spec.StorageClassName = &sc
	}
	return pvc
}

// BuildService exposes game TCP/UDP + reliable messaging TCP.
//
// When Envoy Gateway is disabled, ServiceType/Address pin exposure. Satisfactory
// (Unreal) often breaks behind Envoy's UDP proxy — prefer LoadBalancer+VIP.
func BuildService(server *satisfactoryv1alpha1.SatisfactoryServer) *corev1.Service {
	gamePort := config.GamePort(server.Spec)
	reliablePort := config.ReliablePort(server.Spec)
	svcType := corev1.ServiceTypeClusterIP
	if !config.GatewayEnabled(server.Spec) && server.Spec.Gateway.ServiceType != "" {
		svcType = server.Spec.Gateway.ServiceType
	}

	ports := []corev1.ServicePort{
		{
			Name:       "game-udp",
			Protocol:   corev1.ProtocolUDP,
			Port:       gamePort,
			TargetPort: intstr.FromInt32(gamePort),
		},
		{
			Name:       "game-tcp",
			Protocol:   corev1.ProtocolTCP,
			Port:       gamePort,
			TargetPort: intstr.FromInt32(gamePort),
		},
		{
			Name:       "reliable",
			Protocol:   corev1.ProtocolTCP,
			Port:       reliablePort,
			TargetPort: intstr.FromInt32(reliablePort),
		},
	}
	ports = AppendServerManagerServicePort(server, ports)

	spec := corev1.ServiceSpec{
		Type:     svcType,
		Selector: labelsFor(server),
		Ports:    ports,
	}
	if !config.GatewayEnabled(server.Spec) && svcType == corev1.ServiceTypeLoadBalancer {
		if addr := server.Spec.Gateway.Address; addr != "" {
			spec.LoadBalancerIP = addr
		}
		spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyCluster
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName(server),
			Namespace: server.Namespace,
			Labels:    labelsFor(server),
		},
		Spec: spec,
	}
}

// BuildDeployment creates the Recreate Deployment for the robtme image.
// secretName is optional; used for the Server Manager sidecar when enabled.
func BuildDeployment(server *satisfactoryv1alpha1.SatisfactoryServer, configHash, secretName string) *appsv1.Deployment {
	gamePort := config.GamePort(server.Spec)
	reliablePort := config.ReliablePort(server.Spec)
	replicas := int32(1)
	grace := config.GraceSeconds(server.Spec)
	labels := labelsFor(server)
	image := config.ImageRef(server.Spec)
	pullPolicy := server.Spec.ImagePullPolicy
	if pullPolicy == "" {
		pullPolicy = corev1.PullIfNotPresent
	}

	env := []corev1.EnvVar{
		{Name: "MAXPLAYERS", Value: strconv.FormatInt(int64(config.MaxPlayers(server.Spec)), 10)},
		{Name: "SERVERGAMEPORT", Value: strconv.FormatInt(int64(gamePort), 10)},
		{Name: "SERVERMESSAGINGPORT", Value: strconv.FormatInt(int64(reliablePort), 10)},
		{Name: "SKIPUPDATE", Value: strconv.FormatBool(config.SkipUpdate(server.Spec))},
		{Name: "STEAMBETA", Value: strconv.FormatBool(server.Spec.Update.SteamBeta)},
		{Name: "PUID", Value: strconv.FormatInt(config.PUID(server.Spec), 10)},
		{Name: "PGID", Value: strconv.FormatInt(config.PGID(server.Spec), 10)},
	}

	mountPath := config.MountPath(server.Spec)
	// Run as PUID/PGID (default 1000) so robtme skips root chown+gosu. TrueNAS
	// NFS root_squash maps root-created files to nobody and makes chown fail
	// under set -e. Prefer storageClass truenas-csi-nfs-no-mapall; avoid fsGroup.
	runAs := config.PUID(server.Spec)
	containers := []corev1.Container{
		{
			Name:            "satisfactory",
			Image:           image,
			ImagePullPolicy: pullPolicy,
			Env:             env,
			Ports: []corev1.ContainerPort{
				{Name: "game-udp", ContainerPort: gamePort, Protocol: corev1.ProtocolUDP},
				{Name: "game-tcp", ContainerPort: gamePort, Protocol: corev1.ProtocolTCP},
				{Name: "reliable", ContainerPort: reliablePort, Protocol: corev1.ProtocolTCP},
			},
			Resources: config.DefaultResources(server.Spec),
			SecurityContext: &corev1.SecurityContext{
				RunAsUser:  &runAs,
				RunAsGroup: &runAs,
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: mountPath},
			},
		},
	}
	volumes := []corev1.Volume{
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName(server),
				},
			},
		},
	}
	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: &grace,
		ImagePullSecrets:              server.Spec.ImagePullSecrets,
		NodeSelector:                  server.Spec.NodeSelector,
		Containers:                    containers,
		Volumes:                       volumes,
	}
	if server.Spec.ServerManager.Enabled {
		podSpec.ServiceAccountName = serverManagerSAName(server)
		podSpec.Containers = append(podSpec.Containers, serverManagerSidecar(server, secretName))
		if server.Spec.Backup.Enabled {
			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
				Name: "backups",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: backupPVCName(server),
					},
				},
			})
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(server),
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						AnnotationConfigHash: configHash,
					},
				},
				Spec: podSpec,
			},
		},
	}
}

// SecretNameFor returns the credentials Secret name for a server.
func SecretNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return secretName(server)
}

// ConfigMapNameFor returns the ConfigMap name.
func ConfigMapNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return configMapName(server)
}

// PVCNameFor returns the PVC name.
func PVCNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return pvcName(server)
}

// DeploymentNameFor returns the Deployment name.
func DeploymentNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return deploymentName(server)
}

// ServiceNameFor returns the Service name.
func ServiceNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return serviceName(server)
}
