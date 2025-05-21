package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	xcontext "github.com/krateoplatformops/plumbing/context"
	"github.com/krateoplatformops/plumbing/http/response"
	"github.com/krateoplatformops/plumbing/kubeconfig"
	"github.com/krateoplatformops/smithery/internal/crds"
	"github.com/krateoplatformops/smithery/internal/handlers/util"
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
// @Security Bearer
func Schema() http.Handler {
	return &schemaHandler{}
}

var _ http.Handler = (*schemaHandler)(nil)

type schemaHandler struct{}

func (r *schemaHandler) ServeHTTP(wri http.ResponseWriter, req *http.Request) {
	gvr, err := parseGVR(req)
	if err != nil {
		response.BadRequest(wri, err)
		return
	}

	log := xcontext.Logger(req.Context()).
		With(
			slog.Group("resource",
				slog.String("name", gvr.Resource),
				slog.String("group", gvr.Group),
				slog.String("version", gvr.Version),
			),
		)

	start := time.Now()

	ep, err := xcontext.UserConfig(req.Context())
	if err != nil {
		log.Error("unable to get user endpoint", slog.Any("err", err))
		response.Unauthorized(wri, err)
		return
	}

	rc, err := kubeconfig.NewClientConfig(req.Context(), ep)
	if err != nil {
		log.Error("unable to create kubernetes client config", slog.Any("err", err))
		response.InternalError(wri, err)
		return
	}

	log.Debug("fetching custom resource definition")

	crd, err := crds.Get(req.Context(), crds.GetOptions{
		RC:      rc,
		Name:    gvr.GroupResource().String(),
		Version: gvr.Version,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(wri, err)
		} else {
			response.InternalError(wri, err)
		}
		return
	}

	log.Debug("fetching openapi schema")

	crv, err := crds.OpenAPISchema(crd, gvr.Version)
	if err != nil {
		response.InternalError(wri, err)
		return
	}

	log.Info("openapi schema successfully fetched", slog.String("duration", util.ETA(start)))

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
