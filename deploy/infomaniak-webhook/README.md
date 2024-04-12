# Infomaniak ACME Webhook - Helm Chart

## Flux Helm Release

If you are using Flux to manage your configuration, a `GitRepository` can be used as a source for the `HelmRelease`.
This also allows for customization of the chart values.

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: infomaniak-webhook
  namespace: cert-manager
spec:
  interval: 5m
  url: https://github.com/Infomaniak/cert-manager-webhook-infomaniak
  ref:
    branch: master
  ignore: |
    /*
    !/deploy/
---
apiVersion: helm.toolkit.fluxcd.io/v2beta2
kind: HelmRelease
metadata:
  name: infomaniak-webhook
  namespace: cert-manager
spec:
  interval: 5m
  chart:
    spec:
      chart: deploy/infomaniak-webhook
      sourceRef:
        kind: GitRepository
        name: infomaniak-webhook
  values:
    groupName: example.com
    secretsNames:
      - infomaniak-api-credentials
```
