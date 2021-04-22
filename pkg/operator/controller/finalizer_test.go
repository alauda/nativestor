/*
Copyright 2020 The Rook Authors. All rights reserved.

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
	topolvmv1 "github.com/alauda/topolvm-operator/api/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestAddFinalizerIfNotPresent(t *testing.T) {
	fakeObject := &topolvmv1.TopolvmCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "topolvm-system",
			Finalizers: []string{},
		},
	}

	// Objects to track in the fake client.
	object := []client.Object{
		fakeObject,
	}
	s := runtime.NewScheme()
	s.AddKnownTypes(topolvmv1.GroupVersion, fakeObject)
	cl := fake.NewClientBuilder().WithObjects(object...).WithScheme(s).Build()
	assert.Empty(t, fakeObject.Finalizers)
	err := AddFinalizerIfNotPresent(cl, fakeObject)
	assert.NoError(t, err)
	assert.NotEmpty(t, fakeObject.Finalizers)
}

func TestRemoveFinalizer(t *testing.T) {
	fakeObject := &topolvmv1.TopolvmCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "rook-ceph",
			Finalizers: []string{
				"topolvmcluster.topolvm.cybozu.com",
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "topolvmcluster",
		},
	}

	object := []client.Object{
		fakeObject,
	}
	s := runtime.NewScheme()
	s.AddKnownTypes(topolvmv1.GroupVersion, fakeObject)
	cl := fake.NewClientBuilder().WithObjects(object...).WithScheme(s).Build()
	assert.NotEmpty(t, fakeObject.Finalizers)
	err := RemoveFinalizer(cl, fakeObject)
	assert.NoError(t, err)
	assert.Empty(t, fakeObject.Finalizers)
}
