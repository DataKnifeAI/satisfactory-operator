/*
Copyright 2026 DataKnifeAI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provisioner

import (
	"fmt"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	"github.com/DataKnifeAI/satisfactory-operator/internal/config"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ComponentServerManager   = "server-manager"
	containerServerManager   = "server-manager"
	portNameServerManager    = "manager"
	serverManagerDataPath    = "/data"
	serverManagerBackupsPath = "/backups"
	serverManagerBinary      = "/server-manager"
)

func serverManagerSAName(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return server.Name + "-server-manager"
}

func serverManagerRoleName(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return server.Name + "-server-manager"
}

// ServerManagerSANameFor returns the sidecar ServiceAccount name.
func ServerManagerSANameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return serverManagerSAName(server)
}

// ServerManagerRoleNameFor returns the sidecar Role / RoleBinding name.
func ServerManagerRoleNameFor(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	return serverManagerRoleName(server)
}

func serverManagerLabels(server *satisfactoryv1alpha1.SatisfactoryServer) map[string]string {
	return map[string]string{
		LabelAppName:      "satisfactory-server",
		LabelAppInstance:  server.Name,
		LabelAppManagedBy: ManagedBy,
		LabelAppComponent: ComponentServerManager,
	}
}

// BuildServerManagerServiceAccount for the admin UI sidecar.
func BuildServerManagerServiceAccount(server *satisfactoryv1alpha1.SatisfactoryServer) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverManagerSAName(server),
			Namespace: server.Namespace,
			Labels:    serverManagerLabels(server),
		},
	}
}

// BuildServerManagerRole grants patch on this CR, Deployment, Secret, CronJob/Job.
func BuildServerManagerRole(server *satisfactoryv1alpha1.SatisfactoryServer, secretName string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverManagerRoleName(server),
			Namespace: server.Namespace,
			Labels:    serverManagerLabels(server),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"apps"},
				Resources:     []string{"deployments"},
				ResourceNames: []string{deploymentName(server)},
				Verbs:         []string{"get", "patch", "update"},
			},
			{
				APIGroups:     []string{satisfactoryv1alpha1.GroupVersion.Group},
				Resources:     []string{"satisfactoryservers"},
				ResourceNames: []string{server.Name},
				Verbs:         []string{"get", "patch", "update"},
			},
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
				Verbs:         []string{"get", "patch", "update"},
			},
			{
				APIGroups:     []string{"batch"},
				Resources:     []string{"cronjobs"},
				ResourceNames: []string{backupCronJobName(server)},
				Verbs:         []string{"get"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"create"},
			},
		},
	}
}

// BuildServerManagerRoleBinding binds the sidecar SA to its Role.
func BuildServerManagerRoleBinding(server *satisfactoryv1alpha1.SatisfactoryServer) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverManagerRoleName(server),
			Namespace: server.Namespace,
			Labels:    serverManagerLabels(server),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serverManagerSAName(server),
			Namespace: server.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     serverManagerRoleName(server),
		},
	}
}

func serverManagerSidecar(server *satisfactoryv1alpha1.SatisfactoryServer, secretName string) corev1.Container {
	port := config.ServerManagerPort(server.Spec)
	mounts := []corev1.VolumeMount{
		{Name: "data", MountPath: serverManagerDataPath},
	}
	env := []corev1.EnvVar{
		{Name: "SERVER_MANAGER_LISTEN", Value: fmt.Sprintf(":%d", port)},
		{Name: "SERVER_MANAGER_USER", Value: config.DefaultServerManagerUser},
		{Name: "SERVER_MANAGER_DATA", Value: serverManagerDataPath},
		{Name: "SERVER_MANAGER_CR", Value: server.Name},
		{Name: "SERVER_MANAGER_DEPLOYMENT", Value: deploymentName(server)},
		{Name: "SERVER_MANAGER_SECRET", Value: secretName},
		{
			Name: "SERVER_MANAGER_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		},
		{
			Name: "SERVER_MANAGER_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  satisfactoryv1alpha1.SecretKeyAdminPassword,
				},
			},
		},
	}
	if server.Spec.Backup.Enabled {
		env = append(env, corev1.EnvVar{Name: "SERVER_MANAGER_BACKUPS", Value: serverManagerBackupsPath})
		mounts = append(mounts, corev1.VolumeMount{Name: "backups", MountPath: serverManagerBackupsPath})
	}
	return corev1.Container{
		Name:            containerServerManager,
		Image:           config.ServerManagerImage(server.Spec),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{serverManagerBinary},
		Ports: []corev1.ContainerPort{
			{Name: portNameServerManager, ContainerPort: port, Protocol: corev1.ProtocolTCP},
		},
		Env:          env,
		VolumeMounts: mounts,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             ptr(true),
			AllowPrivilegeEscalation: ptr(false),
		},
	}
}

func ptr[T any](v T) *T { return &v }

// AppendServerManagerServicePort adds the TCP manager port when enabled.
func AppendServerManagerServicePort(server *satisfactoryv1alpha1.SatisfactoryServer, ports []corev1.ServicePort) []corev1.ServicePort {
	if !server.Spec.ServerManager.Enabled {
		return ports
	}
	port := config.ServerManagerPort(server.Spec)
	return append(ports, corev1.ServicePort{
		Name:       portNameServerManager,
		Protocol:   corev1.ProtocolTCP,
		Port:       port,
		TargetPort: intstr.FromInt32(port),
	})
}
