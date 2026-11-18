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
	"github.com/DataKnifeAI/satisfactory-operator/internal/provisioner"
	egv1a1 "github.com/envoyproxy/gateway/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const finalizerName = "satisfactory.dataknife.ai/finalizer"

// SatisfactoryServerReconciler reconciles a SatisfactoryServer object.
type SatisfactoryServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Kube   kubernetes.Interface
}

// +kubebuilder:rbac:groups=satisfactory.dataknife.ai,resources=satisfactoryservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=satisfactory.dataknife.ai,resources=satisfactoryservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=satisfactory.dataknife.ai,resources=satisfactoryservers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// Operator must hold verbs it grants into namespaced Roles (RBAC escalation rules).
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=udproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=envoyproxies,verbs=get;list;watch;create;update;patch;delete

func (r *SatisfactoryServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(server, finalizerName) {
		controllerutil.AddFinalizer(server, finalizerName)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !server.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(server, finalizerName)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := config.ValidateSpec(server.Spec); err != nil {
		_ = r.patchStatus(ctx, server, func(s *satisfactoryv1alpha1.SatisfactoryServer) {
			s.Status.Phase = satisfactoryv1alpha1.PhaseFailed
			s.Status.Ready = false
			s.Status.Message = err.Error()
			meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
				Type:               satisfactoryv1alpha1.ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             "ValidationFailed",
				Message:            err.Error(),
				ObservedGeneration: s.Generation,
			})
		})
		log.Info("validation failed", "error", err)
		return ctrl.Result{}, nil
	}

	secret, generated, err := r.ensureSecret(ctx, server)
	if err != nil {
		return ctrl.Result{}, err
	}
	secretName := ""
	if secret != nil {
		secretName = secret.Name
		if _, err := provisioner.ReadAdminPassword(secret); err != nil {
			_ = r.patchStatus(ctx, server, func(s *satisfactoryv1alpha1.SatisfactoryServer) {
				s.Status.Phase = satisfactoryv1alpha1.PhaseFailed
				s.Status.Ready = false
				s.Status.Message = err.Error()
				meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
					Type:               satisfactoryv1alpha1.ConditionCredentialsValid,
					Status:             metav1.ConditionFalse,
					Reason:             "CredentialsInvalid",
					Message:            err.Error(),
					ObservedGeneration: s.Generation,
				})
			})
			return ctrl.Result{}, nil
		}
	}

	cm := provisioner.BuildConfigMap(server)
	if err := controllerutil.SetControllerReference(server, cm, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.applyObject(ctx, cm); err != nil {
		return ctrl.Result{}, err
	}

	pvc := provisioner.BuildPVC(server)
	if err := controllerutil.SetControllerReference(server, pvc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.applyObject(ctx, pvc); err != nil {
		return ctrl.Result{}, err
	}

	svc := provisioner.BuildService(server)
	if err := controllerutil.SetControllerReference(server, svc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.applyObject(ctx, svc); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileServerManager(ctx, server, secretName); err != nil {
		return ctrl.Result{}, err
	}

	// Backup PVC must exist before the Deployment mounts it for the UI sidecar.
	if err := r.reconcileBackup(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	configHash := cm.Annotations[provisioner.AnnotationConfigHash]
	deploy := provisioner.BuildDeployment(server, configHash, secretName)
	if err := controllerutil.SetControllerReference(server, deploy, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.applyObject(ctx, deploy); err != nil {
		return ctrl.Result{}, err
	}

	gw, err := r.reconcileGatewayStack(ctx, server)
	if err != nil {
		return ctrl.Result{}, err
	}

	ready := false
	phase := satisfactoryv1alpha1.PhaseDeploying
	message := "reconciling child resources"
	current := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: server.Namespace, Name: deploy.Name}, current); err == nil {
		if current.Status.ReadyReplicas >= 1 {
			ready = true
			phase = satisfactoryv1alpha1.PhaseRunning
			message = "game server Deployment is ready"
		}
	}
	if gw.Message != "" {
		if message == "reconciling child resources" || ready {
			message = message + "; " + gw.Message
		}
	}
	if server.Spec.ServerManager.Enabled {
		smAddr, smPort := serverManagerStatus(server)
		if smAddr != "" {
			scheme := "http"
			if config.ServerManagerHTTPSEnabled(server.Spec) {
				scheme = "https"
			}
			message = message + fmt.Sprintf("; server manager %s://%s:%d (admin + admin-password)", scheme, smAddr, smPort)
		}
	}

	connAddr := gw.Address
	if !config.GatewayEnabled(server.Spec) {
		// Direct Service exposure (NodePort / LoadBalancer VIP).
		connAddr = server.Spec.Gateway.Address
	}
	smAddr, smPort := serverManagerStatus(server)

	obs := observedRuntime{}
	if ready {
		obs = r.observeRuntime(ctx, server)
	}

	_ = r.patchStatus(ctx, server, func(s *satisfactoryv1alpha1.SatisfactoryServer) {
		s.Status.Phase = phase
		s.Status.Ready = ready
		s.Status.Message = message
		s.Status.ConnectionAddress = connAddr
		s.Status.ConnectionPort = config.GamePort(server.Spec)
		s.Status.ReliablePort = config.ReliablePort(server.Spec)
		s.Status.ServerManagerAddress = smAddr
		s.Status.ServerManagerPort = smPort
		s.Status.CredentialsSecretName = secretName
		s.Status.CredentialsGenerated = generated
		s.Status.ImageVersion = config.ImageRef(server.Spec)
		s.Status.ServerName = config.ServerName(server.Spec, server.Name)
		_ = obs // runtime log parse reserved for later (build id / claim state)
		s.Status.ObservedGeneration = server.Generation

		credStatus := metav1.ConditionTrue
		credReason := "CredentialsOptional"
		credMsg := "credentials not required for game start (claim via Satisfactory client)"
		if secretName != "" {
			credReason = "CredentialsReady"
			credMsg = fmt.Sprintf("using secret %s", secretName)
		}
		meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
			Type:               satisfactoryv1alpha1.ConditionCredentialsValid,
			Status:             credStatus,
			Reason:             credReason,
			Message:            credMsg,
			ObservedGeneration: server.Generation,
		})
		meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
			Type:               satisfactoryv1alpha1.ConditionReady,
			Status:             boolCond(ready),
			Reason:             readyReason(ready),
			Message:            message,
			ObservedGeneration: server.Generation,
		})
		gwStatus := metav1.ConditionFalse
		gwReason := "GatewayPending"
		if gw.Configured {
			gwStatus = metav1.ConditionTrue
			gwReason = "GatewayConfigured"
		}
		meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
			Type:               satisfactoryv1alpha1.ConditionGatewayConfigured,
			Status:             gwStatus,
			Reason:             gwReason,
			Message:            gw.Message,
			ObservedGeneration: server.Generation,
		})
	})

	return ctrl.Result{}, nil
}

func (r *SatisfactoryServerReconciler) ensureSecret(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) (*corev1.Secret, bool, error) {
	needsSecret := server.Spec.GenerateSecrets ||
		(server.Spec.CredentialsSecretRef != nil && server.Spec.CredentialsSecretRef.Name != "") ||
		server.Spec.ServerManager.Enabled
	if !needsSecret {
		return nil, false, nil
	}

	name := provisioner.SecretNameFor(server)
	existing := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: server.Namespace, Name: name}, existing)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false, err
	}
	found := err == nil

	if !server.Spec.GenerateSecrets {
		if !found {
			return nil, false, fmt.Errorf("credentials secret %q not found", name)
		}
		return existing, false, nil
	}

	var base *corev1.Secret
	if found {
		base = existing
	}
	desired, err := provisioner.DesiredSecret(server, base)
	if err != nil {
		return nil, false, err
	}
	if err := controllerutil.SetControllerReference(server, desired, r.Scheme); err != nil {
		return nil, false, err
	}
	if !found {
		if err := r.Create(ctx, desired); err != nil {
			return nil, false, err
		}
		return desired, true, nil
	}
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.Update(ctx, desired); err != nil {
		return nil, false, err
	}
	return desired, true, nil
}

func (r *SatisfactoryServerReconciler) applyObject(ctx context.Context, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	existing := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, obj)
	}
	if err != nil {
		return err
	}

	switch desired := obj.(type) {
	case *corev1.ConfigMap:
		cur := existing.(*corev1.ConfigMap)
		cur.Data = desired.Data
		cur.Labels = desired.Labels
		cur.Annotations = desired.Annotations
		return r.Update(ctx, cur)
	case *corev1.Service:
		cur := existing.(*corev1.Service)
		cur.Spec.Type = desired.Spec.Type
		cur.Spec.Ports = desired.Spec.Ports
		cur.Spec.Selector = desired.Spec.Selector
		cur.Spec.LoadBalancerIP = desired.Spec.LoadBalancerIP
		cur.Spec.ExternalTrafficPolicy = desired.Spec.ExternalTrafficPolicy
		// Local→Cluster updates must drop healthCheckNodePort or the API rejects the change.
		if desired.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
			cur.Spec.HealthCheckNodePort = 0
		}
		cur.Labels = desired.Labels
		return r.Update(ctx, cur)
	case *appsv1.Deployment:
		cur := existing.(*appsv1.Deployment)
		cur.Spec = desired.Spec
		cur.Labels = desired.Labels
		return r.Update(ctx, cur)
	case *corev1.PersistentVolumeClaim:
		// PVC spec is mostly immutable; only ensure labels.
		cur := existing.(*corev1.PersistentVolumeClaim)
		cur.Labels = desired.Labels
		return r.Update(ctx, cur)
	case *egv1a1.EnvoyProxy:
		cur := existing.(*egv1a1.EnvoyProxy)
		cur.Labels = desired.Labels
		cur.OwnerReferences = desired.OwnerReferences
		cur.Spec = desired.Spec
		return r.Update(ctx, cur)
	case *batchv1.CronJob:
		cur := existing.(*batchv1.CronJob)
		cur.Labels = desired.Labels
		cur.Spec = desired.Spec
		return r.Update(ctx, cur)
	case *corev1.ServiceAccount:
		cur := existing.(*corev1.ServiceAccount)
		cur.Labels = desired.Labels
		return r.Update(ctx, cur)
	case *rbacv1.Role:
		cur := existing.(*rbacv1.Role)
		cur.Labels = desired.Labels
		cur.Rules = desired.Rules
		return r.Update(ctx, cur)
	case *rbacv1.RoleBinding:
		cur := existing.(*rbacv1.RoleBinding)
		cur.Labels = desired.Labels
		cur.Subjects = desired.Subjects
		cur.RoleRef = desired.RoleRef
		return r.Update(ctx, cur)
	case *gatewayv1.Gateway:
		cur := existing.(*gatewayv1.Gateway)
		cur.Labels = desired.Labels
		cur.OwnerReferences = desired.OwnerReferences
		cur.Spec = desired.Spec
		return r.Update(ctx, cur)
	case *gatewayv1.HTTPRoute:
		cur := existing.(*gatewayv1.HTTPRoute)
		cur.Labels = desired.Labels
		cur.OwnerReferences = desired.OwnerReferences
		cur.Spec = desired.Spec
		return r.Update(ctx, cur)
	case *gatewayv1alpha2.UDPRoute:
		cur := existing.(*gatewayv1alpha2.UDPRoute)
		cur.Labels = desired.Labels
		cur.OwnerReferences = desired.OwnerReferences
		cur.Spec = desired.Spec
		return r.Update(ctx, cur)
	default:
		return fmt.Errorf("unsupported apply type %T", obj)
	}
}

func (r *SatisfactoryServerReconciler) patchStatus(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer, mutate func(*satisfactoryv1alpha1.SatisfactoryServer)) error {
	latest := &satisfactoryv1alpha1.SatisfactoryServer{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(server), latest); err != nil {
		return err
	}
	mutate(latest)
	return r.Status().Update(ctx, latest)
}

func boolCond(ready bool) metav1.ConditionStatus {
	if ready {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func readyReason(ready bool) string {
	if ready {
		return "DeploymentReady"
	}
	return "DeploymentNotReady"
}

func (r *SatisfactoryServerReconciler) reconcileBackup(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) error {
	if !server.Spec.Backup.Enabled {
		cj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{
			Name:      provisioner.BackupCronJobNameFor(server),
			Namespace: server.Namespace,
		}}
		if err := r.Delete(ctx, cj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		// Keep backup PVC when disabled so archives are not wiped by a toggle.
		return nil
	}

	bpvc := provisioner.BuildBackupPVC(server)
	if err := controllerutil.SetControllerReference(server, bpvc, r.Scheme); err != nil {
		return err
	}
	if err := r.applyObject(ctx, bpvc); err != nil {
		return err
	}

	cj := provisioner.BuildBackupCronJob(server)
	if err := controllerutil.SetControllerReference(server, cj, r.Scheme); err != nil {
		return err
	}
	return r.applyObject(ctx, cj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SatisfactoryServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.SatisfactoryServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&gatewayv1.Gateway{}).
		Owns(&gatewayv1alpha2.UDPRoute{}).
		Owns(&egv1a1.EnvoyProxy{}).
		Named("satisfactoryserver").
		Complete(r)
}
