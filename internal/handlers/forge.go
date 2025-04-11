package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/krateoplatformops/crdgen"
	"github.com/krateoplatformops/smithery/internal/dynamic"
	"github.com/krateoplatformops/smithery/internal/handlers/util/jsonschema"
	xcontext "github.com/krateoplatformops/snowplow/plumbing/context"
	"github.com/krateoplatformops/snowplow/plumbing/http/response"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

// @Summary Generate a CRD from a JSON Schema
// @Description Generate a CRD from a JSON Schema
// @ID forge
// @Produce  yaml
// @Param apiVersion query string true "API Version"
// @Param resource query string true "Resource name"
// @Success 200 {object} object
// @Router /forge [get]
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

	log := xcontext.Logger(req.Context())

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
		log.Error("unable to extract kind and version from JSON Schema", slog.Any("err", err))
		response.BadRequest(wri, err)
		return
	}

	log.Debug("extracted Kind and Version from JSON Schema",
		slog.String("kind", kind),
		slog.String("version", version),
	)

	spec, err := jsonschema.ExtractSpec(src)
	if err != nil {
		log.Error("unable to extract spec from JSON Schema", slog.Any("err", err))
		response.BadRequest(wri, err)
		return
	}

	dat, err := json.Marshal(spec)
	if err != nil {
		log.Error("unable to convert extracted spec to JSON", slog.Any("err", err))
		response.InternalError(wri, err)
		return
	}

	opts := crdgen.Options{
		WorkDir: "widgets",
		GVK: runtimeschema.GroupVersionKind{
			Group:   widgetsGroup,
			Version: version,
			Kind:    kind,
		},
		SpecJsonSchemaGetter:   fromBytes(dat),
		StatusJsonSchemaGetter: fromBytes([]byte(preserveUnknownFields)),
		Verbose:                false,
	}

	res := crdgen.Generate(req.Context(), opts)
	if res.Err != nil {
		log.Error("unable to generate CRD", slog.Any("err", err))
		response.InternalError(wri, err)
		return
	}

	dc, err := dynamic.NewClient(nil)
	if err != nil {
		log.Error("unable to create dynamic client", slog.Any("err", err))
		response.InternalError(wri, err)
		return
	}

	uns, err := dc.YAMLBytesToUnstructured(res.Manifest)
	if err != nil {
		log.Error("unable to convert CRD data to YAML", slog.Any("err", err))
		response.InternalError(wri, err)
		return
	}
	uns.SetAPIVersion("apiextensions.k8s.io/v1")
	uns.SetKind("CustomResourceDefinition")

	_, err = dc.Apply(req.Context(), uns, dynamic.Options{
		GVR: runtimeschema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		},
	})
	if err != nil {
		log.Error("unable to apply the generated CRD", slog.Any("err", err))
		response.InternalError(wri, err)
		return
	}

	response.Encode(wri, response.New(http.StatusAccepted, nil))
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
