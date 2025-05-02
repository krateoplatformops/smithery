# `smithery`

[![Go Report Card](https://goreportcard.com/badge/github.com/krateoplatformops/smithery)](https://goreportcard.com/report/github.com/krateoplatformops/smithery)



Smithery is a core web service used by the [Krateo Platformops](https://krateo.io/) to dynamically generate declarative user interfaces. 

It serves two main purposes:

1. **From JSON Schema to Kubernetes CR**  
   Given a custom JSON Schema defined by frontend developers to describe a typed UI component, Smithery generates a corresponding Kubernetes Custom Resource (CR).  
   It enriches this resource with advanced features such as:
   - Executing REST API calls,
   - Parsing JSON responses using an internal JQ engine,
   - Resolving placeholders dynamically before rendering the final UI component.

2. **OpenAPI Schema Generation**  
   Smithery also returns the OpenAPI schema for the generated Custom Resource, based on its kind and version, enabling the frontend to understand the CR structure for validation and rendering.

