package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/krateoplatformops/crdgen"
	"github.com/krateoplatformops/krateoctl/jsonschema"
	xcontext "github.com/krateoplatformops/plumbing/context"
	"github.com/krateoplatformops/plumbing/http/response"
	"github.com/krateoplatformops/plumbing/kubeconfig"
	"github.com/krateoplatformops/smithery/internal/dynamic"
	"github.com/krateoplatformops/smithery/internal/handlers/util"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

// @Summary Generate a CRD from a JSON Schema
// @Description Generate a CRD from a JSON Schema
// @ID forge
// @Param apiVersion query string true "API Version"
// @Param resource query string true "Resource name"
// @Param apply query bool true "Apply Generated CRD"
// @Produce      plain
// @Success      200  {string}  string  "CRD YAML"
// @Router /forge [get]
// @Security Bearer
func Forge() http.Handler {
	return &forgeHandler{}
}

const (
	maxBodySize           = 100 * 1024
	widgetsGroup          = "widgets.templates.krateo.io"
	preserveUnknownFields = `{"type": "object", "additionalProperties": true,"x-kubernetes-preserve-unknown-fields": true}`
)

var _ http.Handler = (*forgeHandler)(nil)

type forgeHandler struct{}

func (r *forgeHandler) ServeHTTP(wri http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		response.MethodNotAllowed(wri,
			fmt.Errorf("method %q is not allowed, only POST is supported", req.Method))
		return
	}

	mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		response.NotAcceptable(wri, fmt.Errorf("invalid media type: %s", mediaType))
		return
	}

	apply, err := strconv.ParseBool(req.URL.Query().Get("apply"))
	if err != nil {
		apply = true
	}

	src := map[string]any{}
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&src); err != nil {
		if err == io.EOF {
			response.BadRequest(wri, fmt.Errorf("empty body"))
		} else {
			response.BadRequest(wri, err)
		}
		return
	}

	kind, version, err := jsonschema.ExtractKindAndVersion(src)
	if err != nil {
		response.BadRequest(wri, fmt.Errorf("unable to extract kind and version from JSON Schema: %w", err))
		return
	}

	allowedResources, err := jsonschema.ExtractAllowedResources(src)
	if err != nil {
		response.BadRequest(wri, fmt.Errorf("unable to extract allowedResources from JSON Schema: %w", err))
		return
	}

	spec, err := jsonschema.ExtractSpec(src)
	if err != nil {
		response.BadRequest(wri, fmt.Errorf("unable to extract spec from JSON Schema: %w", err))
		return
	}

	if len(allowedResources) > 0 {
		err = jsonschema.SetAllowedResources(spec, allowedResources)
		if err != nil {
			response.InternalError(wri, fmt.Errorf("unable to inject allowed resources into JSON Schema: %W", err))
			return
		}
	}

	dat, err := json.Marshal(spec)
	if err != nil {
		response.InternalError(wri, fmt.Errorf("unable to convert extracted spec to JSON: %w", err))
		return
	}

	opts := crdgen.Options{
		WorkDir: "widgets",
		GVK: runtimeschema.GroupVersionKind{
			Group:   widgetsGroup,
			Version: version,
			Kind:    kind,
		},
		Categories:             []string{"widgets", "krateo"},
		SpecJsonSchemaGetter:   fromBytes(dat),
		StatusJsonSchemaGetter: fromBytes([]byte(preserveUnknownFields)),
		Verbose:                false,
	}

	log := xcontext.Logger(req.Context()).
		With(
			slog.Group("widget",
				slog.String("kind", kind),
				slog.String("version", version),
			),
		)

	log.Info("generating CRD")

	start := time.Now()
	res := crdgen.Generate(req.Context(), opts)
	if res.Err != nil {
		log.Error("unable to generate CRD", slog.Any("err", res.Err))
		response.InternalError(wri, fmt.Errorf("unable to generate CRD: %w", err))
		return
	}

	log.Info("CRD successfully generated", slog.String("duration", util.ETA(start)))

	if apply {
		log.Info("applying CRD")
		start = time.Now()

		err := r.applyCRD(req.Context(), res.Manifest)
		if err != nil {
			response.Unauthorized(wri, err)
			return
		}

		log.Info("CRD successfully applied", slog.String("duration", util.ETA(start)))
	}

	wri.Header().Set("Content-Type", "application/yaml")
	wri.WriteHeader(http.StatusOK)
	wri.Write(res.Manifest)
}

func (r *forgeHandler) applyCRD(ctx context.Context, crd []byte) error {
	ep, err := xcontext.UserConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to get user endpoint: %w", err)
	}

	rc, err := kubeconfig.NewClientConfig(ctx, ep)
	if err != nil {
		return fmt.Errorf("unable to create kubernetes client config: %w", err)
	}

	dc, err := dynamic.NewClient(rc)
	if err != nil {
		return err
	}

	uns, err := dc.YAMLBytesToUnstructured(crd)
	if err != nil {
		return err
	}
	uns.SetAPIVersion("apiextensions.k8s.io/v1")
	uns.SetKind("CustomResourceDefinition")

	_, err = dc.Apply(ctx, uns, dynamic.Options{
		GVR: runtimeschema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		},
	})
	return err
}

/***************************************/
/* Custom crdgen.JsonSchemaGetter      */
/***************************************/
func fromBytes(data []byte) crdgen.JsonSchemaGetter {
	return &bytesJsonSchemaGetter{
		data: data,
	}
}

var _ crdgen.JsonSchemaGetter = (*bytesJsonSchemaGetter)(nil)

type bytesJsonSchemaGetter struct {
	data []byte
}

func (sg *bytesJsonSchemaGetter) Get() ([]byte, error) {
	return sg.data, nil
}
