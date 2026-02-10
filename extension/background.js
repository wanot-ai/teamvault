// TeamVault Extension — Background Service Worker
// Handles API calls, attestation verification, and token refresh.

(function () {
  'use strict';

  // ─── Constants ─────────────────────────────────────────────────────
  const TOKEN_REFRESH_INTERVAL_MS = 15 * 60 * 1000; // 15 minutes
  const ATTESTATION_CACHE_TTL_MS = 5 * 60 * 1000;   // 5 minutes
  const API_TIMEOUT_MS = 15000;

  // ─── In-memory caches ─────────────────────────────────────────────
  let attestationCache = { verified: null, timestamp: 0, error: null };
  let tokenRefreshTimer = null;

  // ─── Message Listener ──────────────────────────────────────────────
  chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    handleMessage(msg)
      .then(sendResponse)
      .catch((err) => sendResponse({ error: err.message || String(err) }));
    return true; // async response
  });

  async function handleMessage(msg) {
    switch (msg.type) {
      case 'LOGIN':
        return handleLogin(msg);
      case 'SEARCH_SECRETS':
        return handleSearchSecrets(msg);
      case 'GET_SECRET':
        return handleGetSecret(msg);
      case 'CHECK_ATTESTATION':
        return handleCheckAttestation();
      case 'REFRESH_TOKEN':
        return handleRefreshToken();
      default:
        return { error: `Unknown message type: ${msg.type}` };
    }
  }

  // ─── Login ─────────────────────────────────────────────────────────
  async function handleLogin({ server, email, password }) {
    const url = `${server}/api/v1/auth/login`;

    const resp = await fetchWithTimeout(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });

    if (!resp.ok) {
      const body = await resp.json().catch(() => ({}));
      throw new Error(body.error || `Login failed (${resp.status})`);
    }

    const data = await resp.json();
    if (!data.token) {
      throw new Error('Server returned empty token');
    }

    // Verify attestation on first connect if enabled
    const settings = await getSettings();
    if (settings.verifyAttestation) {
      try {
        await verifyAttestation(server);
      } catch (err) {
        // Don't block login, but warn
        console.warn('[TeamVault] Attestation verification failed:', err.message);
      }
    }

    // Start token refresh schedule
    scheduleTokenRefresh();

    return { token: data.token };
  }

  // ─── Search Secrets ────────────────────────────────────────────────
  async function handleSearchSecrets({ query }) {
    const { server, token } = await getCredentials();
    const url = `${server}/api/v1/secrets?q=${encodeURIComponent(query)}`;

    const resp = await fetchWithTimeout(url, {
      method: 'GET',
      headers: authHeaders(token),
    });

    if (resp.status === 401) {
      // Try refresh
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        return handleSearchSecrets({ query });
      }
      throw new Error('Session expired. Please log in again.');
    }

    if (!resp.ok) {
      const body = await resp.json().catch(() => ({}));
      throw new Error(body.error || `Search failed (${resp.status})`);
    }

    const data = await resp.json();
    return { secrets: data.secrets || [] };
  }

  // ─── Get Secret Value ──────────────────────────────────────────────
  async function handleGetSecret({ project, path }) {
    const { server, token } = await getCredentials();
    const url = `${server}/api/v1/secrets/${encodeURIComponent(project)}/${encodeURIComponent(path)}`;

    const resp = await fetchWithTimeout(url, {
      method: 'GET',
      headers: authHeaders(token),
    });

    if (resp.status === 401) {
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        return handleGetSecret({ project, path });
      }
      throw new Error('Session expired. Please log in again.');
    }

    if (!resp.ok) {
      const body = await resp.json().catch(() => ({}));
      throw new Error(body.error || `Failed to fetch secret (${resp.status})`);
    }

    const secret = await resp.json();
    return { secret };
  }

  // ─── TEE Attestation ──────────────────────────────────────────────
  async function handleCheckAttestation() {
    const settings = await getSettings();

    if (!settings.verifyAttestation) {
      return { verified: false, skipped: true };
    }

    // Return cached result if fresh
    if (attestationCache.timestamp > Date.now() - ATTESTATION_CACHE_TTL_MS) {
      return {
        verified: attestationCache.verified,
        error: attestationCache.error,
      };
    }

    try {
      const { server } = await getCredentials();
      const result = await verifyAttestation(server);
      return result;
    } catch (err) {
      return { verified: false, error: err.message };
    }
  }

  /**
   * Verify the server's TEE attestation evidence.
   *
   * This checks the /api/v1/attestation endpoint which returns:
   *   - platform: TEE platform (e.g., "sgx", "sev-snp", "tdx")
   *   - evidence: base64-encoded attestation report
   *   - certificate_chain: PEM certificate chain from the TEE platform
   *   - measurement: expected enclave/VM measurement hash
   *
   * In a production deployment, this would verify the evidence against
   * the TEE vendor's root of trust (Intel for SGX/TDX, AMD for SEV-SNP).
   * For now, we check that the endpoint responds with valid structure
   * and cache the result.
   */
  async function verifyAttestation(server) {
    const url = `${server}/api/v1/attestation`;
    const resp = await fetchWithTimeout(url, { method: 'GET' });

    if (!resp.ok) {
      const result = { verified: false, error: `Attestation endpoint returned ${resp.status}` };
      updateAttestationCache(result);
      return result;
    }

    const data = await resp.json();

    // Validate attestation structure
    if (!data.platform || !data.evidence) {
      const result = { verified: false, error: 'Invalid attestation response: missing platform or evidence' };
      updateAttestationCache(result);
      return result;
    }

    // Check supported platforms
    const supportedPlatforms = ['sgx', 'sev-snp', 'tdx', 'nitro'];
    if (!supportedPlatforms.includes(data.platform)) {
      const result = { verified: false, error: `Unsupported TEE platform: ${data.platform}` };
      updateAttestationCache(result);
      return result;
    }

    // Verify evidence is non-empty base64
    try {
      atob(data.evidence);
    } catch {
      const result = { verified: false, error: 'Invalid attestation evidence encoding' };
      updateAttestationCache(result);
      return result;
    }

    // Check certificate chain exists
    if (!data.certificate_chain || !data.certificate_chain.includes('BEGIN CERTIFICATE')) {
      const result = { verified: false, error: 'Missing or invalid certificate chain' };
      updateAttestationCache(result);
      return result;
    }

    // In production: verify the certificate chain against the TEE vendor's root CA,
    // validate the evidence signature, and check the measurement matches expected value.
    // For now, structural validation passes.
    const result = {
      verified: true,
      platform: data.platform,
      measurement: data.measurement,
    };
    updateAttestationCache(result);
    return result;
  }

  function updateAttestationCache(result) {
    attestationCache = {
      verified: result.verified,
      error: result.error || null,
      timestamp: Date.now(),
    };
  }

  // ─── Token Refresh ─────────────────────────────────────────────────
  async function handleRefreshToken() {
    const refreshed = await tryRefreshToken();
    return { refreshed };
  }

  async function tryRefreshToken() {
    try {
      const { server, token } = await getCredentials();
      const url = `${server}/api/v1/auth/refresh`;

      const resp = await fetchWithTimeout(url, {
        method: 'POST',
        headers: authHeaders(token),
      });

      if (!resp.ok) return false;

      const data = await resp.json();
      if (!data.token) return false;

      // Save new token
      await new Promise((resolve) => {
        chrome.storage.local.set({ tv_token: data.token }, resolve);
      });

      return true;
    } catch (err) {
      console.warn('[TeamVault] Token refresh failed:', err.message);
      return false;
    }
  }

  function scheduleTokenRefresh() {
    if (tokenRefreshTimer) clearInterval(tokenRefreshTimer);
    tokenRefreshTimer = setInterval(() => {
      tryRefreshToken().catch(() => {});
    }, TOKEN_REFRESH_INTERVAL_MS);
  }

  // ─── Helpers ───────────────────────────────────────────────────────
  function authHeaders(token) {
    return {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      'Authorization': `Bearer ${token}`,
    };
  }

  async function getCredentials() {
    return new Promise((resolve, reject) => {
      chrome.storage.local.get(['tv_server', 'tv_token'], (data) => {
        if (!data.tv_server || !data.tv_token) {
          reject(new Error('Not logged in'));
          return;
        }
        resolve({ server: data.tv_server, token: data.tv_token });
      });
    });
  }

  async function getSettings() {
    return new Promise((resolve) => {
      chrome.storage.local.get(['tv_verify_attestation'], (data) => {
        resolve({
          verifyAttestation: data.tv_verify_attestation !== false, // default: true
        });
      });
    });
  }

  /**
   * Fetch with a timeout to prevent hanging requests.
   */
  async function fetchWithTimeout(url, options = {}, timeoutMs = API_TIMEOUT_MS) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), timeoutMs);

    try {
      const resp = await fetch(url, {
        ...options,
        signal: controller.signal,
      });
      return resp;
    } catch (err) {
      if (err.name === 'AbortError') {
        throw new Error('Request timed out');
      }
      throw err;
    } finally {
      clearTimeout(timeout);
    }
  }

  // ─── Extension Install/Update ──────────────────────────────────────
  chrome.runtime.onInstalled.addListener((details) => {
    if (details.reason === 'install') {
      // Set default settings
      chrome.storage.local.set({
        tv_verify_attestation: true,
      });
    }
  });

  // Resume token refresh if already logged in
  getCredentials()
    .then(() => scheduleTokenRefresh())
    .catch(() => {}); // Not logged in, that's fine

})();
