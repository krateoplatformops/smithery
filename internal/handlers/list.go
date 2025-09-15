package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	xcontext "github.com/krateoplatformops/plumbing/context"
	"github.com/krateoplatformops/plumbing/http/response"
	"github.com/krateoplatformops/plumbing/kubeconfig"
	"github.com/krateoplatformops/plumbing/maps"
	"github.com/krateoplatformops/smithery/internal/dynamic"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func List() http.Handler {
	return &listHandler{}
}

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
// @Security Bearer
func (r *listHandler) ServeHTTP(wri http.ResponseWriter, req *http.Request) {
	log := xcontext.Logger(req.Context())

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

	cli, err := dynamic.NewClient(rc)
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

		group := ""
		if val, ok := maps.NestedValue(el.Object, []string{"spec", "group"}); ok {
			group = val.(string)
		}

		vers, _, err := unstructured.NestedSlice(el.Object, "spec", "versions")
		if err != nil {
			log.Error("unable to fetch spec.versions in CRD")
			continue
		}
		versions := make([]string, 0, len(vers))
		for _, vv := range vers {
			ver, _, err := unstructured.NestedString(vv.(map[string]any), "name")
			if err == nil {
				versions = append(versions, ver)
			}
		}

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
			Group:    group,
			Versions: versions,
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
	Resource string   `json:"resource"`
	Kind     string   `json:"kind"`
	Versions []string `json:"versions"`
	Group    string   `json:"group"`
}
