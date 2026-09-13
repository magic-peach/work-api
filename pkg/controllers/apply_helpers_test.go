/*
Copyright 2021 The Kubernetes Authors.

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

package controllers

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newUnstructured(name, namespace string, labels, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetLabels(labels)
	obj.SetAnnotations(annotations)
	return obj
}

func TestIsSameUnstructuredMeta(t *testing.T) {
	base := newUnstructured("cm", "ns", map[string]string{"a": "b"}, map[string]string{"c": "d"})

	cases := map[string]struct {
		other *unstructured.Unstructured
		want  bool
	}{
		"identical":             {other: newUnstructured("cm", "ns", map[string]string{"a": "b"}, map[string]string{"c": "d"}), want: true},
		"different name":        {other: newUnstructured("other", "ns", map[string]string{"a": "b"}, map[string]string{"c": "d"}), want: false},
		"different namespace":   {other: newUnstructured("cm", "other", map[string]string{"a": "b"}, map[string]string{"c": "d"}), want: false},
		"different labels":      {other: newUnstructured("cm", "ns", map[string]string{"a": "z"}, map[string]string{"c": "d"}), want: false},
		"different annotations": {other: newUnstructured("cm", "ns", map[string]string{"a": "b"}, map[string]string{"c": "z"}), want: false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isSameUnstructuredMeta(base, c.other); got != c.want {
				t.Errorf("isSameUnstructuredMeta() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsManifestModified(t *testing.T) {
	required := newUnstructured("cm", "ns", nil, nil)
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

	existing := newUnstructured("cm", "ns", nil, nil)
	existing.SetGeneration(3)
	if isManifestModified(3, gvr, existing, required) {
		t.Error("expected no modification when meta matches and generation is observed")
	}

	existing.SetGeneration(4)
	if !isManifestModified(3, gvr, existing, required) {
		t.Error("expected modification when generation has advanced past what was observed")
	}

	existing.SetGeneration(3)
	existing.SetLabels(map[string]string{"changed": "true"})
	if !isManifestModified(3, gvr, existing, required) {
		t.Error("expected modification when metadata differs")
	}
}
