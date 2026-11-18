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

package gateway

import (
	"testing"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	egv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestDeriveNames(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{
		ObjectMeta: metav1.ObjectMeta{Name: "satisfactory-test"},
	}
	names := DeriveNames(server)
	if names.Gateway != "satisfactory-test-gateway" {
		t.Fatalf("gateway = %q", names.Gateway)
	}
	if names.EnvoyProxy != "game-satisfactory-test-kubevip" {
		t.Fatalf("envoyproxy = %q", names.EnvoyProxy)
	}
	if names.EnvoySvc != "satisfactory-test-envoy" {
		t.Fatalf("envoy svc = %q", names.EnvoySvc)
	}
}

func TestBuildGatewayListeners(t *testing.T) {
	on := true
	server := &satisfactoryv1alpha1.SatisfactoryServer{
		ObjectMeta: metav1.ObjectMeta{Name: "ashenfall", Namespace: "game-servers"},
		Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
			GamePort:     7777,
			ReliablePort: 8888,
			Gateway:      satisfactoryv1alpha1.GatewayConfig{Enabled: &on, Address: "192.168.14.188"},
		},
	}
	gw := BuildGateway(server)
	if len(gw.Spec.Listeners) != 2 {
		t.Fatalf("listeners = %d", len(gw.Spec.Listeners))
	}
	if gw.Spec.Listeners[0].Protocol != gatewayv1.UDPProtocolType {
		t.Fatal("expected UDP listeners")
	}
	if gw.Spec.Addresses[0].Value != "192.168.14.188" {
		t.Fatalf("address = %q", gw.Spec.Addresses[0].Value)
	}
}

func TestBuildEnvoyProxyUsesClusterETP(t *testing.T) {
	on := true
	server := &satisfactoryv1alpha1.SatisfactoryServer{
		ObjectMeta: metav1.ObjectMeta{Name: "satisfactory-test", Namespace: "game-servers"},
		Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
			GamePort:     7777,
			ReliablePort: 8888,
			Gateway:      satisfactoryv1alpha1.GatewayConfig{Enabled: &on, Address: "192.168.14.188"},
		},
	}
	ep := BuildEnvoyProxy(server)
	svc := ep.Spec.Provider.Kubernetes.EnvoyService
	if svc.ExternalTrafficPolicy == nil || *svc.ExternalTrafficPolicy != egv1a1.ServiceExternalTrafficPolicyCluster {
		t.Fatalf("externalTrafficPolicy = %v, want Cluster", svc.ExternalTrafficPolicy)
	}
}

func TestMissingGatewayClassMessage(t *testing.T) {
	msg := MissingGatewayClassMessage("envoy")
	if msg == "" {
		t.Fatal("empty message")
	}
}
