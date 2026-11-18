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

package servermanager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

// Runtime is the in-cluster client for CR / Secret / Deployment / Jobs.
type Runtime struct {
	client    client.Client
	kube      kubernetes.Interface
	namespace string
	crName    string
	deploy    string
	secret    string
}

// NewRuntime builds an in-cluster Runtime.
func NewRuntime(namespace, crName, deploy, secret string) (*Runtime, error) {
	if namespace == "" || crName == "" {
		return nil, fmt.Errorf("namespace and CR name are required")
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	scheme := runtime.NewScheme()
	if err := satisfactoryv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	if deploy == "" {
		deploy = crName
	}
	if secret == "" {
		secret = crName + "-secrets"
	}
	return &Runtime{
		client:    c,
		kube:      kube,
		namespace: namespace,
		crName:    crName,
		deploy:    deploy,
		secret:    secret,
	}, nil
}

// GetServer returns the SatisfactoryServer CR.
func (r *Runtime) GetServer(ctx context.Context) (*satisfactoryv1alpha1.SatisfactoryServer, error) {
	out := &satisfactoryv1alpha1.SatisfactoryServer{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: r.namespace, Name: r.crName}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateServer writes the CR.
func (r *Runtime) UpdateServer(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) error {
	return r.client.Update(ctx, server)
}

// GetSecret returns the credentials Secret.
func (r *Runtime) GetSecret(ctx context.Context) (*corev1.Secret, error) {
	out := &corev1.Secret{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: r.namespace, Name: r.secret}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateSecret writes the credentials Secret.
func (r *Runtime) UpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	return r.client.Update(ctx, secret)
}

// Restart annotates the Deployment pod template to Recreate-roll.
func (r *Runtime) Restart(ctx context.Context) error {
	payload, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]string{
						restartedAtAnnotation: time.Now().UTC().Format(time.RFC3339Nano),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	_, err = r.kube.AppsV1().Deployments(r.namespace).Patch(
		ctx, r.deploy, types.StrategicMergePatchType, payload, metav1.PatchOptions{})
	return err
}

// TriggerBackup creates a one-shot Job from the backup CronJob.
func (r *Runtime) TriggerBackup(ctx context.Context) (string, error) {
	cjName := r.crName + "-backup"
	cj, err := r.kube.BatchV1().CronJobs(r.namespace).Get(ctx, cjName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get CronJob %s: %w (enable spec.backup first)", cjName, err)
	}
	jobName := fmt.Sprintf("%s-manual-%d", cjName, time.Now().Unix())
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.namespace,
			Labels:    map[string]string{"app.kubernetes.io/component": "backup"},
		},
		Spec: cj.Spec.JobTemplate.Spec,
	}
	created, err := r.kube.BatchV1().Jobs(r.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return created.Name, nil
}
