# mailcow-operator

The mailcow API is generated from the [mailcow OpenAPI specification](mailcow/openapi.yaml) using [oapi-codegen](https://github.com/deepmap/oapi-codegen).
The openapi.yaml is pulled from a mailcow version 2024-01d release. And then edited due to the specification not being fully correct and missing schemas for some endpoints.

To regenerate the API code, run:
``` bash
oapi-codegen --config=mailcow/oapi-codegen.yaml mailcow/openapi.yaml
```

## Roadmap

- Update status conditions
- Add more resources
- Add e2e tests
- Allow multiple goto addresses in aliases
- Make the created DKIM ConfigMap name configurable