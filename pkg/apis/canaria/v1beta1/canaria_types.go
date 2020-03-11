package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	K8sDeployStageCanary = iota
	K8sDeployStageRollBack
	K8sDeployStageRollup
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// Canaria represents canaria for k8s deployment.
type Canaria struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired identities of pods in this cronhpa.
	Spec CanariaSpec `json:"spec,omitempty"`

	// Status is the current status of pods in this CronHPA. This data
	// may be out of date by some window of time.
	Status CanariaStatus `json:"status,omitempty"`
}

// A CanariaSpec is the specification of a Canaria.
type CanariaSpec struct {
	TargetSize int32             `json:"targetSize" protobuf:"bytes,1,opt,name=targetSize"`
	Images     map[string]string `json:"images" protobuf:"bytes,2,opt,name=images"`
	Stage      int32             `json:"stage" protobuf:"bytes,2,opt,name=stage"`
}

// CanariaStatus represents the current state of a Canaria.
type CanariaStatus struct {
	// Information when was the last time the schedule was successfully scheduled.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,1,opt,name=lastUpdateTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CanariaList is a collection of Canaria.
type CanariaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Canaria `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Canaria{}, &CanariaList{})
}
