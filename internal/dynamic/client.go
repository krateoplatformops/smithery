package dynamic

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/krateoplatformops/snowplow/plumbing/env"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

func NewClient(rc *rest.Config) (*UnstructuredClient, error) {
	if rc == nil {
		if env.TestMode() {
			return nil, fmt.Errorf("when test mode is on rest.Config must be specified")
		}

		var err error
		rc, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	dynamicClient, err := dynamic.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(rc)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(
		cacheddiscovery.NewMemCacheClient(discoveryClient),
	)

	return &UnstructuredClient{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		mapper:          mapper,
		converter:       runtime.DefaultUnstructuredConverter,
	}, nil
}

type Options struct {
	Namespace string
	GVK       schema.GroupVersionKind
	GVR       schema.GroupVersionResource
}

type UnstructuredClient struct {
	dynamicClient   *dynamic.DynamicClient
	discoveryClient discovery.DiscoveryInterface
	mapper          *restmapper.DeferredDiscoveryRESTMapper
	converter       runtime.UnstructuredConverter
}

func (uc *UnstructuredClient) Get(ctx context.Context, name string, opts Options) (*unstructured.Unstructured, error) {
	ri, err := uc.resourceInterfaceFor(opts)
	if err != nil {
		return nil, err
	}

	return ri.Get(ctx, name, metav1.GetOptions{})
}

func (uc *UnstructuredClient) List(ctx context.Context, opts Options) (*unstructured.UnstructuredList, error) {
	ri, err := uc.resourceInterfaceFor(opts)
	if err != nil {
		return nil, err
	}

	return ri.List(ctx, metav1.ListOptions{})
}

func (uc *UnstructuredClient) Delete(ctx context.Context, name string, opts Options) error {
	ri, err := uc.resourceInterfaceFor(opts)
	if err != nil {
		return err
	}

	return ri.Delete(ctx, name, metav1.DeleteOptions{})
}

func (uc *UnstructuredClient) Create(ctx context.Context, obj *unstructured.Unstructured, opts Options) (*unstructured.Unstructured, error) {
	ri, err := uc.resourceInterfaceFor(opts)
	if err != nil {
		return nil, err
	}

	return ri.Create(ctx, obj, metav1.CreateOptions{})
}

func (uc *UnstructuredClient) Update(ctx context.Context, obj *unstructured.Unstructured, opts Options) (*unstructured.Unstructured, error) {
	ri, err := uc.resourceInterfaceFor(opts)
	if err != nil {
		return nil, err
	}

	return ri.Update(ctx, obj, metav1.UpdateOptions{})
}

func (uc *UnstructuredClient) Apply(ctx context.Context, obj *unstructured.Unstructured, opts Options) (*unstructured.Unstructured, error) {
	name, found, err := unstructured.NestedString(obj.Object, "metadata", "name")
	if err != nil {
		return nil, fmt.Errorf("failed to extract name from object: %w", err)
	}
	if !found || name == "" {
		return nil, fmt.Errorf("object has no name")
	}

	existing, err := uc.Get(ctx, name, opts)
	if err != nil {
		if errors.IsNotFound(err) {
			return uc.Create(ctx, obj, opts)
		}
		return nil, fmt.Errorf("failed to get existing resource: %w", err)
	}

	resourceVersion, found, err := unstructured.NestedString(existing.Object, "metadata", "resourceVersion")
	if err != nil || !found {
		return nil, fmt.Errorf("failed to get resourceVersion: %w", err)
	}

	if err := unstructured.SetNestedField(obj.Object, resourceVersion, "metadata", "resourceVersion"); err != nil {
		return nil, fmt.Errorf("failed to set resourceVersion: %w", err)
	}

	return uc.Update(ctx, obj, opts)
}

func (uc *UnstructuredClient) ApplyFromYAML(ctx context.Context, yamlBytes []byte, opts Options) (*unstructured.Unstructured, error) {
	obj, err := uc.YAMLBytesToUnstructured(yamlBytes)
	if err != nil {
		return nil, err
	}

	return uc.Apply(ctx, obj, opts)
}

func (uc *UnstructuredClient) Convert(in map[string]any, out any) error {
	return uc.converter.FromUnstructured(in, out)
}

func (uc *UnstructuredClient) Discover(ctx context.Context, category string) (all []schema.GroupVersionResource, err error) {
	lists, err := uc.discoveryClient.ServerPreferredResources()
	if err != nil {
		return
	}

	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}

		for _, el := range list.APIResources {
			if !found(el, category) {
				continue
			}

			all = append(all, schema.GroupVersionResource{
				Group:    el.Group,
				Version:  el.Version,
				Resource: el.Name,
			})
		}
	}

	return
}

func (uc *UnstructuredClient) YAMLBytesToUnstructured(yamlBytes []byte) (*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBytes), 4096)

	obj := map[string]any{}
	if err := decoder.Decode(&obj); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return &unstructured.Unstructured{Object: obj}, nil
}

func (uc *UnstructuredClient) resourceInterfaceFor(opts Options) (dynamic.ResourceInterface, error) {
	if opts.GVK.Empty() && !opts.GVR.Empty() {
		gvk, err := uc.mapper.KindFor(opts.GVR)
		if err != nil {
			return nil, err
		}
		opts.GVK = gvk
	}

	restMapping, err := uc.mapper.RESTMapping(opts.GVK.GroupKind(), opts.GVK.Version)
	if err != nil {
		return nil, err
	}

	var ri dynamic.ResourceInterface
	if len(opts.Namespace) == 0 {
		ri = uc.dynamicClient.Resource(restMapping.Resource)
	} else {
		ri = uc.dynamicClient.Resource(restMapping.Resource).
			Namespace(opts.Namespace)
	}
	return ri, nil
}

func found(el metav1.APIResource, str string) bool {
	if strings.EqualFold(el.Name, str) {
		return true
	}

	if strings.EqualFold(el.SingularName, str) {
		return true
	}

	if contains(el.ShortNames, str) {
		return true
	}

	return contains(el.Categories, str)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
