package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/krateoplatformops/smithery/internal/dynamic"
	xcontext "github.com/krateoplatformops/snowplow/plumbing/context"
	"github.com/krateoplatformops/snowplow/plumbing/http/response"
	"github.com/krateoplatformops/snowplow/plumbing/maps"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func List() http.Handler {
	return &listHandler{}
}

const (
	groupVersion = "apiextensions.k8s.io/v1"
)

var _ http.Handler = (*listHandler)(nil)

type listHandler struct {
}

// @Summary List Endpoint
// @Description Returns information about Widgets API names
// @ID list
// @Produce  json
// @Success 200 {object} info
// @Failure 400 {object} response.Status
// @Failure 401 {object} response.Status
// @Failure 404 {object} response.Status
// @Failure 500 {object} response.Status
// @Router /list [get]
func (r *listHandler) ServeHTTP(wri http.ResponseWriter, req *http.Request) {

	log := xcontext.Logger(req.Context())

	cli, err := dynamic.NewClient(nil)
	if err != nil {
		log.Error("unable to create dynamic client", slog.Any("err", err))
		response.InternalError(wri, err)
		return
	}

	all, err := cli.List(req.Context(), dynamic.Options{
		GVR: schema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		},
	})
	if err != nil {
		log.Error("unable to list customresourcedefinitions", slog.Any("err", err))
		if apierrors.IsNotFound(err) {
			response.NotFound(wri, err)
		} else {
			response.InternalError(wri, err)
		}
		return
	}
	if len(all.Items) == 0 {
		log.Info("no customresourcedefinitions found")
		response.NotFound(wri, err)
	}

	result := make([]info, 0, len(all.Items))
	for _, el := range all.Items {
		names, ok, err := maps.NestedMapNoCopy(el.Object, "spec", "names")
		if err != nil {
			log.Warn("unable to fetch spec.names in CRD", slog.Any("err", err))
			continue
		}
		if !ok {
			log.Warn("spec.names not found in CRD")
			continue
		}

		gv := dynamic.GroupVersion(el.Object)

		plural, ok := names["plural"].(string)
		if !ok {
			log.Warn("spec.names.plural not found in CRD")
			continue
		}
		kind, ok := names["kind"].(string)
		if !ok {
			log.Warn("spec.names.kind not found in CRD")
			continue
		}
		result = append(result, info{
			Resource: plural,
			Kind:     kind,
			Group:    gv.Group,
			Version:  gv.Version,
		})
	}

	if len(result) == 0 {
		msg := "no widgets found"
		log.Warn(msg)
		response.NotFound(wri, fmt.Errorf("%s", msg))
		return
	}

	wri.Header().Set("Content-Type", "application/json")
	wri.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(wri)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&result); err != nil {
		log.Error("unable to serve api call response", slog.Any("err", err))
	}
}

type info struct {
	Resource string `json:"resource"`
	Kind     string `json:"kind"`
	Version  string `json:"version"`
	Group    string `json:"group"`
}
