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

package controller

import (
	"context"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	"github.com/DataKnifeAI/satisfactory-operator/internal/config"
	"github.com/DataKnifeAI/satisfactory-operator/internal/gateway"
	"github.com/DataKnifeAI/satisfactory-operator/internal/provisioner"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *SatisfactoryServerReconciler) reconcileServerManager(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
	secretName string,
) error {
	if !server.Spec.ServerManager.Enabled {
		_ = r.deleteServerManagerGateway(ctx, server, gateway.DeriveManagerNames(server))
		return r.deleteServerManagerRBAC(ctx, server)
	}

	sa := provisioner.BuildServerManagerServiceAccount(server)
	if err := controllerutil.SetControllerReference(server, sa, r.Scheme); err != nil {
		return err
	}
	if err := r.applyObject(ctx, sa); err != nil {
		return err
	}

	role := provisioner.BuildServerManagerRole(server, secretName)
	if err := controllerutil.SetControllerReference(server, role, r.Scheme); err != nil {
		return err
	}
	if err := r.applyObject(ctx, role); err != nil {
		return err
	}

	binding := provisioner.BuildServerManagerRoleBinding(server)
	if err := controllerutil.SetControllerReference(server, binding, r.Scheme); err != nil {
		return err
	}
	if err := r.applyObject(ctx, binding); err != nil {
		return err
	}
	return r.reconcileServerManagerGateway(ctx, server)
}

func (r *SatisfactoryServerReconciler) deleteServerManagerRBAC(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
) error {
	objs := []client.Object{
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: provisioner.ServerManagerRoleNameFor(server), Namespace: server.Namespace}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: provisioner.ServerManagerRoleNameFor(server), Namespace: server.Namespace}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: provisioner.ServerManagerSANameFor(server), Namespace: server.Namespace}},
	}
	for _, obj := range objs {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func serverManagerStatus(server *satisfactoryv1alpha1.SatisfactoryServer) (addr string, port int32) {
	if !server.Spec.ServerManager.Enabled {
		return "", 0
	}
	if config.ServerManagerHTTPSEnabled(server.Spec) {
		return config.ServerManagerHostname(server.Spec), config.ServerManagerHTTPSPort(server.Spec)
	}
	addr = server.Spec.Gateway.Address
	if addr == "" {
		addr = server.Status.ConnectionAddress
	}
	return addr, config.ServerManagerPort(server.Spec)
}
