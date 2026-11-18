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
	"crypto/rand"
	"encoding/base64"
	"fmt"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const generatedPasswordBytes = 18

func generatePassword() (string, error) {
	buf := make([]byte, generatedPasswordBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// DesiredSecret builds (or merges into) the credentials Secret.
// Existing non-empty keys are preserved.
func DesiredSecret(server *satisfactoryv1alpha1.SatisfactoryServer, existing *corev1.Secret) (*corev1.Secret, error) {
	if !server.Spec.GenerateSecrets {
		return nil, fmt.Errorf("generateSecrets is false")
	}

	data := map[string][]byte{}
	if existing != nil && existing.Data != nil {
		for k, v := range existing.Data {
			data[k] = v
		}
	}

	if len(data[satisfactoryv1alpha1.SecretKeyAdminPassword]) == 0 {
		pw, err := generatePassword()
		if err != nil {
			return nil, err
		}
		data[satisfactoryv1alpha1.SecretKeyAdminPassword] = []byte(pw)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName(server),
			Namespace: server.Namespace,
			Labels:    labelsFor(server),
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}, nil
}

// ReadAdminPassword extracts admin-password from a Secret.
func ReadAdminPassword(secret *corev1.Secret) (string, error) {
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("credentials secret is missing")
	}
	admin := string(secret.Data[satisfactoryv1alpha1.SecretKeyAdminPassword])
	if admin == "" {
		return "", fmt.Errorf("secret %q missing key %s", secret.Name, satisfactoryv1alpha1.SecretKeyAdminPassword)
	}
	return admin, nil
}

// ReadPasswords is a compatibility wrapper (world password unused for robtme).
func ReadPasswords(secret *corev1.Secret) (admin, world string, err error) {
	admin, err = ReadAdminPassword(secret)
	return admin, "", err
}
