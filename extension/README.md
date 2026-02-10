# TeamVault Browser Extension

End-to-end encrypted secret viewing for TeamVault. Secrets are decrypted and displayed only within the extension — never exposed on web pages.

## Features

- **Search & reveal secrets** in a secure popup — values never touch the page DOM
- **One-click copy** with auto-clear clipboard after 60 seconds
- **Auto-fill** for TeamVault web UI fields via extension (bypasses page JS)
- **TEE attestation verification** — checks the server's Trusted Execution Environment evidence before trusting it with your credentials
- **Token auto-refresh** — background service worker keeps your session alive
- **Dark theme** matching the TeamVault web UI

## Installation

### Chrome / Chromium / Edge / Brave

1. Open `chrome://extensions/` (or `edge://extensions/`, `brave://extensions/`)
2. Enable **Developer mode** (toggle in top-right)
3. Click **Load unpacked**
4. Select the `extension/` directory
5. The TeamVault icon appears in your toolbar

### Firefox

1. Open `about:debugging#/runtime/this-firefox`
2. Click **Load Temporary Add-on...**
3. Select `extension/manifest.json`
4. The TeamVault icon appears in your toolbar

> **Note:** Firefox temporary add-ons are removed when Firefox closes. For persistent installation, the extension must be signed via [addons.mozilla.org](https://addons.mozilla.org).

## Usage

### Login

1. Click the TeamVault icon in your toolbar
2. Enter your server URL (e.g., `https://vault.example.com`)
3. Enter your email and password
4. Click **Login**

Alternatively, paste an API token in **Settings** (gear icon → Options page).

### Search Secrets

1. Open the popup (click toolbar icon)
2. Start typing in the search bar (keyboard shortcut: `/`)
3. Click a result to view its detail
4. **Reveal** to see the value, **Copy** to copy to clipboard

### Auto-fill on TeamVault Web UI

When visiting a TeamVault web page, the extension:
- Shows a green badge: "Protected by TeamVault Extension"
- Replaces secret value displays with "View in TeamVault Extension" overlays
- Secret values are served through the extension, not the page JavaScript

### Settings

Click the ⚙ icon to configure:
- **Server URL** — your TeamVault instance
- **API Token** — alternative to email/password
- **TEE Attestation** — verify server TEE evidence (recommended for production)
- **Strict Mode** — block operations if attestation fails

## Security Model

### Threat Model

| Threat | Mitigation |
|--------|-----------|
| Malicious page JS reads secrets | Secrets are only in extension context, never injected into page DOM |
| XSS on TeamVault web UI | Content script overlays prevent page JS from accessing secret values |
| Man-in-the-middle | HTTPS + TEE attestation verification |
| Compromised server | TEE attestation checks hardware evidence; secrets encrypted end-to-end |
| Clipboard sniffing | Auto-clear clipboard after 60 seconds |
| Screen capture | Auto-hide revealed values after 30 seconds |
| Extension compromise | Manifest V3 service worker has no persistent background page; minimal permissions |

### Data Flow

```
User → Extension Popup → Background Service Worker → TeamVault API
                ↓
         Render in popup
    (never touches page DOM)
```

### Permissions

| Permission | Why |
|-----------|-----|
| `activeTab` | Detect TeamVault pages for auto-fill |
| `storage` | Store server URL, token, and preferences |
| `host_permissions` | API calls to your TeamVault server |

### TEE Attestation

The extension can verify the server's Trusted Execution Environment (TEE) attestation before trusting it. This checks:

1. The `/api/v1/attestation` endpoint returns valid evidence
2. The TEE platform is supported (SGX, SEV-SNP, TDX, or Nitro)
3. The evidence is properly signed (production: verified against vendor root of trust)
4. The certificate chain is valid

Enable in Settings → TEE Attestation → **Verify server attestation**.

## Development

```bash
# Edit files, then reload in Chrome:
# chrome://extensions/ → TeamVault → 🔄 (reload button)

# Structure:
extension/
├── manifest.json       # Manifest V3 config
├── popup.html          # Search/reveal popup
├── popup.js            # Popup logic
├── background.js       # Service worker (API, attestation, token refresh)
├── content.js          # Page integration (auto-fill, badge)
├── content-inject.css  # Styles injected into TeamVault pages
├── options.html        # Settings page
├── options.js          # Settings logic
├── styles.css          # Popup/options shared styles
├── icons/              # Extension icons (16, 48, 128px)
│   ├── icon.svg        # Source SVG
│   └── *.png           # Generated PNGs
└── README.md           # This file
```

## License

Same as the TeamVault project.
