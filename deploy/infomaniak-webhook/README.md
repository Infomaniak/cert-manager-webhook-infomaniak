# Infomaniak ACME Webhook - Helm Chart

## Flux `HelmRelease`

If you are using Flux to manage your configuration, an `OCIRepository` can be used as the source for the `HelmRelease`:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: infomaniak-webhook
  namespace: cert-manager
spec:
  interval: 15m
  layerSelector:
    mediaType: application/vnd.cncf.helm.chart.content.v1.tar+gzip
    operation: copy
  ref:
    tag: 0.5.0 # Update version to what you need
  url: oci://ghcr.io/infomaniak/cert-manager-webhook-infomaniak/infomaniak-webhook
---
apiVersion: helm.toolkit.fluxcd.io/v2beta2
kind: HelmRelease
metadata:
  name: infomaniak-webhook
  namespace: cert-manager
spec:
  chartRef:
    kind: OCIRepository
    name: infomaniak-webhook
  interval: 30m
  values:
    groupName: example.com
    secretsNames:
      - infomaniak-api-credentials
```
