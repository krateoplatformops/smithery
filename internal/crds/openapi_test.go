package crds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractOpenAPISchemaFromCRD(t *testing.T) {
	validCRD := map[string]any{
		"spec": map[string]any{
			"versions": []any{
				map[string]any{
					"name": "v1",
					"schema": map[string]any{
						"openAPIV3Schema": map[string]any{
							"type": "object",
						},
					},
				},
			},
		},
	}

	t.Run("valid schema extraction", func(t *testing.T) {
		schemaData, err := OpenAPISchema(validCRD, "v1")
		assert.NoError(t, err)
		result, err := OpenAPISchemaToCustomResourceValidation(schemaData)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "object", result.OpenAPIV3Schema.Type)
	})

	t.Run("missing version in CRD", func(t *testing.T) {
		_, err := OpenAPISchema(validCRD, "v2")
		assert.Error(t, err)
		assert.Equal(t, "version [v2] not found in CRD schema", err.Error())
	})

	t.Run("missing versions key", func(t *testing.T) {
		invalidCRD := map[string]any{
			"spec": map[string]any{},
		}
		_, err := OpenAPISchema(invalidCRD, "v1")
		assert.Error(t, err)
		assert.Equal(t, "no versions found in CRD", err.Error())
	})

	t.Run("invalid schema format", func(t *testing.T) {
		invalidSchemaCRD := map[string]any{
			"spec": map[string]any{
				"versions": []any{
					map[string]any{
						"name":   "v1",
						"schema": "invalid-format",
					},
				},
			},
		}
		_, err := OpenAPISchema(invalidSchemaCRD, "v1")
		assert.Error(t, err)
	})
}
