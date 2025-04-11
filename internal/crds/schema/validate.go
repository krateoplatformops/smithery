package schema

import (
	"encoding/json"
	"errors"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
)

func validateCustomResource(crv *apiextensions.CustomResourceValidation, document []byte) error {
	var jsonObj map[string]any
	if err := json.Unmarshal(document, &jsonObj); err != nil {
		return err
	}

	validator, _, err := validation.NewSchemaValidator(crv.OpenAPIV3Schema)
	if err != nil {
		return err
	}

	errs := validation.ValidateCustomResource(nil, jsonObj, validator)
	if len(errs) == 0 {
		return nil
	}

	return errors.New(errs.ToAggregate().Error())
}
