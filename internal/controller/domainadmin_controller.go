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

	"k8s.io/apimachinery/pkg/api/errors"
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

// DomainAdminReconciler reconciles a DomainAdmin object
type DomainAdminReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=domainadmins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=domainadmins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=domainadmins/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DomainAdmin object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *DomainAdminReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("namespace", req.NamespacedName)
	log.Info("reconciling domainadmin")

	var domainadmin mailcowv1.DomainAdmin
	if err := r.Get(ctx, req.NamespacedName, &domainadmin); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to find domainadmin")
		return ctrl.Result{}, err
	}

	// Apply finalizer
	if domainadmin.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&domainadmin, constants.Finalizer) {
			controllerutil.AddFinalizer(&domainadmin, constants.Finalizer)
			if err := r.Update(ctx, &domainadmin); err != nil {
				log.Error(err, "unable to update domainadmin with finalizer")
				return ctrl.Result{}, err
			}

			// Return and requeue to get fresh object
			return ctrl.Result{Requeue: true}, nil
		}
		// Set progressing status
		if changed, err := r.setProgressing(ctx, &domainadmin, "Reconciling domainadmin"); err != nil {
			log.Error(err, "unable to set progressing status")
			return ctrl.Result{}, err
		} else if changed {
			// Requeue to get fresh object with updated status
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if err := r.ReconcileResource(ctx, &domainadmin); err != nil {
		log.Error(err, "unable to reconcile mailcow domainadmin")
		// Set degraded status
		if _, errStatus := r.setDegraded(ctx, &domainadmin, err.Error()); errStatus != nil {
			log.Error(errStatus, "unable to set degraded status")
			return ctrl.Result{}, errStatus
		}
		return ctrl.Result{}, err
	}

	// Remove finalizer if deletion timestamp is set
	if !domainadmin.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(&domainadmin, constants.Finalizer) {
		controllerutil.RemoveFinalizer(&domainadmin, constants.Finalizer)
		if err := r.Update(ctx, &domainadmin); err != nil {
			log.Error(err, "unable to update domainadmin with finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Set ready status
	if _, err := r.setReady(ctx, &domainadmin, "DomainAdmin successfully reconciled"); err != nil {
		log.Error(err, "unable to set ready status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DomainAdminReconciler) ReconcileResource(ctx context.Context, domainadmin *mailcowv1.DomainAdmin) error {
	log := log.FromContext(ctx).WithValues("namespace", types.NamespacedName{Namespace: domainadmin.Namespace, Name: domainadmin.Name})
	var err error

	// Get related mailcow resource
	var res mailcowv1.Mailcow
	if err := r.Get(ctx, types.NamespacedName{Name: domainadmin.Spec.Mailcow, Namespace: domainadmin.Namespace}, &res); err != nil {
		log.Error(err, "unable to find related mailcow resource", "mailcow", domainadmin.Spec.Mailcow)
		return err
	}

	// Create mailcow client
	client, err := res.GetClient(ctx, r)
	if err != nil {
		log.Error(err, "unable to create mailcow client")
		return err
	}

	// Get all domain admins to check if this user exists
	// When mailcow has no domain admins, this returns an empty object, not an empty array. That why we don't use the WithResponse function here and Unmarshall it ourselves.
	response, err := client.GetDomainAdmins(ctx)
	if err != nil {
		log.Error(err, "unable to get domainadmins")
		return err
	}

	if response.StatusCode != 200 {
		log.Error(err, "unable to get domainadmins, invalid status code", "statusCode", response.StatusCode)
		return err
	}

	// Unmarshall response
	var parsedResponse *mailcow.GetDomainAdminsResponse
	parsedResponse, _ = mailcow.ParseGetDomainAdminsResponse(response)
	// Ignore unmarshall errors, as mailcow returns an empty object when there are no domain admins

	// Find if the domain admin already exists
	var domainAdminExists bool
	if parsedResponse != nil {
		for _, da := range *parsedResponse.JSON200 {
			if da.Username != nil && *da.Username == domainadmin.Spec.Username {
				domainAdminExists = true
				break
			}
		}
	}

	if !domainadmin.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion
		if domainAdminExists {
			_, err = client.DeleteDomainAdminWithResponse(ctx, mailcow.DeleteDomainAdminJSONRequestBody{domainadmin.Spec.Username})
			if err != nil {
				log.Error(err, "unable to delete domainadmin")
				return err
			}
		}
		return nil
	}

	// Get password from secret
	password, err := domainadmin.GetPassword(ctx, r)
	if err != nil {
		log.Error(err, "unable to get password from secret")
		return err
	}

	if !domainAdminExists {
		// DomainAdmin does not exist, create it
		_, err = client.CreateDomainAdminUserWithResponse(ctx, mailcow.CreateDomainAdminUserJSONRequestBody{
			Username:  &domainadmin.Spec.Username,
			Password:  &password,
			Password2: &password,
			Active:    helpers.BooleanToInt(domainadmin.Spec.Active),
			Domains:   &domainadmin.Spec.Domains,
		})

		if err != nil {
			log.Error(err, "unable to create domainadmin")
			return err
		}
	} else {
		// DomainAdmin exists, update it
		_, err = client.EditDomainAdminUserWithResponse(ctx, mailcow.EditDomainAdminUserJSONRequestBody{
			Attr: &mailcow.EditDomainAdminAttr{
				// Password: &password, // Should we always update the password? assuming someone might want to change it
				Active:  domainadmin.Spec.Active,
				Domains: &domainadmin.Spec.Domains,
			},
			Items: &[]string{domainadmin.Spec.Username},
		})

		if err != nil {
			log.Error(err, "unable to update domainadmin")
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DomainAdminReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mailcowv1.DomainAdmin{}).
		Named("domainadmin").
		Complete(r)
}

func (r *DomainAdminReconciler) setProgressing(ctx context.Context, domainadmin *mailcowv1.DomainAdmin, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&domainadmin.Status.Conditions, "Progressing", "Reconciling", message, domainadmin.Generation)
	if !changed {
		return changed, nil
	}
	domainadmin.Status.Phase = "Progressing"
	return changed, r.Status().Update(ctx, domainadmin)
}

func (r *DomainAdminReconciler) setReady(ctx context.Context, domainadmin *mailcowv1.DomainAdmin, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&domainadmin.Status.Conditions, "Ready", "Reconciled", message, domainadmin.Generation)
	if !changed {
		return changed, nil
	}
	domainadmin.Status.Phase = "Ready"
	return changed, r.Status().Update(ctx, domainadmin)
}

func (r *DomainAdminReconciler) setDegraded(ctx context.Context, domainadmin *mailcowv1.DomainAdmin, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&domainadmin.Status.Conditions, "Degraded", "ReconcileFailed", message, domainadmin.Generation)
	if !changed {
		return changed, nil
	}
	domainadmin.Status.Phase = "Degraded"
	return changed, r.Status().Update(ctx, domainadmin)
}
