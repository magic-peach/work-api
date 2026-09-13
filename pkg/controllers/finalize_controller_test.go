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
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

func newFinalizeReconciler(t *testing.T, objs ...runtime.Object) *FinalizeWorkReconciler {
	scheme := runtime.NewScheme()
	if err := workv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return &FinalizeWorkReconciler{
		client: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build(),
	}
}

func TestFinalizeWorkReconcileMissingWork(t *testing.T) {
	r := newFinalizeReconciler(t)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeWorkReconcileAddsFinalizer(t *testing.T) {
	work := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{Name: "w", Namespace: "ns"}}
	r := newFinalizeReconciler(t, work)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "w", Namespace: "ns"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got workv1alpha1.Work
	if err := r.client.Get(context.Background(), types.NamespacedName{Name: "w", Namespace: "ns"}, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Finalizers) != 1 || got.Finalizers[0] != workFinalizer {
		t.Errorf("Finalizers = %v, want [%s]", got.Finalizers, workFinalizer)
	}
}

func TestFinalizeWorkReconcileRemovesFinalizerOnDeletion(t *testing.T) {
	now := metav1.NewTime(time.Now())
	work := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
		Name: "w", Namespace: "ns",
		Finalizers:        []string{workFinalizer},
		DeletionTimestamp: &now,
	}}
	r := newFinalizeReconciler(t, work)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "w", Namespace: "ns"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got workv1alpha1.Work
	err := r.client.Get(context.Background(), types.NamespacedName{Name: "w", Namespace: "ns"}, &got)
	if !apierrors.IsNotFound(err) {
		t.Errorf("Get() error = %v, want NotFound (removing the last finalizer lets the apiserver delete the object)", err)
	}
}
