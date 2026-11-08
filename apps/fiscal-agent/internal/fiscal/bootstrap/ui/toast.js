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

  /** ONLY list of API codes that carry AT's own (Portuguese) message rather than a localized one. */
  var AT_TEXT_CODES = { at_number_taken: true, movement_start_past: true };

  /** Map API / thrown objects to a short user-facing string (for toast). */
  /** saft_text_invalid (Agent field = dotted path, part = encoding|too_short|too_long, min/max):
   *  "Line 3: Portuguese name must use Latin characters…". Empty when i18n is unavailable. */
  function saftTextMessage(err) {
    var i18n = global.FiscalAdminI18n;
    if (!i18n || typeof i18n.fmt !== 'function' || typeof i18n.t !== 'function') return '';
    var field = err.field != null ? String(err.field) : '';
    var part = err.part != null ? String(err.part) : '';
    var key = 'errors.saft_text_invalid.' + part;
    var name = field.slice(field.lastIndexOf('.') + 1);
    var role = field.slice(0, field.indexOf('.'));
    var label = i18n.t('saft.field.' + name);
    if (label === 'saft.field.' + name) label = field;
    if (role === 'ship_from' || role === 'ship_to') {
      var roleLabel = i18n.t('gr.addr.role.' + role);
      if (roleLabel !== 'gr.addr.role.' + role) label = roleLabel + ' · ' + label;
    }
    var msg = i18n.fmt(key, { field: label, min: err.min, max: err.max });
    if (msg === key) return '';
    var line = /^lines\.(\d+)\./.exec(field);
    if (line) {
      msg = i18n.fmt('saft.line_prefix', { n: line[1] }) + msg;
      if (name === 'saft_name') msg += ' ' + i18n.t('saft.saft_name_hint');
    }
    return msg;
  }

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

    // Structured transport address: prefer role×part over the generic code string.
    if (code === 'transport_address_incomplete') {
      var role = err.field != null ? String(err.field) : '';
      var part = err.part != null ? String(err.part) : '';
      if (role && part) {
        try {
          var i18n = global.FiscalAdminI18n;
          if (i18n && typeof i18n.fmt === 'function' && typeof i18n.t === 'function') {
            var roleLabel = i18n.t('gr.addr.role.' + role);
            var needLabel = i18n.t('gr.addr.part.' + part);
            if (roleLabel && roleLabel !== 'gr.addr.role.' + role &&
                needLabel && needLabel !== 'gr.addr.part.' + part) {
              return i18n.fmt('gr.addr.incomplete', { role: roleLabel, need: needLabel });
            }
          }
        } catch (_) { /* */ }
      }
    }

    if (code === 'saft_text_invalid') {
      var st = saftTextMessage(err);
      if (st) return st;
    }

    // AT business rejection: localized prefix + AT's own message (Portuguese, the only precise reason).
    if (code === 'at_transport_rejected' && msg) {
      try {
        var i18nR = global.FiscalAdminI18n;
        if (i18nR && typeof i18nR.fmt === 'function') {
          var r = i18nR.fmt('errors.at_transport_rejected', { msg: msg });
          if (r && r !== 'errors.at_transport_rejected') return r;
        }
      } catch (_) { /* */ }
    }

    var fromCode = code ? tOr('errors.' + code, '') : '';
    // Codes whose message is AT's raw Portuguese text: the localized code text wins.
    if (fromCode && AT_TEXT_CODES[code]) return fromCode;
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
