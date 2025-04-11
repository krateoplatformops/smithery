package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/krateoplatformops/smithery/internal/crds"
	"github.com/krateoplatformops/smithery/internal/dynamic"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

func ValidateObjectStatus(ctx context.Context, rc *rest.Config, obj map[string]any) error {
	gv := dynamic.GroupVersion(obj)
	gvr, err := dynamic.ResourceFor(rc, gv.WithKind(dynamic.GetKind(obj)))
	if err != nil {
		return err
	}

	status, ok, err := unstructured.NestedMap(obj, "status")
	if err != nil {
		return err
	}
	if !ok {
		name := dynamic.GetName(obj)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusNotFound,
				Reason: metav1.StatusReasonNotFound,
				Details: &metav1.StatusDetails{
					Group: gvr.Group,
					Kind:  gvr.Resource,
					Name:  name,
				},
				Message: fmt.Sprintf("status not found in %s %q", gvr.String(), name),
			}}
	}

	doc, err := json.Marshal(status)
	if err != nil {
		return err
	}

	crd, err := crds.Get(ctx, crds.GetOptions{
		RC:      rc,
		Name:    fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group),
		Version: gvr.Version,
	})
	if err != nil {
		return err
	}

	schemaData, err := crds.OpenAPISchema(crd, gvr.Version)
	if err != nil {
		return err
	}

	crv, err := crds.OpenAPISchemaToCustomResourceValidation(schemaData)
	if err != nil {
		return err
	}

	return validateCustomResource(crv, doc)
}
