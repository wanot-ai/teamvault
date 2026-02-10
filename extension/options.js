// TeamVault Extension — Options Page Script
// Server URL config, token management, and TEE attestation settings.

(function () {
  'use strict';

  // ─── DOM refs ──────────────────────────────────────────────────────
  const optServer = document.getElementById('opt-server');
  const optToken = document.getElementById('opt-token');
  const btnSaveServer = document.getElementById('btn-save-server');
  const serverStatus = document.getElementById('server-status');
  const optVerifyAttestation = document.getElementById('opt-verify-attestation');
  const optStrictAttestation = document.getElementById('opt-strict-attestation');
  const btnSaveAttestation = document.getElementById('btn-save-attestation');
  const attestationStatus = document.getElementById('attestation-status');
  const sessionInfo = document.getElementById('session-info');
  const btnClearSession = document.getElementById('btn-clear-session');

  // ─── Load current settings ─────────────────────────────────────────
  function loadSettings() {
    chrome.storage.local.get([
      'tv_server',
      'tv_token',
      'tv_email',
      'tv_verify_attestation',
      'tv_strict_attestation',
    ], (data) => {
      if (data.tv_server) optServer.value = data.tv_server;
      if (data.tv_token) optToken.placeholder = '••••••••••••••• (token saved)';

      optVerifyAttestation.checked = data.tv_verify_attestation !== false;
      optStrictAttestation.checked = data.tv_strict_attestation === true;

      // Session info
      if (data.tv_token) {
        const maskedToken = data.tv_token.substring(0, 12) + '••••••••';
        sessionInfo.innerHTML = `
          <div style="margin-bottom: 8px;">
            <strong style="color: #f1f5f9;">Server:</strong>
            <span style="color: #94a3b8;">${escapeHtml(data.tv_server || 'Not configured')}</span>
          </div>
          <div style="margin-bottom: 8px;">
            <strong style="color: #f1f5f9;">Email:</strong>
            <span style="color: #94a3b8;">${escapeHtml(data.tv_email || 'N/A')}</span>
          </div>
          <div>
            <strong style="color: #f1f5f9;">Token:</strong>
            <span class="token-display">${escapeHtml(maskedToken)}</span>
          </div>
        `;
        btnClearSession.style.display = 'inline-flex';
      } else {
        sessionInfo.innerHTML = `
          <p style="color: #64748b; font-size: 13px;">
            Not logged in. Use the extension popup or paste a token above.
          </p>
        `;
        btnClearSession.style.display = 'none';
      }
    });
  }

  // ─── Save Server Settings ──────────────────────────────────────────
  btnSaveServer.addEventListener('click', () => {
    const server = optServer.value.trim().replace(/\/+$/, '');
    const token = optToken.value.trim();

    if (!server) {
      showStatus(serverStatus, 'error', 'Server URL is required');
      return;
    }

    // Validate URL format
    try {
      new URL(server);
    } catch {
      showStatus(serverStatus, 'error', 'Invalid URL format');
      return;
    }

    const updates = { tv_server: server };
    if (token) {
      updates.tv_token = token;
    }

    chrome.storage.local.set(updates, () => {
      showStatus(serverStatus, 'success', 'Server settings saved');
      optToken.value = '';
      loadSettings();

      // Test connection
      if (token || optToken.placeholder.includes('saved')) {
        testConnection(server);
      }
    });
  });

  // ─── Save Attestation Settings ─────────────────────────────────────
  btnSaveAttestation.addEventListener('click', () => {
    chrome.storage.local.set({
      tv_verify_attestation: optVerifyAttestation.checked,
      tv_strict_attestation: optStrictAttestation.checked,
    }, () => {
      showStatus(attestationStatus, 'success', 'Attestation settings saved');
    });
  });

  // ─── Clear Session ─────────────────────────────────────────────────
  btnClearSession.addEventListener('click', () => {
    if (!confirm('Are you sure you want to log out? You will need to log in again.')) return;

    chrome.storage.local.remove([
      'tv_token',
      'tv_email',
    ], () => {
      loadSettings();
      showStatus(serverStatus, 'success', 'Session cleared. You have been logged out.');
    });
  });

  // ─── Test Connection ───────────────────────────────────────────────
  async function testConnection(server) {
    try {
      const resp = await fetch(`${server}/api/v1/health`, {
        method: 'GET',
        signal: AbortSignal.timeout(5000),
      });
      if (resp.ok) {
        showStatus(serverStatus, 'success', `✓ Connected to ${server}`);
      } else {
        showStatus(serverStatus, 'error', `Server responded with status ${resp.status}`);
      }
    } catch (err) {
      showStatus(serverStatus, 'error', `Cannot reach server: ${err.message}`);
    }
  }

  // ─── Helpers ───────────────────────────────────────────────────────
  function showStatus(el, type, message) {
    el.className = `status-msg ${type}`;
    el.textContent = message;
    el.style.display = 'block';

    // Auto-hide after 5 seconds
    setTimeout(() => {
      el.style.display = 'none';
    }, 5000);
  }

  function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  // ─── Init ──────────────────────────────────────────────────────────
  loadSettings();
})();
