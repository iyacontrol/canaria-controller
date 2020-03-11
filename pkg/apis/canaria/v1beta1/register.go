// NOTE: Boilerplate only.  Ignore this file.

// Package v1beta1 contains API Schema definitions for the confighpas v1beta1 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/iyacontrol/canaria-controller/pkg/apis/canaria
// +k8s:defaulter-gen=TypeMeta
// +groupName=canaria.shareit.com
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: "canaria.shareit.com", Version: "v1beta1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)
