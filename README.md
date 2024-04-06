# Infomaniak ACME webhook

A cert-manager webhook which works with domains handled by [Infomaniak](https://www.infomaniak.com), a 🇨🇭 Swiss hosting provider

## Quick start

1. Deploy cert-manager (if needed)
    ```
    $ kubectl apply --validate=false -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.4/cert-manager.yaml
    ```

1. Deploy Infomaniak webhook
    ```
    $ kubectl apply -f https://github.com/infomaniak/cert-manager-webhook-infomaniak/releases/download/v0.2.0/rendered-manifest.yaml
    ```

1. Create a Secret with your Infomaniak API token. You can generate a new one by [clicking here](https://manager.infomaniak.com/v3/infomaniak-api). You need to have at least the `Domain` scope.
    ```
    $ cat <<EOF | kubectl apply -f -
    ---
    apiVersion: v1
    kind: Secret
    metadata:
      name: infomaniak-api-credentials
      namespace: cert-manager
    type: Opaque
    data:
      api-token: $(echo -n $INFOMANIAK_TOKEN|base64 -w0)
    EOF
    ```

1. Create a Secret with your staging ACME private key (if you don't have one, the next step will generate one)

    ```
    $ cat <<EOF | kubectl apply -f -
    ---
    apiVersion: v1
    kind: Secret
    metadata:
      name: le-staging-account-key
      namespace: cert-manager
    type: Opaque
    data:
      tls.key: <<YOUR_KEY_BASE64>>
    EOF
    ```


1. Create a staging ClusterIssuer
    ```
    $ cat <<EOF | kubectl apply -f -
    ---
    apiVersion: cert-manager.io/v1
    kind: ClusterIssuer
    metadata:
      name: letsencrypt-staging
    spec:
      acme:
        email: acme@example.com
        privateKeySecretRef:
          name: le-staging-account-key
        server: https://acme-staging-v02.api.letsencrypt.org/directory
        solvers:
        - selector: {}
          dns01:
            webhook:
              groupName: acme.infomaniak.com
              solverName: infomaniak
              config:
                apiTokenSecretRef:
                  name: infomaniak-api-credentials
                  key: api-token
    EOF
    ```

1. Create a Certificate, the issued cert will be stored in the specified Secret (keys tls.crt & tls.key)
    ```
    $ cat <<EOF | kubectl apply -f -
    ---
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: test-example-com
    spec:
      secretName: test-example-com-tls
      issuerRef:
        name: letsencrypt-staging
        kind: ClusterIssuer
      dnsNames:
      - test.example.com
    EOF

    $ kubectl get secret test-example-com-tls -o json | jq -r '.data."tls.crt"' | base64 -d | openssl x509 -text -noout | grep Subject:
        Subject: CN = test.example.com
    ```

1. If everything worked as expected using letsencrypt staging environment, repeat the 3 last steps using letsencrypt's prod environment
    - ClusterIssuer `.spec.acme.server`: `https://acme-v02.api.letsencrypt.org/directory`
    - ClusterIssuer `.spec.acme.email`: your "prod" e-mail address
    - ClusterIssuer `.spec.acme.privateKeySecretRef`: you can leave it blank and a new one will be generated for you

1. Have fun 🎉 !

## Building

Run `make build`

## Running the test suite

All DNS providers **must** run the DNS01 provider conformance testing suite,
else they will have undetermined behaviour when used with cert-manager.

You can run the test suite by exporting your API token in `INFOMANIAK_TOKEN`, then by running `TEST_ZONE_NAME=example.com. make test`.
