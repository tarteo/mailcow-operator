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

package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DomainAdminSpec defines the desired state of DomainAdmin.
type DomainAdminSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Mailcow string `json:"mailcow"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Username is immutable"
	Username       string                   `json:"username"`
	PasswordSecret corev1.SecretKeySelector `json:"passwordSecret"`

	// +kubebuilder:default:=true
	Active *bool `json:"active,omitempty"`

	Domains []string `json:"domains"`
}

// DomainAdminStatus defines the observed state of DomainAdmin.
type DomainAdminStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DomainAdmin is the Schema for the domainadmins API.
type DomainAdmin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainAdminSpec   `json:"spec,omitempty"`
	Status DomainAdminStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DomainAdminList contains a list of DomainAdmin.
type DomainAdminList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DomainAdmin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DomainAdmin{}, &DomainAdminList{})
}

func (domainadmin *DomainAdmin) GetPassword(ctx context.Context, r client.Reader) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: domainadmin.Spec.PasswordSecret.Name, Namespace: domainadmin.Namespace}, &secret); err != nil {
		return "", err
	}

	value, ok := secret.Data[domainadmin.Spec.PasswordSecret.Key]
	if !ok {
		return "", fmt.Errorf("key `%s` not found in secret `%s`", domainadmin.Spec.PasswordSecret.Key, secret.Name)
	}

	return string(value), nil
}
