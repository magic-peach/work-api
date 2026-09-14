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
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	workv1alpha1 "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

func TestFindManifestConditionByIdentifier(t *testing.T) {
	full := workv1alpha1.ResourceIdentifier{Ordinal: 1, Group: "", Version: "v1", Kind: "ConfigMap", Resource: "configmaps", Namespace: "ns", Name: "a"}
	conditions := []workv1alpha1.ManifestCondition{
		{Identifier: full},
		{Identifier: workv1alpha1.ResourceIdentifier{Ordinal: 2, Version: "v1", Kind: "ConfigMap", Resource: "configmaps", Namespace: "ns", Name: "b"}},
	}

	if got := findManifestConditionByIdentifier(full, conditions); got == nil || got.Identifier != full {
		t.Errorf("exact match: got %v, want identifier %v", got, full)
	}

	ordinalOnly := workv1alpha1.ResourceIdentifier{Ordinal: 3}
	if got := findManifestConditionByIdentifier(ordinalOnly, conditions); got != nil {
		t.Errorf("ordinal only with no match: got %v, want nil", got)
	}

	byOtherFields := workv1alpha1.ResourceIdentifier{Ordinal: 99, Version: "v1", Kind: "ConfigMap", Resource: "configmaps", Namespace: "ns", Name: "b"}
	if got := findManifestConditionByIdentifier(byOtherFields, conditions); got == nil || got.Identifier.Name != "b" {
		t.Errorf("match by non ordinal fields: got %v, want identifier for b", got)
	}

	noMatch := workv1alpha1.ResourceIdentifier{Ordinal: 5, Version: "v1", Kind: "Secret", Resource: "secrets", Namespace: "ns", Name: "c"}
	if got := findManifestConditionByIdentifier(noMatch, conditions); got != nil {
		t.Errorf("no match: got %v, want nil", got)
	}
}

func TestFindObservedGenerationOfManifest(t *testing.T) {
	identifier := workv1alpha1.ResourceIdentifier{Ordinal: 1, Version: "v1", Kind: "ConfigMap", Resource: "configmaps", Namespace: "ns", Name: "a"}

	if got := findObservedGenerationOfManifest(identifier, nil); got != 0 {
		t.Errorf("no matching condition: got %d, want 0", got)
	}

	noApplied := []workv1alpha1.ManifestCondition{
		{Identifier: identifier, Conditions: []metav1.Condition{{Type: "Other", Status: metav1.ConditionTrue}}},
	}
	if got := findObservedGenerationOfManifest(identifier, noApplied); got != 0 {
		t.Errorf("no applied condition: got %d, want 0", got)
	}

	withApplied := []workv1alpha1.ManifestCondition{
		{Identifier: identifier, Conditions: []metav1.Condition{{Type: "Applied", Status: metav1.ConditionTrue, ObservedGeneration: 7}}},
	}
	if got := findObservedGenerationOfManifest(identifier, withApplied); got != 7 {
		t.Errorf("applied condition present: got %d, want 7", got)
	}
}

func TestSetSpecHashAnnotation(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.Object["data"] = map[string]interface{}{"key": "value"}
	obj.Object["status"] = map[string]interface{}{"ignored": "true"}

	if err := setSpecHashAnnotation(obj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	first := obj.GetAnnotations()[specHashAnnotation]
	if first == "" {
		t.Fatal("expected a non empty spec hash annotation")
	}

	obj.Object["status"] = map[string]interface{}{"ignored": "false"}
	obj.SetAnnotations(map[string]string{"unrelated": "kept"})
	if err := setSpecHashAnnotation(obj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second := obj.GetAnnotations()[specHashAnnotation]
	if second != first {
		t.Errorf("status change altered the hash: got %q, want %q", second, first)
	}
	if obj.GetAnnotations()["unrelated"] != "kept" {
		t.Error("expected pre existing annotations to be preserved")
	}

	obj.Object["data"] = map[string]interface{}{"key": "changed"}
	if err := setSpecHashAnnotation(obj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.GetAnnotations()[specHashAnnotation] == first {
		t.Error("expected data change to alter the hash")
	}
}

func TestBuildResourceIdentifier(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName("web")
	obj.SetNamespace("ns")
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	got := buildResourceIdentifier(2, obj, gvr)
	want := workv1alpha1.ResourceIdentifier{
		Ordinal:   2,
		Group:     "apps",
		Version:   "v1",
		Kind:      "Deployment",
		Resource:  "deployments",
		Namespace: "ns",
		Name:      "web",
	}
	if got != want {
		t.Errorf("buildResourceIdentifier() = %+v, want %+v", got, want)
	}
}

func TestBuildAppliedStatusCondition(t *testing.T) {
	failed := buildAppliedStatusCondition(errors.New("boom"), 0)
	if failed.Status != metav1.ConditionFalse || failed.Reason != "AppliedManifestFailed" {
		t.Errorf("error case: got %+v, want False/AppliedManifestFailed", failed)
	}

	ok := buildAppliedStatusCondition(nil, 4)
	if ok.Status != metav1.ConditionTrue || ok.Reason != "AppliedManifestComplete" || ok.ObservedGeneration != 4 {
		t.Errorf("success case: got %+v, want True/AppliedManifestComplete/ObservedGeneration 4", ok)
	}
}

func TestGenerateWorkAppliedStatusCondition(t *testing.T) {
	failing := []workv1alpha1.ManifestCondition{
		{Conditions: []metav1.Condition{{Type: "Applied", Status: metav1.ConditionFalse}}},
	}
	got := generateWorkAppliedStatusCondition(failing, 3)
	if got.Status != metav1.ConditionFalse || got.Reason != "AppliedWorkFailed" {
		t.Errorf("failing manifest: got %+v, want False/AppliedWorkFailed", got)
	}

	passing := []workv1alpha1.ManifestCondition{
		{Conditions: []metav1.Condition{{Type: "Applied", Status: metav1.ConditionTrue}}},
	}
	got = generateWorkAppliedStatusCondition(passing, 3)
	if got.Status != metav1.ConditionTrue || got.Reason != "AppliedWorkComplete" {
		t.Errorf("passing manifests: got %+v, want True/AppliedWorkComplete", got)
	}
}
