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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DomainSpec defines the desired state of Domain.
type DomainSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Mailcow string `json:"mailcow"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Domain is immutable"
	Domain       string `json:"domain"`
	Description  string `json:"description,omitempty"`
	Quota        int64  `json:"quota"`
	MaxQuota     int64  `json:"maxQuota"`
	DefQuota     int64  `json:"defQuota"`
	MaxMailboxes int64  `json:"maxMailboxes"`
	RateLimit    *int   `json:"rateLimit,omitempty"`
	// +kubebuilder:validation:Enum:=h;s;m;d
	// +kubebuilder:default:=h
	RateLimitFrame string `json:"rateLimitFrame,omitempty"`

	// +kubebuilder:default:=true
	Active *bool `json:"active,omitempty"`
}

// DomainStatus defines the observed state of Domain.
type DomainStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Domain is the Schema for the domains API.
type Domain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainSpec   `json:"spec,omitempty"`
	Status DomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DomainList contains a list of Domain.
type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Domain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Domain{}, &DomainList{})
}
