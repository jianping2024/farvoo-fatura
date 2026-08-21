/* Fiscal Admin toast — mirrors restaurant-ordering components/ui/Toast.tsx
 * API: FiscalUI.showToast(message, 'success'|'error'|'info')
 * Placement: fixed bottom-right stack (same as ToastContainer).
 * Do not reimplement toast in page HTML; load this file once.
 */
(function (global) {
  'use strict';

  var DURATION_MS = { success: 3000, error: 5000, info: 3000 };
  var ICONS = { success: '✅', error: '⚠️', info: 'ℹ️' };

  function ensureRoot() {
    var el = document.getElementById('fiscal-toast-root');
    if (el) return el;
    el = document.createElement('div');
    el.id = 'fiscal-toast-root';
    el.className = 'fiscal-toast-root';
    el.setAttribute('aria-live', 'polite');
    document.body.appendChild(el);
    return el;
  }

  /**
   * @param {string} message
   * @param {'success'|'error'|'info'} [type]
   */
  function showToast(message, type) {
    type = type || 'info';
    if (type !== 'success' && type !== 'error' && type !== 'info') {
      type = 'info';
    }
    var root = ensureRoot();
    var el = document.createElement('div');
    el.className = 'fiscal-toast fiscal-toast--' + type;
    el.setAttribute('role', 'status');

    var inner = document.createElement('div');
    inner.className = 'fiscal-toast__inner';

    var icon = document.createElement('span');
    icon.className = 'fiscal-toast__icon';
    icon.setAttribute('aria-hidden', 'true');
    icon.textContent = ICONS[type];

    var msg = document.createElement('p');
    msg.className = 'fiscal-toast__msg';
    msg.textContent = String(message == null ? '' : message);

    inner.appendChild(icon);
    inner.appendChild(msg);
    el.appendChild(inner);
    root.appendChild(el);

    var ms = DURATION_MS[type] || 3000;
    setTimeout(function () {
      el.classList.add('fiscal-toast--hide');
      setTimeout(function () {
        if (el.parentNode) el.parentNode.removeChild(el);
      }, 300);
    }, ms);
  }

  /** Map API / thrown objects to a short user-facing string (for toast). */
  function formatError(err) {
    if (err == null) return 'unknown error';
    if (typeof err === 'string') return err;
    if (err.error || err.message) {
      return (err.error ? String(err.error) + ': ' : '') + String(err.message || '');
    }
    try {
      return JSON.stringify(err);
    } catch (_) {
      return String(err);
    }
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.showToast = showToast;
  global.FiscalUI.formatError = formatError;
})(typeof window !== 'undefined' ? window : this);
