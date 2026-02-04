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

	"github.com/tarteo/mailcow-operator/mailcow"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MailcowSpec defines the desired state of Mailcow.
type MailcowSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Secret   corev1.SecretKeySelector `json:"secret"`
	Endpoint string                   `json:"endpoint"`
}

// MailcowStatus defines the observed state of Mailcow.
type MailcowStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// +kubebuilder:validation:Enum=Progressing;Ready;Degraded
	Phase string `json:"phase,omitempty"`
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Mailcow is the Schema for the mailcows API.
type Mailcow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MailcowSpec   `json:"spec,omitempty"`
	Status MailcowStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MailcowList contains a list of Mailcow.
type MailcowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mailcow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mailcow{}, &MailcowList{})
}

func (res *Mailcow) GetClient(ctx context.Context, r client.Reader) (*mailcow.ClientWithResponses, error) {

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: res.Spec.Secret.Name, Namespace: res.Namespace}, &secret); err != nil {
		return nil, err
	}
	value, ok := secret.Data[res.Spec.Secret.Key]
	if !ok {
		return nil, fmt.Errorf("key `%s` not found in secret `%s`", res.Spec.Secret.Key, secret.Name)
	}
	apiKey := string(value)
	endpoint := res.Spec.Endpoint

	client, err := mailcow.NewCustomClientWithResponses(endpoint, apiKey)
	if err != nil {
		return nil, err
	}
	return client, nil
}
