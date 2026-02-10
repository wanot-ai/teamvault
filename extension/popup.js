// TeamVault Extension — Popup Script
// Handles secret search, reveal, and copy within the popup (never on the page).

(function () {
  'use strict';

  // ─── State ─────────────────────────────────────────────────────────
  let currentSecret = null;
  let isRevealed = false;
  let searchTimeout = null;

  // ─── DOM refs ──────────────────────────────────────────────────────
  const viewLogin = document.getElementById('view-login');
  const viewMain = document.getElementById('view-main');
  const loginServer = document.getElementById('login-server');
  const loginEmail = document.getElementById('login-email');
  const loginPassword = document.getElementById('login-password');
  const loginError = document.getElementById('login-error');
  const btnLogin = document.getElementById('btn-login');
  const linkTokenLogin = document.getElementById('link-token-login');
  const btnSettings = document.getElementById('btn-settings');
  const searchInput = document.getElementById('search-input');
  const resultsEmpty = document.getElementById('results-empty');
  const resultsLoading = document.getElementById('results-loading');
  const resultsList = document.getElementById('results-list');
  const secretDetail = document.getElementById('secret-detail');
  const detailPath = document.getElementById('detail-path');
  const detailValue = document.getElementById('detail-value');
  const detailVersion = document.getElementById('detail-version');
  const detailCreated = document.getElementById('detail-created');
  const btnBack = document.getElementById('btn-back');
  const btnReveal = document.getElementById('btn-reveal');
  const btnCopy = document.getElementById('btn-copy');
  const footerUser = document.getElementById('footer-user');
  const btnLogout = document.getElementById('btn-logout');
  const attestationBadge = document.getElementById('attestation-badge');

  // ─── Initialization ────────────────────────────────────────────────
  async function init() {
    const session = await getSession();
    if (session && session.token) {
      showMainView(session);
      updateAttestationBadge();
    } else {
      showLoginView();
    }
    bindEvents();
  }

  function bindEvents() {
    btnLogin.addEventListener('click', handleLogin);
    loginPassword.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') handleLogin();
    });
    linkTokenLogin.addEventListener('click', (e) => {
      e.preventDefault();
      chrome.runtime.openOptionsPage();
    });
    btnSettings.addEventListener('click', () => {
      chrome.runtime.openOptionsPage();
    });
    searchInput.addEventListener('input', handleSearch);
    btnBack.addEventListener('click', showSearchResults);
    btnReveal.addEventListener('click', toggleReveal);
    btnCopy.addEventListener('click', handleCopy);
    btnLogout.addEventListener('click', handleLogout);

    // Keyboard shortcut: / focuses search
    document.addEventListener('keydown', (e) => {
      if (e.key === '/' && document.activeElement !== searchInput) {
        e.preventDefault();
        searchInput.focus();
      }
      if (e.key === 'Escape') {
        if (secretDetail.style.display !== 'none') {
          showSearchResults();
        }
      }
    });
  }

  // ─── Session Management ────────────────────────────────────────────
  async function getSession() {
    return new Promise((resolve) => {
      chrome.storage.local.get(['tv_server', 'tv_token', 'tv_email'], (data) => {
        if (data.tv_token && data.tv_server) {
          resolve({
            server: data.tv_server,
            token: data.tv_token,
            email: data.tv_email || '',
          });
        } else {
          resolve(null);
        }
      });
    });
  }

  async function saveSession(server, token, email) {
    return new Promise((resolve) => {
      chrome.storage.local.set({
        tv_server: server,
        tv_token: token,
        tv_email: email,
      }, resolve);
    });
  }

  async function clearSession() {
    return new Promise((resolve) => {
      chrome.storage.local.remove(['tv_server', 'tv_token', 'tv_email'], resolve);
    });
  }

  // ─── Views ─────────────────────────────────────────────────────────
  function showLoginView() {
    viewLogin.style.display = 'block';
    viewMain.style.display = 'none';
    loginError.style.display = 'none';

    // Pre-fill server from saved config
    chrome.storage.local.get(['tv_server', 'tv_email'], (data) => {
      if (data.tv_server) loginServer.value = data.tv_server;
      if (data.tv_email) loginEmail.value = data.tv_email;
    });
  }

  function showMainView(session) {
    viewLogin.style.display = 'none';
    viewMain.style.display = 'block';
    footerUser.textContent = session.email || 'Connected';
    searchInput.focus();
  }

  function showSearchResults() {
    secretDetail.style.display = 'none';
    document.getElementById('results-container').style.display = 'block';
    currentSecret = null;
    isRevealed = false;
    searchInput.focus();
  }

  function showSecretDetail(secret) {
    currentSecret = secret;
    isRevealed = false;

    document.getElementById('results-container').style.display = 'none';
    secretDetail.style.display = 'block';

    detailPath.textContent = secret.path;
    detailValue.textContent = '••••••••••••';
    detailValue.classList.add('masked');
    detailVersion.textContent = `v${secret.version || 1}`;
    detailCreated.textContent = secret.created_at ? formatDate(secret.created_at) : '';

    btnReveal.innerHTML = `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
      Reveal`;
  }

  // ─── Login ─────────────────────────────────────────────────────────
  async function handleLogin() {
    const server = loginServer.value.trim().replace(/\/+$/, '');
    const email = loginEmail.value.trim();
    const password = loginPassword.value;

    if (!server || !email || !password) {
      showLoginError('All fields are required');
      return;
    }

    btnLogin.disabled = true;
    btnLogin.textContent = 'Connecting...';
    loginError.style.display = 'none';

    try {
      // Send login request through background service worker
      const response = await sendMessage({
        type: 'LOGIN',
        server,
        email,
        password,
      });

      if (response.error) {
        showLoginError(response.error);
        return;
      }

      await saveSession(server, response.token, email);
      showMainView({ server, token: response.token, email });
      updateAttestationBadge();
    } catch (err) {
      showLoginError(err.message || 'Connection failed');
    } finally {
      btnLogin.disabled = false;
      btnLogin.textContent = 'Login';
    }
  }

  function showLoginError(msg) {
    loginError.textContent = msg;
    loginError.style.display = 'block';
  }

  async function handleLogout() {
    await clearSession();
    resultsList.innerHTML = '';
    showLoginView();
  }

  // ─── Search ────────────────────────────────────────────────────────
  function handleSearch() {
    const query = searchInput.value.trim();

    if (searchTimeout) clearTimeout(searchTimeout);

    if (!query) {
      resultsList.innerHTML = '';
      resultsEmpty.style.display = 'block';
      resultsLoading.style.display = 'none';
      return;
    }

    resultsEmpty.style.display = 'none';
    resultsLoading.style.display = 'flex';

    // Debounce 300ms
    searchTimeout = setTimeout(() => performSearch(query), 300);
  }

  async function performSearch(query) {
    try {
      const response = await sendMessage({
        type: 'SEARCH_SECRETS',
        query,
      });

      resultsLoading.style.display = 'none';

      if (response.error) {
        resultsList.innerHTML = `<li class="result-error">${escapeHtml(response.error)}</li>`;
        return;
      }

      const secrets = response.secrets || [];
      if (secrets.length === 0) {
        resultsList.innerHTML = '<li class="result-empty">No secrets found</li>';
        return;
      }

      resultsList.innerHTML = secrets.map((s) => `
        <li class="result-item" data-project="${escapeAttr(s.project_id || '')}" data-path="${escapeAttr(s.path)}">
          <div class="result-path">${highlightMatch(s.path, query)}</div>
          <div class="result-meta">
            ${s.description ? `<span class="result-desc">${escapeHtml(s.description)}</span>` : ''}
          </div>
        </li>
      `).join('');

      // Bind click events on results
      resultsList.querySelectorAll('.result-item').forEach((el) => {
        el.addEventListener('click', () => {
          fetchAndShowSecret(el.dataset.project, el.dataset.path);
        });
      });
    } catch (err) {
      resultsLoading.style.display = 'none';
      resultsList.innerHTML = `<li class="result-error">${escapeHtml(err.message)}</li>`;
    }
  }

  async function fetchAndShowSecret(project, path) {
    try {
      const response = await sendMessage({
        type: 'GET_SECRET',
        project,
        path,
      });

      if (response.error) {
        alert('Failed to fetch secret: ' + response.error);
        return;
      }

      showSecretDetail(response.secret);
    } catch (err) {
      alert('Error: ' + err.message);
    }
  }

  // ─── Reveal / Copy ─────────────────────────────────────────────────
  function toggleReveal() {
    if (!currentSecret) return;

    if (isRevealed) {
      detailValue.textContent = '••••••••••••';
      detailValue.classList.add('masked');
      btnReveal.innerHTML = `
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
          <circle cx="12" cy="12" r="3"/>
        </svg>
        Reveal`;
      isRevealed = false;
    } else {
      detailValue.textContent = currentSecret.value || '';
      detailValue.classList.remove('masked');
      btnReveal.innerHTML = `
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/>
          <line x1="1" y1="1" x2="23" y2="23"/>
        </svg>
        Hide`;
      isRevealed = true;

      // Auto-hide after 30 seconds
      setTimeout(() => {
        if (isRevealed && currentSecret) {
          toggleReveal();
        }
      }, 30000);
    }
  }

  async function handleCopy() {
    if (!currentSecret || !currentSecret.value) return;

    try {
      await navigator.clipboard.writeText(currentSecret.value);
      const originalText = btnCopy.innerHTML;
      btnCopy.innerHTML = `
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <polyline points="20 6 9 17 4 12"/>
        </svg>
        Copied!`;
      btnCopy.classList.add('btn-success');

      setTimeout(() => {
        btnCopy.innerHTML = originalText;
        btnCopy.classList.remove('btn-success');
      }, 2000);

      // Clear clipboard after 60 seconds for security
      setTimeout(() => {
        navigator.clipboard.writeText('').catch(() => {});
      }, 60000);
    } catch (err) {
      // Fallback for environments where clipboard API fails
      const textarea = document.createElement('textarea');
      textarea.value = currentSecret.value;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  }

  // ─── Attestation Badge ─────────────────────────────────────────────
  async function updateAttestationBadge() {
    try {
      const response = await sendMessage({ type: 'CHECK_ATTESTATION' });
      if (response.verified) {
        attestationBadge.className = 'badge badge-verified';
        attestationBadge.title = 'TEE attestation verified ✓';
      } else if (response.skipped) {
        attestationBadge.className = 'badge badge-skipped';
        attestationBadge.title = 'TEE attestation skipped (disabled in settings)';
      } else {
        attestationBadge.className = 'badge badge-failed';
        attestationBadge.title = 'TEE attestation failed: ' + (response.error || 'unknown');
      }
    } catch {
      attestationBadge.className = 'badge badge-unknown';
      attestationBadge.title = 'Attestation status unknown';
    }
  }

  // ─── Message Passing ───────────────────────────────────────────────
  function sendMessage(msg) {
    return new Promise((resolve, reject) => {
      chrome.runtime.sendMessage(msg, (response) => {
        if (chrome.runtime.lastError) {
          reject(new Error(chrome.runtime.lastError.message));
        } else {
          resolve(response || {});
        }
      });
    });
  }

  // ─── Helpers ───────────────────────────────────────────────────────
  function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  function escapeAttr(str) {
    return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  function highlightMatch(text, query) {
    if (!query) return escapeHtml(text);
    const escaped = escapeHtml(text);
    const queryEscaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(`(${queryEscaped})`, 'gi');
    return escaped.replace(regex, '<mark>$1</mark>');
  }

  function formatDate(isoStr) {
    try {
      const d = new Date(isoStr);
      return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    } catch {
      return isoStr;
    }
  }

  // ─── Boot ──────────────────────────────────────────────────────────
  init();
})();
