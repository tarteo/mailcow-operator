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

// AliasReconciler reconciles a Alias object
type AliasReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=aliases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=aliases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=aliases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Alias object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *AliasReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("namespace", req.NamespacedName)
	log.Info("reconciling alias")

	var alias mailcowv1.Alias
	if err := r.Get(ctx, req.NamespacedName, &alias); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to find alias")
		return ctrl.Result{}, err
	}

	// Apply finalizer
	if alias.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&alias, constants.Finalizer) {
			controllerutil.AddFinalizer(&alias, constants.Finalizer)
			if err := r.Update(ctx, &alias); err != nil {
				log.Error(err, "unable to update alias with finalizer")
				return ctrl.Result{}, err
			}

			// Return and requeue to get fresh object
			return ctrl.Result{Requeue: true}, nil
		}
		// Set progressing status
		if changed, err := r.setProgressing(ctx, &alias, "Reconciling alias"); err != nil {
			log.Error(err, "unable to set progressing status")
			return ctrl.Result{}, err
		} else if changed {
			// Requeue to get fresh object with updated status
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Reconcile the resource
	if err := r.ReconcileResource(ctx, &alias); err != nil {
		log.Error(err, "unable to reconcile mailcow alias")
		// Set degraded status
		if _, errStatus := r.setDegraded(ctx, &alias, err.Error()); errStatus != nil {
			log.Error(errStatus, "unable to set degraded status")
			return ctrl.Result{}, errStatus
		}
		return ctrl.Result{}, err
	}

	// Remove finalizer if deletion timestamp is set
	if !alias.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(&alias, constants.Finalizer) {
		controllerutil.RemoveFinalizer(&alias, constants.Finalizer)
		if err := r.Update(ctx, &alias); err != nil {
			log.Error(err, "unable to update alias with finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Set ready status
	if _, err := r.setReady(ctx, &alias, "Alias successfully reconciled"); err != nil {
		log.Error(err, "unable to set ready status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AliasReconciler) ReconcileResource(ctx context.Context, alias *mailcowv1.Alias) error {
	log := log.FromContext(ctx).WithValues("namespace", types.NamespacedName{Namespace: alias.Namespace, Name: alias.Name})
	var err error

	// Get related mailcow resource
	var res mailcowv1.Mailcow
	if err := r.Get(ctx, types.NamespacedName{Name: alias.Spec.Mailcow, Namespace: alias.Namespace}, &res); err != nil {
		log.Error(err, "unable to find related mailcow resource", "mailcow", alias.Spec.Mailcow)
		return err
	}

	// Reconcile mailcow alias
	client, err := res.GetClient(ctx, r)
	if err != nil {
		log.Error(err, "unable to create mailcow client")
		return err
	}

	// Get alias to check if this address exists
	// When mailcow has no aliases for this address, this returns an empty object, not an empty array. That's why we don't use the WithResponse function here and unmarshall it ourselves.
	response, err := client.GetAliasesWithResponse(ctx, mailcow.GetAliasesParamsId(alias.Spec.Address), nil)
	if err != nil {
		log.Error(err, "unable to get alias")
		return err
	}

	// Find if the alias already exists
	var aliasExists bool = response.JSON200 != nil

	if !alias.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion
		if aliasExists {
			_, err = client.DeleteAliasWithResponse(ctx, mailcow.DeleteAliasJSONRequestBody{alias.Spec.Address})
			if err != nil {
				log.Error(err, "unable to delete alias")
				return err
			}
		}
		return nil
	}

	if !aliasExists {
		// Alias does not exist, create it
		_, err = client.CreateAliasWithResponse(ctx, mailcow.CreateAliasJSONRequestBody{
			Address: &alias.Spec.Address,
			Goto:    &alias.Spec.GoTo,
			Active:  &alias.Spec.Active,
		})

		if err != nil {
			log.Error(err, "unable to create alias")
			return err
		}
	} else {
		// Alias exists, update it
		_, err = client.UpdateAliasWithResponse(ctx, mailcow.UpdateAliasJSONRequestBody{
			Attr: &mailcow.EditAliasAttr{
				Goto:   &alias.Spec.GoTo,
				Active: &alias.Spec.Active,
			},
			Items: &[]string{alias.Spec.Address},
		})

		if err != nil {
			log.Error(err, "unable to update alias")
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AliasReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mailcowv1.Alias{}).
		Named("alias").
		Complete(r)
}

func (r *AliasReconciler) setProgressing(ctx context.Context, alias *mailcowv1.Alias, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&alias.Status.Conditions, constants.ConditionProgressing, "Reconciling", message, alias.Generation)
	if !changed {
		return changed, nil
	}
	alias.Status.Phase = constants.ConditionProgressing
	return changed, r.Status().Update(ctx, alias)
}

func (r *AliasReconciler) setReady(ctx context.Context, alias *mailcowv1.Alias, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&alias.Status.Conditions, constants.ConditionReady, "Reconciled", message, alias.Generation)
	if !changed {
		return changed, nil
	}
	alias.Status.Phase = constants.ConditionReady
	return changed, r.Status().Update(ctx, alias)
}

func (r *AliasReconciler) setDegraded(ctx context.Context, alias *mailcowv1.Alias, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&alias.Status.Conditions, constants.ConditionDegraded, "ReconcileFailed", message, alias.Generation)
	if !changed {
		return changed, nil
	}
	alias.Status.Phase = constants.ConditionDegraded
	return changed, r.Status().Update(ctx, alias)
}
