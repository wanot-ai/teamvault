// TeamVault Extension — Content Script
// Detects TeamVault web UI pages and provides secure auto-fill via extension.
// Secret values are NEVER injected into the DOM — they flow through the extension popup.

(function () {
  'use strict';

  // ─── Constants ─────────────────────────────────────────────────────
  const BADGE_ID = 'teamvault-ext-badge';
  const FILL_ATTR = 'data-teamvault-field';

  // ─── Detection ─────────────────────────────────────────────────────
  function isTeamVaultPage() {
    // Check multiple signals
    const meta = document.querySelector('meta[name="teamvault"]');
    if (meta) return true;

    const title = document.title.toLowerCase();
    if (title.includes('teamvault')) return true;

    // Check for TeamVault-specific DOM elements
    const tvRoot = document.querySelector('[data-teamvault-app]');
    if (tvRoot) return true;

    return false;
  }

  // ─── Badge Indicator ───────────────────────────────────────────────
  function injectBadge() {
    if (document.getElementById(BADGE_ID)) return;

    const badge = document.createElement('div');
    badge.id = BADGE_ID;
    badge.innerHTML = `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14" style="vertical-align: middle;">
        <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
        <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
        <circle cx="12" cy="16" r="1"/>
      </svg>
      <span>Protected by TeamVault Extension</span>
    `;
    document.body.appendChild(badge);
  }

  // ─── Auto-fill Integration ─────────────────────────────────────────
  // Instead of putting secret values on the page, we intercept TeamVault
  // secret display elements and replace them with extension-powered UI.
  //
  // The flow:
  //   1. Content script finds elements marked with data-teamvault-field
  //   2. Replaces their content with a "Reveal via Extension" button
  //   3. Clicking the button opens the extension popup (or sends a message
  //      to the background script to handle the reveal securely)
  //
  // This prevents secrets from ever appearing in the page DOM where
  // malicious scripts, browser devtools, or screen capture could grab them.

  function setupAutoFill() {
    // Find all secret value fields in the TeamVault web UI
    const secretFields = document.querySelectorAll(
      `[${FILL_ATTR}], .secret-value-display, [data-secret-path]`
    );

    secretFields.forEach((field) => {
      // Skip already processed fields
      if (field.dataset.tvExtProcessed) return;
      field.dataset.tvExtProcessed = 'true';

      const secretPath = field.dataset.secretPath || field.dataset.teamvaultField || '';

      // Save original content and replace
      const originalDisplay = field.style.display;
      field.style.position = 'relative';

      // Create overlay
      const overlay = document.createElement('div');
      overlay.className = 'teamvault-ext-overlay';
      overlay.innerHTML = `
        <div class="teamvault-ext-overlay-content">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16" style="vertical-align: middle; margin-right: 6px;">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
          </svg>
          <span>View in TeamVault Extension</span>
        </div>
      `;

      overlay.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();

        // Notify the extension to open with this secret
        chrome.runtime.sendMessage({
          type: 'OPEN_SECRET',
          path: secretPath,
        });
      });

      // Mask the original value
      const originalText = field.textContent;
      if (originalText && originalText.trim().length > 0) {
        field.textContent = '••••••••••••';
      }

      field.appendChild(overlay);
    });
  }

  // ─── Mutation Observer ─────────────────────────────────────────────
  // Watch for dynamically added secret fields (SPA navigation)
  function observeDOMChanges() {
    const observer = new MutationObserver((mutations) => {
      let hasRelevantChanges = false;
      for (const mutation of mutations) {
        if (mutation.addedNodes.length > 0) {
          hasRelevantChanges = true;
          break;
        }
      }
      if (hasRelevantChanges) {
        // Debounce
        clearTimeout(observeDOMChanges._timeout);
        observeDOMChanges._timeout = setTimeout(setupAutoFill, 200);
      }
    });

    observer.observe(document.body, {
      childList: true,
      subtree: true,
    });
  }

  // ─── Message Listener (from background/popup) ─────────────────────
  chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    if (msg.type === 'FILL_FIELD') {
      // Securely fill a specific input field (e.g., for CI/CD token fields)
      // Only fills actual <input> elements, never display-only fields
      const input = document.querySelector(
        `input[${FILL_ATTR}="${msg.fieldId}"], input[data-secret-path="${msg.path}"]`
      );
      if (input && (input.tagName === 'INPUT' || input.tagName === 'TEXTAREA')) {
        // Use native input setter to trigger React/Vue change events
        const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
          window.HTMLInputElement.prototype, 'value'
        ).set;
        nativeInputValueSetter.call(input, msg.value);
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        sendResponse({ filled: true });
      } else {
        sendResponse({ filled: false, error: 'Field not found' });
      }
    }

    if (msg.type === 'PING') {
      sendResponse({ pong: true, isTeamVault: isTeamVaultPage() });
    }

    return true;
  });

  // ─── Initialize ────────────────────────────────────────────────────
  function init() {
    if (!isTeamVaultPage()) {
      // Not a TeamVault page — still listen for messages but don't inject UI
      return;
    }

    // Inject badge
    injectBadge();

    // Set up auto-fill protection
    setupAutoFill();

    // Watch for SPA navigation changes
    observeDOMChanges();

    // Notify background that we're on a TeamVault page
    chrome.runtime.sendMessage({
      type: 'TEAMVAULT_PAGE_DETECTED',
      url: window.location.href,
    });
  }

  // Run after DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
