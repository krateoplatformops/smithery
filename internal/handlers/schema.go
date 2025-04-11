package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/krateoplatformops/smithery/internal/crds"
	"github.com/krateoplatformops/smithery/internal/handlers/util"
	xcontext "github.com/krateoplatformops/snowplow/plumbing/context"
	"github.com/krateoplatformops/snowplow/plumbing/http/response"
	"k8s.io/apimachinery/pkg/api/errors"
)

// @Summary Fetch CRD OpenAPI Schema
// @Description CRD OpenAPI Schema
// @ID schema
// @Produce  json
// @Param apiVersion query string true "API Version"
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

	gvr, err := util.ParseGVR(req)
	if err != nil {
		response.BadRequest(wri, err)
		return
	}
	log.Debug("fetching custom resource definition",
		slog.String("resource", gvr.GroupResource().String()),
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
