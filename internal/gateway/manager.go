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
	"encoding/json"
	"strings"

	satisfactoryv1alpha1 "github.com/DataKnifeAI/satisfactory-operator/api/v1alpha1"
	"github.com/DataKnifeAI/satisfactory-operator/internal/config"
	egv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	ListenerServerManagerHTTPS = "server-manager-https"
	ComponentServerManagerGW   = "server-manager-gateway"
	serverManagerHTTPTimeout   = gatewayv1.Duration("3600s")
)

// ManagerNames holds HTTPS-only Gateway resources for Server Manager.
type ManagerNames struct {
	Gateway    string
	EnvoyProxy string
	HTTPRoute  string
}

// DeriveManagerNames returns HTTPS Gateway resource names.
func DeriveManagerNames(server *satisfactoryv1alpha1.SatisfactoryServer) ManagerNames {
	base := strings.TrimSuffix(server.Name, "-server")
	return ManagerNames{
		Gateway:    server.Name + "-manager-gateway",
		EnvoyProxy: "game-" + base + "-manager",
		HTTPRoute:  server.Name + "-server-manager",
	}
}

func managerLabels(server *satisfactoryv1alpha1.SatisfactoryServer) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "satisfactory-server",
		"app.kubernetes.io/instance":   server.Name,
		"app.kubernetes.io/managed-by": "satisfactory-operator",
		"app.kubernetes.io/component":  ComponentServerManagerGW,
	}
}

// BuildManagerEnvoyProxy creates a ClusterIP Envoy Service patched with
// externalIPs=[gateway.address] so HTTPS can share the game VIP without a
// second kube-vip LoadBalancer claim (UDP stays on the game Service LB).
func BuildManagerEnvoyProxy(server *satisfactoryv1alpha1.SatisfactoryServer) (*egv1a1.EnvoyProxy, error) {
	names := DeriveManagerNames(server)
	vip := server.Spec.Gateway.Address
	patchObj := map[string]any{
		"spec": map[string]any{
			"externalIPs": []string{vip},
		},
	}
	raw, err := json.Marshal(patchObj)
	if err != nil {
		return nil, err
	}
	return &egv1a1.EnvoyProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.EnvoyProxy,
			Namespace: server.Namespace,
			Labels:    managerLabels(server),
		},
		Spec: egv1a1.EnvoyProxySpec{
			Logging: egv1a1.ProxyLogging{
				Level: map[egv1a1.ProxyLogComponent]egv1a1.LogLevel{
					egv1a1.LogComponentDefault: egv1a1.LogLevelWarn,
				},
			},
			Provider: &egv1a1.EnvoyProxyProvider{
				Type: egv1a1.ProviderTypeKubernetes,
				Kubernetes: &egv1a1.EnvoyProxyKubernetesProvider{
					EnvoyService: &egv1a1.KubernetesServiceSpec{
						Type:                  ptr.To(egv1a1.ServiceTypeClusterIP),
						ExternalTrafficPolicy: ptr.To(egv1a1.ServiceExternalTrafficPolicyCluster),
						Patch: &egv1a1.KubernetesPatchSpec{
							Type:  ptr.To(egv1a1.StrategicMerge),
							Value: apiextensionsv1.JSON{Raw: raw},
						},
					},
				},
			},
		},
	}, nil
}

// BuildManagerGateway is an HTTPS-only Gateway for the Server Manager UI.
func BuildManagerGateway(server *satisfactoryv1alpha1.SatisfactoryServer, tlsSecretName string) *gatewayv1.Gateway {
	names := DeriveManagerNames(server)
	host := config.ServerManagerHostname(server.Spec)
	sameNS := &gatewayv1.AllowedRoutes{
		Namespaces: &gatewayv1.RouteNamespaces{From: ptr.To(gatewayv1.NamespacesFromSame)},
	}
	listener := gatewayv1.Listener{
		Name:          gatewayv1.SectionName(ListenerServerManagerHTTPS),
		Port:          gatewayv1.PortNumber(config.ServerManagerHTTPSPort(server.Spec)),
		Protocol:      gatewayv1.HTTPSProtocolType,
		AllowedRoutes: sameNS,
		TLS: &gatewayv1.GatewayTLSConfig{
			Mode: ptr.To(gatewayv1.TLSModeTerminate),
			CertificateRefs: []gatewayv1.SecretObjectReference{{
				Name: gatewayv1.ObjectName(tlsSecretName),
			}},
		},
	}
	if host != "" {
		listener.Hostname = ptr.To(gatewayv1.Hostname(host))
	}
	return &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.Gateway,
			Namespace: server.Namespace,
			Labels:    managerLabels(server),
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(className(server.Spec)),
			Infrastructure: &gatewayv1.GatewayInfrastructure{
				ParametersRef: &gatewayv1.LocalParametersReference{
					Group: gatewayv1.Group(egv1a1.GroupVersion.Group),
					Kind:  gatewayv1.Kind("EnvoyProxy"),
					Name:  names.EnvoyProxy,
				},
			},
			Listeners: []gatewayv1.Listener{listener},
		},
	}
}

// BuildManagerHTTPRoute routes hostname → game Service :serverManager.port.
func BuildManagerHTTPRoute(server *satisfactoryv1alpha1.SatisfactoryServer) *gatewayv1.HTTPRoute {
	names := DeriveManagerNames(server)
	host := config.ServerManagerHostname(server.Spec)
	port := config.ServerManagerPort(server.Spec)
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.HTTPRoute,
			Namespace: server.Namespace,
			Labels:    managerLabels(server),
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name:        gatewayv1.ObjectName(names.Gateway),
					Namespace:   ptr.To(gatewayv1.Namespace(server.Namespace)),
					SectionName: ptr.To(gatewayv1.SectionName(ListenerServerManagerHTTPS)),
				}},
			},
			Rules: []gatewayv1.HTTPRouteRule{{
				Timeouts: &gatewayv1.HTTPRouteTimeouts{
					Request:        ptr.To(serverManagerHTTPTimeout),
					BackendRequest: ptr.To(serverManagerHTTPTimeout),
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(server.Name),
							Port: ptr.To(gatewayv1.PortNumber(port)),
						},
					},
				}},
			}},
		},
	}
	if host != "" {
		route.Spec.Hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(host)}
	}
	return route
}
