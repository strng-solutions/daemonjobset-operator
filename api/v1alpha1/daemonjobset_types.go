/*
Copyright 2021.

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

package v1alpha1

import (
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type DaemonJobSetPlacement struct {
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
}

type CronJobTemplateSpec struct {
	// Standard object's metadata of the cron jobs created from this template.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the cron job.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec batchv1beta1.CronJobSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// DaemonJobSetSpec defines the desired state of DaemonJobSet
type DaemonJobSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// +optional
	Placement DaemonJobSetPlacement `json:"placement,omitempty"`

	CronJobTemplate CronJobTemplateSpec `json:"cronJobTemplate"`
}

// DaemonJobSetStatus defines the observed state of DaemonJobSet
type DaemonJobSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// A list of pointers to currently running jobs.
	// +optional
	Enabled []v1.ObjectReference `json:"enabled,omitempty" protobuf:"bytes,1,rep,name=enabled"`

	// Information when was the last time the job was successfully scheduled.
	// +optional
	//LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty" protobuf:"bytes,4,opt,name=lastScheduleTime"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DaemonJobSet is the Schema for the daemonjobsets API
type DaemonJobSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DaemonJobSetSpec   `json:"spec,omitempty"`
	Status DaemonJobSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DaemonJobSetList contains a list of DaemonJobSet
type DaemonJobSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DaemonJobSet `json:"items"`
}

func (in *DaemonJobSet) GetNodeLabelSelector() (labels.Selector, error) {
	nodeSelector := labels.NewSelector()

	if len(in.Spec.Placement.NodeSelector) == 0 {
		nodeSelector = labels.Everything()
	}

	for nodeLabelName, nodeLabelValue := range in.Spec.Placement.NodeSelector {
		nodeRequirement, err := labels.NewRequirement(nodeLabelName, selection.Equals, []string{nodeLabelValue})
		if err != nil {
			return nil, err
		}

		nodeSelector = nodeSelector.Add(*nodeRequirement)
	}

	return nodeSelector, nil
}

func init() {
	SchemeBuilder.Register(&DaemonJobSet{}, &DaemonJobSetList{})
}
