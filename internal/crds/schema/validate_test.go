package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

func TestValidate(t *testing.T) {

	validSchema := &apiextensions.CustomResourceValidation{
		OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
			Type: "object",
			Properties: map[string]apiextensions.JSONSchemaProps{
				"name": {Type: "string"},
				"age":  {Type: "integer"},
			},
		},
	}

	validDocument := []byte(`{"name": "John", "age": 30}`)
	invalidDocument := []byte(`{"name": "John", "age": "thirty"}`)
	malformedDocument := []byte(`{"name": "John", "age": 30`) // JSON non valido

	t.Run("valid document", func(t *testing.T) {
		err := validateCustomResource(validSchema, validDocument)
		assert.NoError(t, err)
	})

	t.Run("invalid document type mismatch", func(t *testing.T) {
		err := validateCustomResource(validSchema, invalidDocument)
		assert.Error(t, err)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		err := validateCustomResource(validSchema, malformedDocument)
		assert.Error(t, err)
	})
}
