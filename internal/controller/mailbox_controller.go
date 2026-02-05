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

// MailboxReconciler reconciles a Mailbox object
type MailboxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=mailboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=mailboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mailcow.onestein.nl,resources=mailboxes/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Mailbox object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *MailboxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("namespace", req.NamespacedName)
	log.Info("reconciling mailbox")

	var mailbox mailcowv1.Mailbox
	if err := r.Get(ctx, req.NamespacedName, &mailbox); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to find mailbox")
		return ctrl.Result{}, err
	}

	// Apply finalizer
	if mailbox.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&mailbox, constants.Finalizer) {
			controllerutil.AddFinalizer(&mailbox, constants.Finalizer)
			if err := r.Update(ctx, &mailbox); err != nil {
				log.Error(err, "unable to update mailbox with finalizer")
				return ctrl.Result{}, err
			}

			// Return and requeue to get fresh object
			return ctrl.Result{Requeue: true}, nil
		}
		// Set progressing status
		if changed, err := r.setProgressing(ctx, &mailbox, "Reconciling mailbox"); err != nil {
			log.Error(err, "unable to set progressing status")
			return ctrl.Result{}, err
		} else if changed {
			// Requeue to get fresh object with updated status
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if err := r.ReconcileResource(ctx, &mailbox); err != nil {
		log.Error(err, "unable to reconcile mailcow mailbox")
		// Set degraded status
		if _, errStatus := r.setDegraded(ctx, &mailbox, "ReconcileFailed", err.Error()); errStatus != nil {
			log.Error(errStatus, "unable to set degraded status")
			return ctrl.Result{}, errStatus
		}
		return ctrl.Result{}, err
	}

	// Remove finalizer if deletion timestamp is set
	if !mailbox.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(&mailbox, constants.Finalizer) {
		controllerutil.RemoveFinalizer(&mailbox, constants.Finalizer)
		if err := r.Update(ctx, &mailbox); err != nil {
			log.Error(err, "unable to update mailbox with finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Set ready status
	if _, err := r.setReady(ctx, &mailbox, "Mailbox successfully reconciled"); err != nil {
		log.Error(err, "unable to set ready status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MailboxReconciler) ReconcileResource(ctx context.Context, mailbox *mailcowv1.Mailbox) error {
	log := log.FromContext(ctx).WithValues("namespace", types.NamespacedName{Namespace: mailbox.Namespace, Name: mailbox.Name})
	var err error

	// Get related mailcow resource
	var res mailcowv1.Mailcow
	if err := r.Get(ctx, types.NamespacedName{Name: mailbox.Spec.Mailcow, Namespace: mailbox.Namespace}, &res); err != nil {
		log.Error(err, "unable to find related mailcow resource", "mailcow", mailbox.Spec.Mailcow)
		return err
	}

	// Create mailcow client
	client, err := res.GetClient(ctx, r)
	if err != nil {
		log.Error(err, "unable to create mailcow client")
		return err
	}

	// Construct the full email address
	email := mailbox.Spec.LocalPart + "@" + mailbox.Spec.Domain

	response, err := client.GetMailboxesWithResponse(ctx, mailcow.GetMailboxesParamsId(email), nil)
	if err != nil {
		log.Error(err, "unable to get mailbox")
		return err
	}

	if !mailbox.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion
		if response.JSON200.Username != nil {
			_, err = client.DeleteMailboxWithResponse(ctx, mailcow.DeleteMailboxJSONRequestBody{email})
			if err != nil {
				log.Error(err, "unable to delete mailbox")
				return err
			}
		}
		return nil
	}

	// Get password from secret
	password, err := mailbox.GetPassword(ctx, r)
	if err != nil {
		log.Error(err, "unable to get password from secret")
		return err
	}

	if response.JSON200.Username == nil {
		// Mailbox does not exist, create it
		_, err = client.CreateMailboxWithResponse(ctx, mailcow.CreateMailboxJSONRequestBody{
			Domain:        &mailbox.Spec.Domain,
			LocalPart:     &mailbox.Spec.LocalPart,
			Name:          &mailbox.Spec.Name,
			Password:      &password,
			Password2:     &password,
			Active:        mailbox.Spec.Active,
			ForcePwUpdate: mailbox.Spec.ForcePasswordChange,
			Quota:         helpers.Int64ToFloat32(mailbox.Spec.Quota),
		})

		if err != nil {
			log.Error(err, "unable to create mailbox")
			return err
		}
	} else {
		// Mailbox exists, update it
		_, err = client.UpdateMailboxWithResponse(ctx, mailcow.UpdateMailboxJSONRequestBody{
			Attr: &mailcow.EditMailboxAttr{
				// Name: &mailbox.Spec.Name,
				// Password:      &password, // Should we always update the password? assuming someone might want to change it
				// Password2:     &password,
				Active: mailbox.Spec.Active,
				// ForcePwUpdate: mailbox.Spec.ForcePasswordChange,
				Quota: helpers.Int64ToFloat32(mailbox.Spec.Quota),
			},
			Items: &[]string{email},
		})

		if err != nil {
			log.Error(err, "unable to update mailbox")
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MailboxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mailcowv1.Mailbox{}).
		Named("mailbox").
		Complete(r)
}

func (r *MailboxReconciler) setProgressing(ctx context.Context, mailbox *mailcowv1.Mailbox, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&mailbox.Status.Conditions, "Progressing", "Reconciling", message, mailbox.Generation)
	if !changed {
		return changed, nil
	}
	mailbox.Status.Phase = "Progressing"
	return changed, r.Status().Update(ctx, mailbox)
}

func (r *MailboxReconciler) setReady(ctx context.Context, mailbox *mailcowv1.Mailbox, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&mailbox.Status.Conditions, "Ready", "Reconciled", message, mailbox.Generation)
	if !changed {
		return changed, nil
	}
	mailbox.Status.Phase = "Ready"
	return changed, r.Status().Update(ctx, mailbox)
}

func (r *MailboxReconciler) setDegraded(ctx context.Context, mailbox *mailcowv1.Mailbox, reason, message string) (bool, error) {
	changed := helpers.SetConditionStatus(&mailbox.Status.Conditions, "Degraded", reason, message, mailbox.Generation)
	if !changed {
		return changed, nil
	}
	mailbox.Status.Phase = "Degraded"
	return changed, r.Status().Update(ctx, mailbox)
}
