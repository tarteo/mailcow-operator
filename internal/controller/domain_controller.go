/*
Copyright 2026.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mailcowv1 "github.com/tarteo/mailcow-operator/api/v1"
	constants "github.com/tarteo/mailcow-operator/common"
	helpers "github.com/tarteo/mailcow-operator/helpers"
	"github.com/tarteo/mailcow-operator/mailcow"
)

// DomainReconciler reconciles a Domain object
type DomainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=domains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=domains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=domains/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Domain object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *DomainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("namespace", req.NamespacedName)
	log.Info("reconciling domain")

	var domain mailcowv1.Domain
	if err := r.Get(ctx, req.NamespacedName, &domain); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to find domain")
		return ctrl.Result{}, err
	}

	// Apply finalizer
	if domain.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(&domain, constants.Finalizer) {
		controllerutil.AddFinalizer(&domain, constants.Finalizer)
		if err := r.Update(ctx, &domain); err != nil {
			log.Error(err, "unable to update domain with finalizer")
			return ctrl.Result{}, err
		}

		// Return and requeue to get fresh object
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.ReconcileResource(ctx, &domain); err != nil {
		log.Error(err, "unable to reconcile mailcow domain")
		return ctrl.Result{}, err
	}

	// Remove finalizer if deletion timestamp is set
	if !domain.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(&domain, constants.Finalizer) {
		controllerutil.RemoveFinalizer(&domain, constants.Finalizer)
		if err := r.Update(ctx, &domain); err != nil {
			log.Error(err, "unable to update domain with finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DomainReconciler) ReconcileResource(ctx context.Context, domain *mailcowv1.Domain) error {
	log := log.FromContext(ctx).WithValues("namespace", types.NamespacedName{Namespace: domain.Namespace, Name: domain.Name})
	var err error

	// Get related mailcow resource
	var res mailcowv1.Mailcow
	if err := r.Get(ctx, types.NamespacedName{Name: domain.Spec.Mailcow, Namespace: domain.Namespace}, &res); err != nil {
		log.Error(err, "unable to find related mailcow resource", "mailcow", domain.Spec.Mailcow)
		return err
	}

	// Create mailcow client
	client, err := res.GetClient(ctx, r)
	if err != nil {
		log.Error(err, "unable to create mailcow client")
		return err
	}

	response, err := client.GetDomainsWithResponse(ctx, mailcow.GetDomainsParamsId(domain.Spec.Domain), nil)
	if err != nil {
		log.Error(err, "unable to get domain")
		return err
	}

	if !domain.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion
		if response.JSON200.DomainName != nil {
			_, err = client.DeleteDomainWithResponse(ctx, mailcow.DeleteDomainJSONRequestBody{domain.Spec.Domain})
			if err != nil {
				log.Error(err, "unable to delete domain")
				return err
			}
		}
		return nil
	}

	if response.JSON200.DomainName == nil {
		// Domain does not exist, create it
		var rlFrame = mailcow.CreateDomainJSONBodyRlFrame(domain.Spec.RateLimitFrame)
		_, err = client.CreateDomainWithResponse(ctx, mailcow.CreateDomainJSONRequestBody{
			Domain:      &domain.Spec.Domain,
			Description: &domain.Spec.Description,
			Quota:       helpers.Int64ToFloat32(&domain.Spec.Quota),
			Defquota:    helpers.Int64ToFloat32(&domain.Spec.DefQuota),
			Maxquota:    helpers.Int64ToFloat32(&domain.Spec.MaxQuota),
			Active:      domain.Spec.Active,
			Mailboxes:   helpers.Int64ToFloat32(&domain.Spec.MaxMailboxes),
			RlValue:     domain.Spec.RateLimit,
			RlFrame:     &rlFrame,
		})
	} else {
		// Domain exists, update it
		_, err = client.UpdateDomainWithResponse(ctx, mailcow.UpdateDomainJSONRequestBody{
			Attr: &mailcow.EditDomainAttr{
				Description: &domain.Spec.Description,
				Quota:       helpers.Int64ToFloat32(&domain.Spec.Quota),
				Defquota:    helpers.Int64ToFloat32(&domain.Spec.DefQuota),
				Maxquota:    helpers.Int64ToFloat32(&domain.Spec.MaxQuota),
				Active:      domain.Spec.Active,
				Mailboxes:   helpers.Int64ToFloat32(&domain.Spec.MaxMailboxes),
			},
			Items: &[]string{domain.Spec.Domain},
		})

		if err == nil {
			// Update rate limits, the main update endpoint doesn't handle rate limits
			_, err = client.EditDomainRatelimits(ctx, mailcow.EditDomainRatelimitsJSONRequestBody{
				Attr: &mailcow.EditRatelimitDomainAttr{
					RlValue: domain.Spec.RateLimit,
					RlFrame: &domain.Spec.RateLimitFrame,
				},
				Items: &[]string{domain.Spec.Domain},
			})
		}
	}

	if err != nil {
		log.Error(err, "unable to create or update domain")
		return err
	}

	// Reconcile DKIM
	if err := r.reconcileDKIM(ctx, client, domain); err != nil {
		log.Error(err, "unable to reconcile DKIM")
		return err
	}

	return nil
}

// reconcileDKIM handles DKIM key retrieval/generation and ConfigMap creation
func (r *DomainReconciler) reconcileDKIM(ctx context.Context, client *mailcow.ClientWithResponses, domain *mailcowv1.Domain) error {
	log := log.FromContext(ctx).WithValues("namespace", types.NamespacedName{Namespace: domain.Namespace, Name: domain.Name})

	// Try to get existing DKIM key
	dkimResponse, err := client.GetDKIMKeyWithResponse(ctx, domain.Spec.Domain, nil)
	if err != nil {
		log.Error(err, "unable to get DKIM key")
		return err
	}

	var dkimGetResponse *mailcow.GetDKIMKeyResponse

	// If DKIM key doesn't exist, generate one
	if dkimResponse.JSON200 == nil || dkimResponse.JSON200.DkimTxt == nil {
		log.Info("generating new DKIM key")
		keySize := float32(2048)
		selector := "dkim"
		_, err := client.GenerateDKIMKeyWithResponse(ctx, mailcow.GenerateDKIMKeyJSONRequestBody{
			Domains:      &domain.Spec.Domain,
			KeySize:      &keySize,
			DkimSelector: &selector,
		})
		if err != nil {
			log.Error(err, "unable to generate DKIM key")
			return err
		}

		// Retrieve the newly generated DKIM key
		dkimResponse, err = client.GetDKIMKeyWithResponse(ctx, domain.Spec.Domain, nil)
		if err != nil {
			log.Error(err, "unable to retrieve generated DKIM key")
			return err
		}
		dkimGetResponse = dkimResponse
	} else {
		dkimGetResponse = dkimResponse
	}

	dkimData := dkimGetResponse.JSON200

	// Create ConfigMap with DKIM data
	if dkimData != nil {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dkim-" + domain.Name,
				Namespace: domain.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(domain, mailcowv1.GroupVersion.WithKind("Domain")),
				},
			},
			Data: make(map[string]string),
		}

		if dkimData.DkimSelector != nil {
			configMap.Data["selector"] = *dkimData.DkimSelector
		}
		if dkimData.DkimTxt != nil {
			configMap.Data["txt"] = *dkimData.DkimTxt
		}
		if dkimData.Length != nil {
			configMap.Data["length"] = *dkimData.Length
		}
		if dkimData.Pubkey != nil {
			configMap.Data["pubkey"] = *dkimData.Pubkey
		}

		// Try to get existing ConfigMap
		var existingConfigMap corev1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, &existingConfigMap); err != nil {
			if errors.IsNotFound(err) {
				// Create new ConfigMap
				if err := r.Create(ctx, configMap); err != nil {
					log.Error(err, "unable to create ConfigMap")
					return err
				}
				log.Info("created DKIM ConfigMap")
			} else {
				log.Error(err, "unable to get ConfigMap")
				return err
			}
		} else {
			// Update existing ConfigMap
			existingConfigMap.Data = configMap.Data
			if err := r.Update(ctx, &existingConfigMap); err != nil {
				log.Error(err, "unable to update ConfigMap")
				return err
			}
			log.Info("updated DKIM ConfigMap")
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DomainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mailcowv1.Domain{}).
		Named("domain").
		Complete(r)
}

func (r *DomainReconciler) setProgressing(ctx context.Context, domain *mailcowv1.Domain, message string) error {
	helpers.SetConditionStatus(&domain.Status.Conditions, "Progressing", "Reconciling", message, domain.Generation)
	domain.Status.Phase = "Progressing"
	return r.Status().Update(ctx, domain)
}

func (r *DomainReconciler) setReady(ctx context.Context, domain *mailcowv1.Domain, reason, message string) error {
	helpers.SetConditionStatus(&domain.Status.Conditions, "Ready", reason, message, domain.Generation)
	domain.Status.Phase = "Ready"
	return r.Status().Update(ctx, domain)
}

func (r *DomainReconciler) setDegraded(ctx context.Context, domain *mailcowv1.Domain, reason, message string) error {
	helpers.SetConditionStatus(&domain.Status.Conditions, "Degraded", reason, message, domain.Generation)
	domain.Status.Phase = "Degraded"
	return r.Status().Update(ctx, domain)
}
