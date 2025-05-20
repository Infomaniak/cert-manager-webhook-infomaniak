# buildifier: disable=module-docstring
load("ext://podman", "podman_build")
load('ext://helm_resource', 'helm_resource', 'helm_repo')
load('ext://namespace', 'namespace_create')

if "Podman" in str(local("docker version", quiet = True)):
    docker_build = podman_build

# setup env
debug_enabled = os.getenv("DEBUG", "false").lower()
debug_port = os.getenv("DEBUG_PORT", "30000")

infomaniak_token = os.getenv("INFOMANIAK_TOKEN")
if infomaniak_token == None:
    fail("INFOMANIAK_TOKEN env variable should be set")

acme_email = os.getenv("EMAIL")
if acme_email == None:
    fail("EMAIL env variable should be set")

test_zone = os.getenv("TEST_ZONE")
if test_zone == None:
    fail("TEST_ZONE env variable should be set")

# setup cert-manager
update_settings(k8s_upsert_timeout_secs=300)
helm_repo("jetstack", "https://charts.jetstack.io", resource_name="jetstack-repo", labels = ["cert-manager"])
helm_resource(
    "cert-manager", "jetstack/cert-manager",
    namespace="cert-manager",
    flags = [
        "--create-namespace",
        "--set",
        "crds.enabled=true"
    ],
    labels = ["cert-manager"]
)

# build webhook
tiltbuild_path = ".tiltbuild"
webhook_bin = tiltbuild_path + "/webhook" 
local_resource(
    "dockerfile",
    cmd = "mkdir -p {tiltbuild_path};cp Tilt.Dockerfile {tiltbuild_path}/Dockerfile".format(
        tiltbuild_path = shlex.quote(tiltbuild_path),
    ),
    deps = ["Tilt.Dockerfile"],
    labels = ["cert-manager-infomaniak"]
)
local_resource(
    "webhook-binary",
    cmd = 'CGO_ENABLED=0 go build -o {webhook_bin} -gcflags "all=-N -l" .'.format(
        webhook_bin = webhook_bin,
    ),
    deps = [
        "main.go",
        "main_test.go",
        "infomaniak_api.go",
        "go.mod",
        "go.sum",
    ],
    resource_deps = ["dockerfile"],
    labels = ["cert-manager-infomaniak"]
)
docker_build(
    "cert-manager-webhook-infomaniak",
    tiltbuild_path,
    deps = [webhook_bin],
    live_update = [
        sync(webhook_bin, "/usr/local/bin/webhook"),
        run("sh /restart.sh"),
    ],
)

# deploy webhook
namespace_create("cert-manager-infomaniak")
webhook_yaml = helm(
                    "./deploy/infomaniak-webhook",
                    name="infomaniak-webhook",
                    namespace="cert-manager-infomaniak",
                    values = ["./deploy/infomaniak-webhook/values.yaml"],
                    set = [
                        "image.repository=cert-manager-webhook-infomaniak",
                        "debug.enabled=" + debug_enabled
                    ]
)
k8s_yaml(webhook_yaml)

port_forwards = []
if debug_enabled == "true":
  port_forwards = [port_forward(int(debug_port), int(debug_port))]
k8s_resource(
    workload = "infomaniak-webhook",
    port_forwards = port_forwards,
    objects = [
      "infomaniak-webhook:serviceaccount", "infomaniak-webhook\\:secret-reader:role", "infomaniak-webhook\\:domain-solver:clusterrole",
      "infomaniak-webhook\\:webhook-authentication-reader:rolebinding", "infomaniak-webhook\\:secret-reader:rolebinding",
      "infomaniak-webhook\\:auth-delegator:clusterrolebinding", "infomaniak-webhook\\:domain-solver:clusterrolebinding",
      "infomaniak-webhook-ca:certificate", "infomaniak-webhook-webhook-tls:certificate", "infomaniak-webhook-selfsign:issuer",
      "infomaniak-webhook-ca:issuer", "cert-manager-infomaniak:namespace", "v1alpha1.acme.infomaniak.com:apiservice"
    ],
    resource_deps = ["cert-manager", "webhook-binary"],
    labels = ["cert-manager-infomaniak"]
)

# deploy additional test resources
test_objects = read_yaml_stream("tilt/manifests.yaml")
for o in test_objects:
    if o.get("kind") == "Secret" and "stringData" in o and "api-token" in o["stringData"]:
        o["stringData"]["api-token"] = infomaniak_token
    if o.get("kind") == "ClusterIssuer" and "acme" in o["spec"] and "email" in o["spec"]["acme"]:
        o["spec"]["acme"]["email"] = acme_email
    if o.get("kind") == "Certificate" and "dnsNames" in o["spec"]:
        o["spec"]["dnsNames"] = [test_zone]
k8s_yaml(encode_yaml_stream(test_objects))
k8s_resource(
    new_name = "test-manifests",
    objects = ["infomaniak-api-credentials:secret", "letsencrypt-staging:clusterissuer", "test-certificate:certificate"],
    resource_deps = ["cert-manager"],
    labels = ["tests"]
)
