/* Fiscal Admin toast — mirrors restaurant-ordering components/ui/Toast.tsx
 * API:
 *   FiscalUI.showToast(message, 'success'|'error'|'info')  — presentation only
 *   FiscalUI.formatError(err)                              — thrown/API → user string
 *   FiscalUI.reportError(err)                              — ONLY error-toast path for thrown objects
 * Placement: fixed bottom-right stack (same as ToastContainer).
 * Do not reimplement toast in page HTML; load this file once.
 *
 * Error UX contract (who speaks):
 *   - Business helpers ONLY throw { error, message } (message already i18n when client-side).
 *   - withBusy / .catch(FiscalUI.reportError) speak once.
 *   - Do not toast then throw the same failure.
 */
(function (global) {
  'use strict';

  var DURATION_MS = { success: 3000, error: 5000, info: 3000 };
  var ICONS = { success: '✅', error: '⚠️', info: 'ℹ️' };

  /** English API crumbs → i18n keys (never show raw tech to cashiers). */
  var MESSAGE_I18N = {
    'station_id required': 'settings.printers.need'
  };

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

  function tOr(key, fallback) {
    try {
      var i18n = global.FiscalAdminI18n;
      if (i18n && typeof i18n.t === 'function') {
        var v = i18n.t(key);
        if (v && v !== key) return v;
      }
    } catch (_) { /* */ }
    return fallback || '';
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
    if (err == null) return tOr('errors.unknown', 'unknown error');
    if (typeof err === 'string') return err;

    var code = err.error != null ? String(err.error) : '';
    var msg = '';
    if (err.message != null) msg = String(err.message);
    else if (typeof Error !== 'undefined' && err instanceof Error) msg = String(err.message || '');

    var aliasKey = MESSAGE_I18N[msg];
    if (aliasKey) return tOr(aliasKey, msg);
    if (code === 'validation_failed' && /station_id/i.test(msg)) {
      return tOr('settings.printers.need', msg);
    }

    var fromCode = code ? tOr('errors.' + code, '') : '';
    // Already-localized client message (CJK / accented) wins over error code.
    if (msg && /[^\x00-\x7F]/.test(msg)) return msg;
    if (fromCode) return fromCode;
    if (msg) return msg;
    if (code) return code;
    return tOr('errors.unknown', 'unknown error');
  }

  /** ONLY path that turns a thrown/API error object into an error toast. */
  function reportError(err) {
    showToast(formatError(err), 'error');
    return err;
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.showToast = showToast;
  global.FiscalUI.formatError = formatError;
  global.FiscalUI.reportError = reportError;
})(typeof window !== 'undefined' ? window : this);
