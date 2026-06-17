# Offline `.run` Package

`aisphere-auth` supports building a self-extracting offline `.run` package. The package contains:

- container image tar files
- `images/image-index.tsv`
- Kubernetes manifest templates
- an embedded installer

The install side can load images, retag them to any internal registry, push them, render manifests and apply them to Kubernetes.

## Build locally

```bash
bash build.sh --arch amd64
bash build.sh --arch arm64
bash build.sh --arch all
```

Outputs:

```text
dist/aisphere-auth-<version>-amd64.run
dist/aisphere-auth-<version>-amd64.run.sha256
dist/aisphere-auth-<version>-arm64.run
dist/aisphere-auth-<version>-arm64.run.sha256
```

## Build in GitHub Actions

Workflow:

```text
.github/workflows/offline-run.yml
```

Manual dispatch supports:

```text
arch = all | amd64 | arm64
```

Tag pushes matching `v*` build both architectures and upload `.run` / `.sha256` files to GitHub Release.

## Image manifest

The build process reads:

```text
images/image.json
```

Current image entries build from the repository Dockerfile:

```json
{
  "name": "aisphere-auth",
  "arch": "amd64",
  "platform": "linux/amd64",
  "dockerfile": "deployments/docker/Dockerfile",
  "context": ".",
  "tag": "sealos.hub:5000/aisphere/aisphere-auth:${VERSION}-amd64",
  "tar": "aisphere-auth-amd64.tar"
}
```

`build.sh` replaces `${VERSION}` and `${ARCH}` before building.

## Install into an offline environment

Default target registry is `sealos.hub:5000`:

```bash
chmod +x aisphere-auth-0.1.0-amd64.run
./aisphere-auth-0.1.0-amd64.run install -y
```

Use a different registry:

```bash
./aisphere-auth-0.1.0-amd64.run install -y \
  --registry 10.10.10.10:5000 \
  --namespace aisphere-system
```

Harbor example:

```bash
./aisphere-auth-0.1.0-amd64.run install -y \
  --registry harbor.local:5000 \
  --namespace aisphere-system
```

The installer retags the packaged image from:

```text
sealos.hub:5000/aisphere/aisphere-auth:<version>-<arch>
```

to:

```text
<registry>/aisphere/aisphere-auth:<version>-<arch>
```

Then it pushes the image and renders Kubernetes manifests with that target image.

## Render only

```bash
./aisphere-auth-0.1.0-amd64.run install -y \
  --registry sealos.hub:5000 \
  --skip-push \
  --skip-apply \
  --output-dir ./out
```

Rendered manifest:

```text
./out/rendered/aisphere-auth.yaml
```

## Runtime configuration

Common install flags:

```bash
./aisphere-auth-0.1.0-amd64.run install -y \
  --registry sealos.hub:5000 \
  --namespace aisphere-system \
  --public-base-url http://aisphere-auth.example.com \
  --casdoor-endpoint http://casdoor.example.com \
  --casdoor-owner skillhub \
  --casdoor-application aisphere \
  --casdoor-client-id '<client-id>' \
  --casdoor-client-secret '<client-secret>' \
  --casdoor-permission-id skillhub/platform_permission \
  --session-provider redis \
  --redis-addrs redis.redis.svc.cluster.local:6379
```

If `--service-token` or `--jwt-secret` is omitted, the installer generates a random value.

## Commands

```bash
./aisphere-auth-0.1.0-amd64.run install -h
./aisphere-auth-0.1.0-amd64.run extract --output-dir ./payload
```

## Notes

- Docker is required on the install host for image load, retag and push.
- `kubectl` is required unless `--skip-apply` is used.
- The target registry must be reachable and writable from the install host.
- Registry login should be completed before running the installer when the registry requires authentication.
