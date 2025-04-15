## Generate Widget CRD

```sh 
curl -v --request POST \
  -H 'Content-Type: application/json' \
  -d @testdata/widgets.templates.krateo.io_buttons.json \
  "http://127.0.0.1:30081/forge?apply=true"
```

## List all Widgets 

```sh 
curl -v --request GET "http://127.0.0.1:30081/list"
```

```json
[
  {
    "resource": "buttons",
    "kind": "Button",
    "versions": [
      "v1beta1"
    ],
    "group": "widgets.templates.krateo.io"
  }
]
```

## Fetch OpenAPI Schema

```sh 
curl -v -G GET -d 'version=v1beta1' -d 'resource=buttons' "http://127.0.0.1:30081/schema"
```


