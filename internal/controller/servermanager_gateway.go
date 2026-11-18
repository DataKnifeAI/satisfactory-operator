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

package controller

import (
	"context"
	"fmt"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	"github.com/DataKnifeAI/satisfactory-operator/internal/config"
	"github.com/DataKnifeAI/satisfactory-operator/internal/gateway"
	egv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (r *SatisfactoryServerReconciler) reconcileServerManagerGateway(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
) error {
	names := gateway.DeriveManagerNames(server)
	if !config.ServerManagerHTTPSEnabled(server.Spec) {
		return r.deleteServerManagerGateway(ctx, server, names)
	}

	tlsName, err := r.reconcileServerManagerTLSSecret(ctx, server)
	if err != nil {
		return err
	}

	ep, err := gateway.BuildManagerEnvoyProxy(server)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(server, ep, r.Scheme); err != nil {
		return err
	}
	if err := r.applyObject(ctx, ep); err != nil {
		return err
	}

	gw := gateway.BuildManagerGateway(server, tlsName)
	if err := controllerutil.SetControllerReference(server, gw, r.Scheme); err != nil {
		return err
	}
	if err := r.applyObject(ctx, gw); err != nil {
		return err
	}

	route := gateway.BuildManagerHTTPRoute(server)
	if err := controllerutil.SetControllerReference(server, route, r.Scheme); err != nil {
		return err
	}
	return r.applyObject(ctx, route)
}

func (r *SatisfactoryServerReconciler) reconcileServerManagerTLSSecret(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
) (string, error) {
	ref := server.Spec.ServerManager.TLSSecretRef
	if ref == nil || ref.Name == "" {
		return "", fmt.Errorf("spec.serverManager.tlsSecretRef is required")
	}
	srcNS := ref.Namespace
	if srcNS == "" {
		srcNS = server.Namespace
	}
	if srcNS == server.Namespace {
		return ref.Name, nil
	}

	src := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: srcNS}, src); err != nil {
		return "", fmt.Errorf("read TLS secret %s/%s: %w", srcNS, ref.Name, err)
	}
	cert := src.Data[corev1.TLSCertKey]
	key := src.Data[corev1.TLSPrivateKeyKey]
	if len(cert) == 0 || len(key) == 0 {
		return "", fmt.Errorf("TLS secret %s/%s missing tls.crt or tls.key", srcNS, ref.Name)
	}

	dst := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: server.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dst, func() error {
		if err := controllerutil.SetControllerReference(server, dst, r.Scheme); err != nil {
			return err
		}
		dst.Type = corev1.SecretTypeTLS
		dst.Data = map[string][]byte{
			corev1.TLSCertKey:       cert,
			corev1.TLSPrivateKeyKey: key,
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("copy TLS secret %s/%s: %w", srcNS, ref.Name, err)
	}
	logf.FromContext(ctx).V(1).Info("copied TLS secret for server manager", "from", srcNS+"/"+ref.Name)
	return ref.Name, nil
}

func (r *SatisfactoryServerReconciler) deleteServerManagerGateway(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
	names gateway.ManagerNames,
) error {
	toDelete := []client.Object{
		&gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.HTTPRoute, Namespace: server.Namespace}},
		&gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: names.Gateway, Namespace: server.Namespace}},
		&egv1a1.EnvoyProxy{ObjectMeta: metav1.ObjectMeta{Name: names.EnvoyProxy, Namespace: server.Namespace}},
	}
	for _, obj := range toDelete {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
