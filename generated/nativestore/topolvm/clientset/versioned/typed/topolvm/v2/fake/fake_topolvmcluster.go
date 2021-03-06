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
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v2 "github.com/alauda/nativestor/apis/topolvm/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTopolvmClusters implements TopolvmClusterInterface
type FakeTopolvmClusters struct {
	Fake *FakeTopolvmV2
	ns   string
}

var topolvmclustersResource = schema.GroupVersionResource{Group: "topolvm.cybozu.com", Version: "v2", Resource: "topolvmclusters"}

var topolvmclustersKind = schema.GroupVersionKind{Group: "topolvm.cybozu.com", Version: "v2", Kind: "TopolvmCluster"}

// Get takes name of the topolvmCluster, and returns the corresponding topolvmCluster object, and an error if there is any.
func (c *FakeTopolvmClusters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2.TopolvmCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(topolvmclustersResource, c.ns, name), &v2.TopolvmCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2.TopolvmCluster), err
}

// List takes label and field selectors, and returns the list of TopolvmClusters that match those selectors.
func (c *FakeTopolvmClusters) List(ctx context.Context, opts v1.ListOptions) (result *v2.TopolvmClusterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(topolvmclustersResource, topolvmclustersKind, c.ns, opts), &v2.TopolvmClusterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2.TopolvmClusterList{ListMeta: obj.(*v2.TopolvmClusterList).ListMeta}
	for _, item := range obj.(*v2.TopolvmClusterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested topolvmClusters.
func (c *FakeTopolvmClusters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(topolvmclustersResource, c.ns, opts))

}

// Create takes the representation of a topolvmCluster and creates it.  Returns the server's representation of the topolvmCluster, and an error, if there is any.
func (c *FakeTopolvmClusters) Create(ctx context.Context, topolvmCluster *v2.TopolvmCluster, opts v1.CreateOptions) (result *v2.TopolvmCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(topolvmclustersResource, c.ns, topolvmCluster), &v2.TopolvmCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2.TopolvmCluster), err
}

// Update takes the representation of a topolvmCluster and updates it. Returns the server's representation of the topolvmCluster, and an error, if there is any.
func (c *FakeTopolvmClusters) Update(ctx context.Context, topolvmCluster *v2.TopolvmCluster, opts v1.UpdateOptions) (result *v2.TopolvmCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(topolvmclustersResource, c.ns, topolvmCluster), &v2.TopolvmCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2.TopolvmCluster), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeTopolvmClusters) UpdateStatus(ctx context.Context, topolvmCluster *v2.TopolvmCluster, opts v1.UpdateOptions) (*v2.TopolvmCluster, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(topolvmclustersResource, "status", c.ns, topolvmCluster), &v2.TopolvmCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2.TopolvmCluster), err
}

// Delete takes name of the topolvmCluster and deletes it. Returns an error if one occurs.
func (c *FakeTopolvmClusters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(topolvmclustersResource, c.ns, name), &v2.TopolvmCluster{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTopolvmClusters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(topolvmclustersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v2.TopolvmClusterList{})
	return err
}

// Patch applies the patch and returns the patched topolvmCluster.
func (c *FakeTopolvmClusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2.TopolvmCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(topolvmclustersResource, c.ns, name, pt, data, subresources...), &v2.TopolvmCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2.TopolvmCluster), err
}
