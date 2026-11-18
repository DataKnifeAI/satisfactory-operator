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

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// DefaultResources returns CPU/memory for Satisfactory.
// Wiki guidance: ~8–16Gi RAM; we request 12Gi / limit 16Gi by default and scale
// mildly with maxPlayers.
func DefaultResources(spec satisfactoryv1alpha1.SatisfactoryServerSpec) corev1.ResourceRequirements {
	if spec.Resources != nil {
		return *spec.Resources
	}
	players := MaxPlayers(spec)
	memGi := 12
	limitGi := 16
	if players >= 8 {
		memGi = 16
		limitGi = 24
	}
	cpuReq := "2"
	cpuLim := "4"
	if players >= 8 {
		cpuReq = "4"
		cpuLim = "8"
	}
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuReq),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", memGi)),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuLim),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", limitGi)),
		},
	}
}
