package crds

import (
	"fmt"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func OpenAPISchema(crd map[string]any, version string) (map[string]any, error) {
	versions, found, err := unstructured.NestedSlice(crd, "spec", "versions")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no versions found in CRD")
	}

	for _, v := range versions {
		versionMap, ok := v.(map[string]any)
		if !ok {
			continue
		}

		if name, found := versionMap["name"].(string); found && name == version {
			schemaData, exists, err := unstructured.NestedMap(versionMap, "schema", "openAPIV3Schema")
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("schema OpenAPI v3 not found for version: %s", version)
			}

			return schemaData, nil
		}
	}

	return nil, fmt.Errorf("version [%s] not found in CRD schema", version)
}

func OpenAPISchemaToCustomResourceValidation(schemaData map[string]any) (*apiextensions.CustomResourceValidation, error) {
	schemaProps := &apiextensions.JSONSchemaProps{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(schemaData, schemaProps)
	if err != nil {
		return nil, err
	}

	return &apiextensions.CustomResourceValidation{
		OpenAPIV3Schema: schemaProps,
	}, nil
}
