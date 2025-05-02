package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	xcontext "github.com/krateoplatformops/plumbing/context"
	"github.com/krateoplatformops/plumbing/http/response"
	"github.com/krateoplatformops/smithery/internal/crds"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// @Summary Fetch CRD OpenAPI Schema
// @Description CRD OpenAPI Schema
// @ID schema
// @Produce  json
// @Param version query string true "API Version"
// @Param resource query string true "Resource name"
// @Success 200 {object} object
// @Router /schema [get]
func Schema() http.Handler {
	return &schemaHandler{}
}

var _ http.Handler = (*schemaHandler)(nil)

type schemaHandler struct{}

func (r *schemaHandler) ServeHTTP(wri http.ResponseWriter, req *http.Request) {
	log := xcontext.Logger(req.Context())

	gvr, err := parseGVR(req)
	if err != nil {
		response.BadRequest(wri, err)
		return
	}
	log.Debug("fetching custom resource definition",
		slog.String("resource", gvr.Resource),
		slog.String("version", gvr.Version),
	)

	crd, err := crds.Get(req.Context(), crds.GetOptions{
		Name: gvr.GroupResource().String(), Version: gvr.Version,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(wri, err)
		} else {
			response.InternalError(wri, err)
		}
		return
	}

	log.Debug("fetching openapi schema",
		slog.String("resource", gvr.GroupResource().String()),
		slog.String("version", gvr.Version),
	)

	crv, err := crds.OpenAPISchema(crd, gvr.Version)
	if err != nil {
		response.InternalError(wri, err)
		return
	}

	wri.Header().Set("Content-Type", "application/json")
	wri.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(wri)
	enc.SetIndent("", "  ")
	if err := enc.Encode(crv); err != nil {
		log.Error("unable to serve openapi schema for CRD",
			slog.String("resource", gvr.GroupResource().String()),
			slog.String("version", gvr.Version),
			slog.Any("err", err))
	}
}

func parseGVR(req *http.Request) (gvr schema.GroupVersionResource, err error) {
	ver := req.URL.Query().Get("version")
	if len(ver) == 0 {
		err = fmt.Errorf("missing 'version' query parameter")
		return
	}
	api := fmt.Sprintf("%s/%s", widgetsGroup, ver)

	res := req.URL.Query().Get("resource")
	if len(res) == 0 {
		err = fmt.Errorf("missing 'resource' query parameter")
		return
	}

	gv, err := schema.ParseGroupVersion(api)
	if err != nil {
		return gvr, err
	}

	gvr = gv.WithResource(res)
	return
}
