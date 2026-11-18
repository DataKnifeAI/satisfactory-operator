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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type gatewayResult struct {
	Configured bool
	Address    string
	Message    string
}

func (r *SatisfactoryServerReconciler) reconcileGatewayStack(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
) (gatewayResult, error) {
	names := gateway.DeriveNames(server)

	if !config.GatewayEnabled(server.Spec) {
		_ = r.deleteOwnedGatewayStack(ctx, server, names)
		msg := "gateway disabled; using Service only (set gateway.serviceType=NodePort|LoadBalancer)"
		addr := server.Spec.Gateway.Address
		if server.Spec.Gateway.ServiceType == corev1.ServiceTypeLoadBalancer && addr != "" {
			msg = fmt.Sprintf("gateway disabled; Service LoadBalancer VIP %s (game %d TCP/UDP, reliable %d TCP)", addr, config.GamePort(server.Spec), config.ReliablePort(server.Spec))
		}
		return gatewayResult{
			Configured: true,
			Address:    addr,
			Message:    msg,
		}, nil
	}

	class := server.Spec.Gateway.ClassName
	if class == "" {
		class = "envoy"
	}
	gc := &gatewayv1.GatewayClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: class}, gc); err != nil {
		if apierrors.IsNotFound(err) {
			return gatewayResult{
				Configured: false,
				Message:    gateway.MissingGatewayClassMessage(class),
			}, nil
		}
		return gatewayResult{}, err
	}
	accepted := false
	for _, c := range gc.Status.Conditions {
		if c.Type == string(gatewayv1.GatewayClassConditionStatusAccepted) && c.Status == metav1.ConditionTrue {
			accepted = true
			break
		}
	}
	if !accepted {
		return gatewayResult{
			Configured: false,
			Message:    fmt.Sprintf("GatewayClass %q exists but is not Accepted", class),
		}, nil
	}

	envoySvc := gateway.BuildEnvoyBackendService(server)
	if err := controllerutil.SetControllerReference(server, envoySvc, r.Scheme); err != nil {
		return gatewayResult{}, err
	}
	if err := r.applyObject(ctx, envoySvc); err != nil {
		return gatewayResult{}, err
	}

	ep := gateway.BuildEnvoyProxy(server)
	if err := controllerutil.SetControllerReference(server, ep, r.Scheme); err != nil {
		return gatewayResult{}, err
	}
	if err := r.applyObject(ctx, ep); err != nil {
		return gatewayResult{}, err
	}

	gw := gateway.BuildGateway(server)
	if err := controllerutil.SetControllerReference(server, gw, r.Scheme); err != nil {
		return gatewayResult{}, err
	}
	if err := r.applyObject(ctx, gw); err != nil {
		return gatewayResult{}, err
	}

	gameRoute := gateway.BuildUDPRoute(server, names.GameRoute, gateway.ListenerGameUDP, config.GamePort(server.Spec))
	if err := controllerutil.SetControllerReference(server, gameRoute, r.Scheme); err != nil {
		return gatewayResult{}, err
	}
	if err := r.applyObject(ctx, gameRoute); err != nil {
		return gatewayResult{}, err
	}

	beaconRoute := gateway.BuildUDPRoute(server, names.BeaconRoute, gateway.ListenerBeaconUDP, config.BeaconPort(server.Spec))
	if err := controllerutil.SetControllerReference(server, beaconRoute, r.Scheme); err != nil {
		return gatewayResult{}, err
	}
	if err := r.applyObject(ctx, beaconRoute); err != nil {
		return gatewayResult{}, err
	}

	live := &gatewayv1.Gateway{}
	_ = r.Get(ctx, client.ObjectKey{Namespace: server.Namespace, Name: names.Gateway}, live)
	addr := gateway.ConnectionAddress(server, live)
	return gatewayResult{
		Configured: true,
		Address:    addr,
		Message:    fmt.Sprintf("Envoy Gateway %s on %s (experimental; prefer LoadBalancer VIP) game UDP %d", names.Gateway, addr, config.GamePort(server.Spec)),
	}, nil
}

func (r *SatisfactoryServerReconciler) deleteOwnedGatewayStack(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
	names gateway.Names,
) error {
	toDelete := []client.Object{
		&gatewayv1alpha2.UDPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.GameRoute, Namespace: server.Namespace}},
		&gatewayv1alpha2.UDPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.BeaconRoute, Namespace: server.Namespace}},
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
