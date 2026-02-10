# TeamVault CSI Driver

## Overview

The TeamVault CSI (Container Storage Interface) driver integrates with the
[Secrets Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/) to
mount TeamVault secrets as files inside Kubernetes pod volumes. This enables
applications to read secrets from the filesystem without any code changes.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Kubernetes Node                                            │
│                                                             │
│  ┌─────────────┐      ┌──────────────────┐                  │
│  │  Application │      │  Secrets Store   │                  │
│  │  Pod         │◄────▶│  CSI Driver      │                  │
│  │             │      │  (DaemonSet)     │                  │
│  │  /mnt/secrets/     │                  │                  │
│  │    db-password     └────────┬─────────┘                  │
│  │    api-key                  │                            │
│  └─────────────┘               │                            │
│                                ▼                            │
│                    ┌───────────────────────┐                │
│                    │  TeamVault CSI        │                │
│                    │  Provider (DaemonSet) │                │
│                    │                       │                │
│                    │  gRPC Server          │                │
│                    │  /var/run/teamvault/  │                │
│                    │    teamvault.sock     │                │
│                    └───────────┬───────────┘                │
│                                │                            │
└────────────────────────────────┼────────────────────────────┘
                                 │ HTTPS
                                 ▼
                    ┌───────────────────────┐
                    │  TeamVault Server     │
                    │  (API)                │
                    └───────────────────────┘
```

## How It Works

1. **Pod Creation**: A pod references a `SecretProviderClass` in a CSI volume.
2. **CSI Mount**: The Secrets Store CSI driver calls the TeamVault provider via gRPC.
3. **Secret Fetch**: The provider authenticates to TeamVault using the pod's
   service account token or a referenced Kubernetes Secret.
4. **File Write**: Secrets are written as individual files into a tmpfs volume
   mounted into the pod.
5. **Optional Sync**: The CSI driver can optionally create Kubernetes `Secret`
   objects from the fetched values, enabling env-var consumption.

## Components

### CSI Driver Registration (`csi-driver.yaml`)

Registers the TeamVault provider with the Secrets Store CSI driver framework.

### Provider DaemonSet (`daemonset.yaml`)

Runs the TeamVault CSI provider on every node. The provider:

- Listens on a Unix domain socket at `/var/run/teamvault/teamvault.sock`
- Implements the `provider.SecretsStoreProviderServer` gRPC interface
- Fetches secrets from TeamVault over HTTPS
- Writes secrets as files to the target path
- Supports rotation via periodic re-mount

### SecretProviderClass (`../manifests/secret-provider-class.yaml`)

User-defined resource specifying which secrets to mount.

## Security Considerations

- **In-Memory Storage**: Secrets are stored on tmpfs volumes (RAM-backed), never
  written to disk.
- **Minimal Permissions**: The provider only needs read access to secrets in
  TeamVault. Write permissions are never required.
- **Token Rotation**: If using Kubernetes service account tokens, the provider
  supports projected service account tokens with automatic rotation.
- **Network Policy**: Consider applying a NetworkPolicy to restrict the provider's
  egress to only the TeamVault server.
- **Audit Trail**: All secret reads are logged in TeamVault's audit log.

## Installation

### Prerequisites

1. Install the Secrets Store CSI Driver:

```bash
helm repo add secrets-store-csi-driver \
  https://kubernetes-sigs.github.io/secrets-store-csi-driver/charts
helm install csi-secrets-store \
  secrets-store-csi-driver/secrets-store-csi-driver \
  --namespace kube-system \
  --set syncSecret.enabled=true \
  --set enableSecretRotation=true \
  --set rotationPollInterval=120s
```

2. Install the TeamVault CSI Provider:

```bash
kubectl apply -f kubernetes/csi-driver/csi-driver.yaml
kubectl apply -f kubernetes/csi-driver/daemonset.yaml
```

3. Create a TeamVault token secret:

```bash
kubectl create secret generic teamvault-creds \
  --namespace=default \
  --from-literal=token=<your-teamvault-token>
```

### Usage

Create a `SecretProviderClass`:

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: my-app-secrets
spec:
  provider: teamvault
  parameters:
    teamvaultAddr: "https://teamvault.teamvault-system.svc:8443"
    project: "my-project"
    objects: |
      - objectName: "database/password"
        objectAlias: "db-password"
      - objectName: "api/key"
        objectAlias: "api-key"
```

Reference it in your pod:

```yaml
volumes:
  - name: secrets
    csi:
      driver: secrets-store.csi.k8s.io
      readOnly: true
      volumeAttributes:
        secretProviderClass: my-app-secrets
      nodePublishSecretRef:
        name: teamvault-creds
```

## Rotation

When `enableSecretRotation=true` is set on the Secrets Store CSI Driver, it
periodically re-invokes the provider to refresh secrets. Updated values are
written to the mounted volume atomically. Applications using inotify or
periodic file reads will pick up the new values.

## Troubleshooting

### Provider Not Found

```
Error: provider "teamvault" not found
```

Ensure the TeamVault CSI provider DaemonSet is running and the socket exists:

```bash
kubectl get pods -n teamvault-system -l app=teamvault-csi-provider
kubectl exec -n teamvault-system <pod> -- ls -la /var/run/teamvault/
```

### Secret Not Found

```
Error: secret "my-project/path/to/secret" not found
```

Verify the secret exists in TeamVault:

```bash
teamvault kv get --project my-project --path path/to/secret
```

### Permission Denied

Ensure the token in your `teamvault-creds` secret has read access to the
requested project and paths.
