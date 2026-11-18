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

package config

import (
	"fmt"
	"strings"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
)

// ValidateSpec returns a user-facing error if the CR cannot be reconciled.
func ValidateSpec(spec satisfactoryv1alpha1.SatisfactoryServerSpec) error {
	players := MaxPlayers(spec)
	if players < 1 || players > 16 {
		return fmt.Errorf("spec.maxPlayers must be between 1 and 16, got %d", players)
	}
	if spec.GenerateSecrets && spec.CredentialsSecretRef != nil && spec.CredentialsSecretRef.Name != "" {
		return fmt.Errorf("specify only one of spec.generateSecrets or spec.credentialsSecretRef")
	}
	if needsCredentials(spec) && !spec.GenerateSecrets && (spec.CredentialsSecretRef == nil || spec.CredentialsSecretRef.Name == "") {
		return fmt.Errorf("provide spec.credentialsSecretRef or set spec.generateSecrets=true when serverManager is enabled")
	}
	if gatewayEnabled(spec) && strings.TrimSpace(spec.Gateway.Address) == "" {
		return fmt.Errorf("spec.gateway.address is required when gateway is enabled")
	}
	if !gatewayEnabled(spec) && spec.Gateway.ServiceType == "LoadBalancer" && strings.TrimSpace(spec.Gateway.Address) == "" {
		return fmt.Errorf("spec.gateway.address is required when serviceType is LoadBalancer")
	}
	if err := validateServerManager(spec); err != nil {
		return err
	}
	return nil
}

func needsCredentials(spec satisfactoryv1alpha1.SatisfactoryServerSpec) bool {
	return spec.ServerManager.Enabled
}

func validateServerManager(spec satisfactoryv1alpha1.SatisfactoryServerSpec) error {
	if !spec.ServerManager.Enabled {
		return nil
	}
	host := strings.TrimSpace(spec.ServerManager.Hostname)
	ref := spec.ServerManager.TLSSecretRef
	if host == "" && ref == nil {
		return nil
	}
	if host == "" {
		return fmt.Errorf("spec.serverManager.hostname is required when tlsSecretRef is set")
	}
	if ref == nil || strings.TrimSpace(ref.Name) == "" {
		return fmt.Errorf("spec.serverManager.tlsSecretRef.name is required when hostname is set")
	}
	if strings.TrimSpace(spec.Gateway.Address) == "" {
		return fmt.Errorf("spec.gateway.address is required for server manager HTTPS (VIP for Gateway externalIPs)")
	}
	return nil
}

func gatewayEnabled(spec satisfactoryv1alpha1.SatisfactoryServerSpec) bool {
	if spec.Gateway.Enabled == nil {
		return false
	}
	return *spec.Gateway.Enabled
}

// MaxPlayers returns the effective player cap.
func MaxPlayers(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	if spec.MaxPlayers == 0 {
		return satisfactoryv1alpha1.DefaultMaxPlayers
	}
	return spec.MaxPlayers
}

// Players is an alias kept for older call sites.
func Players(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	return MaxPlayers(spec)
}

// GamePort returns the effective game port (TCP+UDP).
func GamePort(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	if spec.GamePort == 0 {
		return satisfactoryv1alpha1.DefaultGamePort
	}
	return spec.GamePort
}

// ReliablePort returns the effective reliable messaging TCP port.
func ReliablePort(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	if spec.ReliablePort != 0 {
		return spec.ReliablePort
	}
	return satisfactoryv1alpha1.DefaultReliablePort
}

// BeaconPort is a compatibility alias for ReliablePort (UDP beacon naming from
// other Unreal operators). Satisfactory uses TCP for this port.
func BeaconPort(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	return ReliablePort(spec)
}

// ServerName returns the effective server display name.
func ServerName(spec satisfactoryv1alpha1.SatisfactoryServerSpec, fallback string) string {
	if strings.TrimSpace(spec.ServerName) != "" {
		return spec.ServerName
	}
	if fallback != "" {
		return fallback
	}
	return "satisfactory"
}

// ImageRef builds the container image reference from image + optional tag.
func ImageRef(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	image := spec.Image
	if image == "" {
		image = satisfactoryv1alpha1.DefaultImage
	}
	if spec.ImageTag == "" {
		return image
	}
	if strings.Contains(image, "@") || strings.Contains(image, ":") {
		return image
	}
	return image + ":" + spec.ImageTag
}

// MountPath returns the PVC mount path.
func MountPath(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	if spec.Persistence.MountPath != "" {
		return spec.Persistence.MountPath
	}
	return "/config"
}

// PersistenceSize returns the PVC size string.
func PersistenceSize(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	if spec.Persistence.Size != "" {
		return spec.Persistence.Size
	}
	return satisfactoryv1alpha1.DefaultPersistenceSize
}

// GraceSeconds returns terminationGracePeriodSeconds.
func GraceSeconds(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int64 {
	if spec.TerminationGracePeriodSeconds != nil {
		return *spec.TerminationGracePeriodSeconds
	}
	return satisfactoryv1alpha1.DefaultGraceSeconds
}

// GatewayEnabled reports whether Envoy Gateway resources should be reconciled.
func GatewayEnabled(spec satisfactoryv1alpha1.SatisfactoryServerSpec) bool {
	return gatewayEnabled(spec)
}

// SkipUpdate returns whether SKIPUPDATE should be true (default true).
func SkipUpdate(spec satisfactoryv1alpha1.SatisfactoryServerSpec) bool {
	if spec.Update.SkipUpdate == nil {
		return true
	}
	return *spec.Update.SkipUpdate
}

// PUID returns the process user id.
func PUID(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int64 {
	if spec.PUID != nil {
		return *spec.PUID
	}
	return satisfactoryv1alpha1.DefaultPUID
}

// PGID returns the process group id.
func PGID(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int64 {
	if spec.PGID != nil {
		return *spec.PGID
	}
	return satisfactoryv1alpha1.DefaultPGID
}

const (
	DefaultBackupSchedule         = "0 4 * * *"
	DefaultBackupRetain           = int32(7)
	DefaultBackupImage            = "busybox:1.36"
	DefaultServerManagerPort      = int32(8088)
	DefaultServerManagerHTTPSPort = int32(443)
	DefaultServerManagerImage     = "harbor.dataknife.net/library/satisfactory-operator:latest"
	DefaultServerManagerUser      = "admin"
)

// ServerManagerPort returns the TCP port for the admin UI sidecar.
func ServerManagerPort(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	if spec.ServerManager.Port > 0 {
		return spec.ServerManager.Port
	}
	return DefaultServerManagerPort
}

// ServerManagerHTTPSPort returns the Gateway HTTPS listen port.
func ServerManagerHTTPSPort(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	if spec.ServerManager.HTTPSPort > 0 {
		return spec.ServerManager.HTTPSPort
	}
	return DefaultServerManagerHTTPSPort
}

// ServerManagerHostname returns the trimmed HTTPS hostname (may be empty).
func ServerManagerHostname(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	return strings.TrimSpace(spec.ServerManager.Hostname)
}

// ServerManagerHTTPSEnabled is true when hostname + tlsSecretRef are set.
func ServerManagerHTTPSEnabled(spec satisfactoryv1alpha1.SatisfactoryServerSpec) bool {
	return spec.ServerManager.Enabled &&
		ServerManagerHostname(spec) != "" &&
		spec.ServerManager.TLSSecretRef != nil &&
		strings.TrimSpace(spec.ServerManager.TLSSecretRef.Name) != ""
}

// ServerManagerImage returns the sidecar image (operator image with /server-manager).
func ServerManagerImage(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	if strings.TrimSpace(spec.ServerManager.Image) != "" {
		return spec.ServerManager.Image
	}
	return DefaultServerManagerImage
}

// BackupSchedule returns the Cron schedule for world backups.
func BackupSchedule(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	if strings.TrimSpace(spec.Backup.Schedule) != "" {
		return spec.Backup.Schedule
	}
	return DefaultBackupSchedule
}

// BackupRetain returns how many tar archives to keep.
func BackupRetain(spec satisfactoryv1alpha1.SatisfactoryServerSpec) int32 {
	if spec.Backup.Retain > 0 {
		return spec.Backup.Retain
	}
	return DefaultBackupRetain
}

// BackupSize returns the backup PVC size.
func BackupSize(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	if strings.TrimSpace(spec.Backup.Size) != "" {
		return spec.Backup.Size
	}
	return PersistenceSize(spec)
}

// BackupImage returns the backup job image.
func BackupImage(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	if strings.TrimSpace(spec.Backup.Image) != "" {
		return spec.Backup.Image
	}
	return DefaultBackupImage
}

// BackupStorageClass returns the backup PVC storage class (may be empty).
func BackupStorageClass(spec satisfactoryv1alpha1.SatisfactoryServerSpec) string {
	if strings.TrimSpace(spec.Backup.StorageClassName) != "" {
		return spec.Backup.StorageClassName
	}
	return spec.Persistence.StorageClassName
}
