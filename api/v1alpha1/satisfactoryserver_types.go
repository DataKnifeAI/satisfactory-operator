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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending         = "Pending"
	PhaseDeploying       = "Deploying"
	PhaseRunning         = "Running"
	PhaseVersionMismatch = "VersionMismatch"
	PhaseFailed          = "Failed"

	ConditionReady             = "Ready"
	ConditionCredentialsValid  = "CredentialsValid"
	ConditionWorldLoaded       = "WorldLoaded"
	ConditionGatewayConfigured = "GatewayConfigured"

	// DefaultImage is the wiki-recommended community image (MIT, ex-wolveix).
	DefaultImage           = "ghcr.io/robtme/satisfactory-server"
	DefaultGamePort        = int32(7777)
	DefaultReliablePort    = int32(8888)
	DefaultMaxPlayers      = int32(4)
	DefaultPersistenceSize = "40Gi"
	DefaultGraceSeconds    = int64(60)
	DefaultPUID            = int64(1000)
	DefaultPGID            = int64(1000)

	SecretKeyAdminPassword = "admin-password"
)

// GatewayConfig configures exposure for Satisfactory game traffic.
type GatewayConfig struct {
	// Enabled creates Gateway / UDPRoute / TCPRoute resources (Envoy Gateway).
	// Default false — Satisfactory (Unreal) joins often fail behind Envoy's UDP
	// proxy because the game sees the Envoy pod IP. Prefer serviceType=LoadBalancer
	// with address set to a kube-vip VIP instead.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Address is the dedicated external IP (Kube-VIP / MetalLB).
	// Required when enabled is true; also used as LoadBalancerIP when
	// enabled is false and serviceType is LoadBalancer.
	// +optional
	// +kubebuilder:validation:Pattern=`^$|^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	Address string `json:"address,omitempty"`

	// ClassName is the GatewayClass name.
	// +kubebuilder:default="envoy"
	// +optional
	ClassName string `json:"className,omitempty"`

	// GatewayName overrides the Gateway resource name.
	// Default: {name}-gateway (strips a trailing "-server" suffix from name).
	// +optional
	GatewayName string `json:"gatewayName,omitempty"`

	// EnvoyProxyName overrides the EnvoyProxy resource name.
	// Default: game-{base}-kubevip.
	// +optional
	EnvoyProxyName string `json:"envoyProxyName,omitempty"`

	// ServiceType overrides the backing Service type when gateway is disabled.
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

// UpdateConfig controls SteamCMD / branch behaviour for the robtme image.
type UpdateConfig struct {
	// SkipUpdate maps to SKIPUPDATE=true so container start does not pull a
	// multi-GB Steam update. Default true for cluster safety (opt-in updates).
	// +kubebuilder:default=true
	// +optional
	SkipUpdate *bool `json:"skipUpdate,omitempty"`

	// SteamBeta when true sets STEAMBETA=true (experimental branch).
	// +optional
	SteamBeta bool `json:"steamBeta,omitempty"`

	// AutoUpdateImage is reserved for future digest/watch tooling. The robtme
	// image updates game files on start unless skipUpdate is true.
	// +optional
	AutoUpdateImage bool `json:"autoUpdateImage,omitempty"`
}

// PersistenceConfig configures the /config PVC (gamefiles, saves, backups).
type PersistenceConfig struct {
	// Size is the PVC capacity. Default 40Gi (gamefiles ~15Gi + saves headroom).
	// +kubebuilder:default="40Gi"
	// +optional
	Size string `json:"size,omitempty"`

	// StorageClassName selects the StorageClass. Empty uses the cluster default.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// MountPath is the container path for the PVC. robtme default: /config
	// +kubebuilder:default="/config"
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// ServerManagerConfig is an optional authenticated admin UI sidecar on the
// game pod. When enabled, TCP port (default 8088) is published on the same
// Service as game traffic (LoadBalancer VIP:port). With hostname + tlsSecretRef,
// the operator also creates a Gateway API HTTPS listener (Envoy) and HTTPRoute.
// Basic auth uses username "admin" and the admin-password Secret key.
type ServerManagerConfig struct {
	// Enabled starts the Server Manager sidecar and exposes its TCP port.
	// Default false. Enabling Recreate-rolls the game Deployment.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Port is the sidecar listen port (and Service TCP port). Default 8088.
	// +kubebuilder:default=8088
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// Hostname is the public DNS name for HTTPS (must match the TLS cert SAN).
	// When set with tlsSecretRef, the operator creates Gateway + HTTPRoute.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// HTTPSPort is the Gateway HTTPS listen port. Default 443.
	// +kubebuilder:default=443
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	HTTPSPort int32 `json:"httpsPort,omitempty"`

	// TLSSecretRef is an existing kubernetes.io/tls Secret. Name is required when
	// hostname is set. Namespace defaults to the SatisfactoryServer namespace.
	// +optional
	TLSSecretRef *corev1.SecretReference `json:"tlsSecretRef,omitempty"`

	// Image is the container image that provides /server-manager (the operator
	// image). Default: harbor.dataknife.net/library/satisfactory-operator:latest
	// +optional
	Image string `json:"image,omitempty"`
}

// BackupConfig configures scheduled tar backups of the data PVC.
type BackupConfig struct {
	// Enabled creates a backup PVC + CronJob. Default false.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Schedule is a Cron expression (UTC). Default "0 4 * * *" (04:00 daily).
	// +kubebuilder:default="0 4 * * *"
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Retain is how many compressed archives to keep. Default 7.
	// +kubebuilder:default=7
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=90
	// +optional
	Retain int32 `json:"retain,omitempty"`

	// Size is the backup PVC capacity. Default matches persistence size.
	// +optional
	Size string `json:"size,omitempty"`

	// StorageClassName for the backup PVC. Empty inherits persistence.storageClassName.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// Image is the backup job container image. Default busybox:1.36.
	// +kubebuilder:default="busybox:1.36"
	// +optional
	Image string `json:"image,omitempty"`
}

// SatisfactoryServerSpec defines the desired state of a Satisfactory dedicated server.
// Configuration maps to the community robtme image environment variables.
type SatisfactoryServerSpec struct {
	// ServerName is a human display name (status / docs). In-game name is set
	// when claiming the server via the Satisfactory HTTPS API / client UI.
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// MaxPlayers maps to MAXPLAYERS (1–16). Default 4.
	// +kubebuilder:default=4
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16
	// +optional
	MaxPlayers int32 `json:"maxPlayers,omitempty"`

	// GamePort is SERVERGAMEPORT (TCP + UDP). Default 7777. Host and container
	// ports must match (Satisfactory does not support remapping the game port).
	// +kubebuilder:default=7777
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	GamePort int32 `json:"gamePort,omitempty"`

	// ReliablePort is SERVERMESSAGINGPORT (TCP). Default 8888 (wiki 1.1+).
	// +kubebuilder:default=8888
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	ReliablePort int32 `json:"reliablePort,omitempty"`

	// PUID is the container process user id (PUID). Default 1000.
	// +kubebuilder:default=1000
	// +optional
	PUID *int64 `json:"puid,omitempty"`

	// PGID is the container process group id (PGID). Default 1000.
	// +kubebuilder:default=1000
	// +optional
	PGID *int64 `json:"pgid,omitempty"`

	// Image is the dedicated server container image repository.
	// +kubebuilder:default="ghcr.io/robtme/satisfactory-server"
	// +optional
	Image string `json:"image,omitempty"`

	// ImageTag pins an image tag. Empty uses the image as given (often :latest).
	// Prefer digests in production.
	// +optional
	ImageTag string `json:"imageTag,omitempty"`

	// ImagePullPolicy for the game server container.
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets for private registries.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// GenerateSecrets when true creates an Opaque Secret with a random
	// admin-password key if missing (for optional Server Manager / future API
	// claim). Existing non-empty keys are never overwritten.
	// +optional
	GenerateSecrets bool `json:"generateSecrets,omitempty"`

	// CredentialsSecretRef names an existing Secret with key admin-password.
	// Required when serverManager.enabled and generateSecrets is false.
	// +optional
	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`

	// Update configures SteamCMD / branch behaviour.
	// +optional
	Update UpdateConfig `json:"update,omitempty"`

	// Gateway configures Envoy Gateway exposure (or LoadBalancer / NodePort).
	// +optional
	Gateway GatewayConfig `json:"gateway,omitempty"`

	// Persistence configures the PVC mounted at /config.
	// +optional
	Persistence PersistenceConfig `json:"persistence,omitempty"`

	// Backup configures an optional CronJob that tars the data PVC into a
	// separate backup PVC (retain N archives).
	// +optional
	Backup BackupConfig `json:"backup,omitempty"`

	// ServerManager is an optional authenticated admin UI sidecar.
	// +optional
	ServerManager ServerManagerConfig `json:"serverManager,omitempty"`

	// Resources overrides auto-selected CPU/memory.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// TerminationGracePeriodSeconds is the SIGTERM grace window (default 60)
	// so the server can flush the world save on stop.
	// +kubebuilder:default=60
	// +kubebuilder:validation:Minimum=30
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// NodeSelector pins the game server pod to specific nodes.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// SatisfactoryServerStatus defines the observed state of SatisfactoryServer.
type SatisfactoryServerStatus struct {
	// Phase is Pending, Deploying, Running, VersionMismatch, or Failed.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Ready is true when the game server Deployment is available.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// ConnectionAddress is the IP clients should use.
	// +optional
	ConnectionAddress string `json:"connectionAddress,omitempty"`

	// ConnectionPort is the game port clients should use (TCP+UDP).
	// +optional
	ConnectionPort int32 `json:"connectionPort,omitempty"`

	// ReliablePort is the observed reliable messaging TCP port.
	// +optional
	ReliablePort int32 `json:"reliablePort,omitempty"`

	// Message is a human-readable status detail.
	// +optional
	Message string `json:"message,omitempty"`

	// CredentialsSecretName is the Secret that holds admin-password when managed.
	// Never contains plaintext passwords.
	// +optional
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`

	// CredentialsGenerated is true when generateSecrets manages the Secret.
	// +optional
	CredentialsGenerated bool `json:"credentialsGenerated,omitempty"`

	// ImageVersion is the running container image reference.
	// +optional
	ImageVersion string `json:"imageVersion,omitempty"`

	// ServerName echoes the configured display name.
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// ServerManagerAddress is the IP for the optional admin UI (usually the VIP).
	// +optional
	ServerManagerAddress string `json:"serverManagerAddress,omitempty"`

	// ServerManagerPort is the TCP port for the optional admin UI.
	// +optional
	ServerManagerPort int32 `json:"serverManagerPort,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sfy;satisfactory
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.status.connectionAddress`
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=`.status.connectionPort`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SatisfactoryServer is the Schema for the satisfactoryservers API.
type SatisfactoryServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SatisfactoryServerSpec   `json:"spec,omitempty"`
	Status SatisfactoryServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SatisfactoryServerList contains a list of SatisfactoryServer.
type SatisfactoryServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SatisfactoryServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SatisfactoryServer{}, &SatisfactoryServerList{})
}
