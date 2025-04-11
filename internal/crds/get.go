package crds

import (
	"context"
	"fmt"

	"github.com/krateoplatformops/smithery/internal/dynamic"
	"github.com/krateoplatformops/snowplow/plumbing/env"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type GetOptions struct {
	RC      *rest.Config
	Name    string
	Version string
}

func Get(ctx context.Context, opts GetOptions) (map[string]any, error) {
	if opts.RC == nil {
		if env.TestMode() {
			return map[string]any{}, fmt.Errorf("with 'test mode' enabled rest.Config cannot be nil")
		}

		var err error
		opts.RC, err = rest.InClusterConfig()
		if err != nil {
			return map[string]any{}, err
		}
	}

	cli, err := dynamic.NewClient(opts.RC)
	if err != nil {
		return map[string]any{}, err
	}

	got, err := cli.Get(ctx, opts.Name, dynamic.Options{
		GVR: runtimeschema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		},
		Namespace: "",
	})
	if err != nil {
		return map[string]any{}, err
	}
	if got != nil {
		return got.UnstructuredContent(), nil
	}

	return map[string]any{}, err
}
