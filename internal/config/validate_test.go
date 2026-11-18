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
	"testing"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestValidateSpec(t *testing.T) {
	enabled := true
	disabled := false
	skipFalse := false

	tests := []struct {
		name    string
		spec    satisfactoryv1alpha1.SatisfactoryServerSpec
		wantErr bool
	}{
		{
			name: "valid generateSecrets",
			spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				GenerateSecrets: true,
				Gateway:         satisfactoryv1alpha1.GatewayConfig{Enabled: &disabled},
			},
		},
		{
			name: "serverManager without credentials",
			spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				ServerManager: satisfactoryv1alpha1.ServerManagerConfig{Enabled: true},
				Gateway:         satisfactoryv1alpha1.GatewayConfig{Enabled: &disabled, Address: "203.0.113.1"},
			},
			wantErr: true,
		},
		{
			name: "both credential sources",
			spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				GenerateSecrets:      true,
				CredentialsSecretRef: &corev1.LocalObjectReference{Name: "creds"},
				Gateway:              satisfactoryv1alpha1.GatewayConfig{Enabled: &disabled},
			},
			wantErr: true,
		},
		{
			name: "gateway enabled without address",
			spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				GenerateSecrets: true,
				Gateway:         satisfactoryv1alpha1.GatewayConfig{Enabled: &enabled},
			},
			wantErr: true,
		},
		{
			name: "maxPlayers too high",
			spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				MaxPlayers:      17,
				GenerateSecrets: true,
				Gateway:         satisfactoryv1alpha1.GatewayConfig{Enabled: &disabled},
			},
			wantErr: true,
		},
		{
			name: "valid serverManager with generateSecrets",
			spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				GenerateSecrets: true,
				ServerManager:   satisfactoryv1alpha1.ServerManagerConfig{Enabled: true},
				Gateway:         satisfactoryv1alpha1.GatewayConfig{Enabled: &disabled, Address: "203.0.113.1"},
				Update:          satisfactoryv1alpha1.UpdateConfig{SkipUpdate: &skipFalse},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReliablePortDefault(t *testing.T) {
	if got := ReliablePort(satisfactoryv1alpha1.SatisfactoryServerSpec{}); got != 8888 {
		t.Fatalf("default reliable = %d, want 8888", got)
	}
	if got := BeaconPort(satisfactoryv1alpha1.SatisfactoryServerSpec{ReliablePort: 9999}); got != 9999 {
		t.Fatalf("explicit reliable = %d, want 9999", got)
	}
}

func TestImageRef(t *testing.T) {
	if got := ImageRef(satisfactoryv1alpha1.SatisfactoryServerSpec{}); got != satisfactoryv1alpha1.DefaultImage {
		t.Fatalf("default image = %q", got)
	}
	if got := ImageRef(satisfactoryv1alpha1.SatisfactoryServerSpec{ImageTag: "v1"}); got != satisfactoryv1alpha1.DefaultImage+":v1" {
		t.Fatalf("tagged image = %q", got)
	}
}

func TestDefaultResources(t *testing.T) {
	res := DefaultResources(satisfactoryv1alpha1.SatisfactoryServerSpec{MaxPlayers: 8})
	mem := res.Requests[corev1.ResourceMemory]
	want := resource.MustParse("16Gi")
	if mem.Cmp(want) != 0 {
		t.Fatalf("8-player memory request = %s, want 16Gi", mem.String())
	}
}