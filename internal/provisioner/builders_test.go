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
	"testing"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sampleServer() *satisfactoryv1alpha1.SatisfactoryServer {
	gw := false
	return &satisfactoryv1alpha1.SatisfactoryServer{
		ObjectMeta: metav1.ObjectMeta{Name: "ashenfall-home", Namespace: "games"},
		Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
			ServerName:      "Factory",
			MaxPlayers:      4,
			GamePort:        7777,
			ReliablePort:    8888,
			GenerateSecrets: true,
			Gateway:         satisfactoryv1alpha1.GatewayConfig{Enabled: &gw, ServiceType: corev1.ServiceTypeNodePort},
		},
	}
}

func TestBuildDeployment(t *testing.T) {
	server := sampleServer()
	deploy := BuildDeployment(server, "abc123", "ashenfall-home-secrets")
	if deploy.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType {
		t.Fatalf("strategy = %s, want Recreate", deploy.Spec.Strategy.Type)
	}
	c := deploy.Spec.Template.Spec.Containers[0]
	if c.Image != satisfactoryv1alpha1.DefaultImage {
		t.Fatalf("image = %s", c.Image)
	}
	foundPlayers := false
	for _, e := range c.Env {
		if e.Name == "MAXPLAYERS" && e.Value == "4" {
			foundPlayers = true
		}
	}
	if !foundPlayers {
		t.Fatal("missing MAXPLAYERS env")
	}
	if deploy.Spec.Template.Annotations[AnnotationConfigHash] != "abc123" {
		t.Fatal("missing config hash annotation")
	}
}

func TestBuildServiceNodePort(t *testing.T) {
	svc := BuildService(sampleServer())
	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Fatalf("type = %s, want NodePort", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 3 {
		t.Fatalf("ports = %d, want 3 (game-udp, game-tcp, reliable)", len(svc.Spec.Ports))
	}
}

func TestBuildServiceLoadBalancerVIP(t *testing.T) {
	gw := false
	server := sampleServer()
	server.Spec.Gateway = satisfactoryv1alpha1.GatewayConfig{
		Enabled:     &gw,
		ServiceType: corev1.ServiceTypeLoadBalancer,
		Address:     "192.168.14.188",
	}
	svc := BuildService(server)
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Fatalf("type = %s", svc.Spec.Type)
	}
	if svc.Spec.LoadBalancerIP != "192.168.14.188" {
		t.Fatalf("loadBalancerIP = %q", svc.Spec.LoadBalancerIP)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyCluster {
		t.Fatalf("etp = %s", svc.Spec.ExternalTrafficPolicy)
	}
}

func TestBuildBackupCronJob(t *testing.T) {
	server := sampleServer()
	server.Spec.Backup = satisfactoryv1alpha1.BackupConfig{Enabled: true, Retain: 3}
	cj := BuildBackupCronJob(server)
	if cj.Name != "ashenfall-home-backup" {
		t.Fatalf("name = %q", cj.Name)
	}
	if cj.Spec.Schedule == "" {
		t.Fatal("empty schedule")
	}
	pvc := BuildBackupPVC(server)
	if pvc.Name != "ashenfall-home-backups" {
		t.Fatalf("pvc = %q", pvc.Name)
	}
}

func TestDesiredSecret(t *testing.T) {
	server := sampleServer()
	sec, err := DesiredSecret(server, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sec.Data[satisfactoryv1alpha1.SecretKeyAdminPassword]) == 0 {
		t.Fatal("admin password empty")
	}
	// Second call must preserve existing password.
	sec2, err := DesiredSecret(server, sec)
	if err != nil {
		t.Fatal(err)
	}
	if string(sec2.Data[satisfactoryv1alpha1.SecretKeyAdminPassword]) != string(sec.Data[satisfactoryv1alpha1.SecretKeyAdminPassword]) {
		t.Fatal("admin password was overwritten")
	}
}
