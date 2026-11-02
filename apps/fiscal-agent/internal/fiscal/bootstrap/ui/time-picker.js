/* FiscalUI TimePicker — 24h HH:MM sole time control for Admin (movement start).
 * API: FiscalUI.createTimePicker(container, options) -> { getValue, setValue, setDisabled, relabel, focus, close }
 * FiscalUI.hmInTimezone(date, tz) -> "HH:MM" wall clock in tz.
 * Do not use native input type=time beside this.
 */
(function (global) {
  'use strict';

  var POPUP_GAP = 6;
  var VIEWPORT_PAD = 8;
  var HM_RE = /^\d{2}:\d{2}$/;

  function resolveContainer(container) {
    if (typeof container === 'string') return document.querySelector(container);
    return container;
  }

  function pad2(n) {
    return String(n).padStart(2, '0');
  }

  function parseHM(hm) {
    var s = String(hm || '');
    if (!HM_RE.test(s)) return null;
    var h = Number(s.slice(0, 2));
    var m = Number(s.slice(3, 5));
    if (h > 23 || m > 59) return null;
    return { h: h, m: m };
  }

  function hmInTimezone(date, timeZone) {
    var d = date instanceof Date ? date : new Date();
    try {
      var parts = new Intl.DateTimeFormat('en-GB', {
        timeZone: timeZone || 'Europe/Lisbon',
        hour: '2-digit',
        minute: '2-digit',
        hourCycle: 'h23',
      }).formatToParts(d);
      var hour = '';
      var minute = '';
      for (var i = 0; i < parts.length; i++) {
        if (parts[i].type === 'hour') hour = parts[i].value;
        if (parts[i].type === 'minute') minute = parts[i].value;
      }
      if (hour.length === 1) hour = '0' + hour;
      if (minute.length === 1) minute = '0' + minute;
      if (HM_RE.test(hour + ':' + minute)) return hour + ':' + minute;
    } catch (_) { /* */ }
    return pad2(d.getHours()) + ':' + pad2(d.getMinutes());
  }

  function computePopupCoords(anchor, popup) {
    var anchorRect = anchor.getBoundingClientRect();
    var popupHeight = popup.offsetHeight;
    var popupWidth = Math.max(anchorRect.width, popup.offsetWidth);
    var spaceBelow = window.innerHeight - anchorRect.bottom;
    var spaceAbove = anchorRect.top;
    var openUpward = spaceBelow < popupHeight + POPUP_GAP && spaceAbove > spaceBelow;
    var top = openUpward ? anchorRect.top - popupHeight - POPUP_GAP : anchorRect.bottom + POPUP_GAP;
    var left = Math.max(
      VIEWPORT_PAD,
      Math.min(anchorRect.left, window.innerWidth - popupWidth - VIEWPORT_PAD)
    );
    top = Math.max(VIEWPORT_PAD, Math.min(top, window.innerHeight - popupHeight - VIEWPORT_PAD));
    return { top: top, left: left, width: Math.max(anchorRect.width, 9.5 * 16) };
  }

  function createTimePicker(container, options) {
    options = options || {};
    var root = resolveContainer(container);
    if (!root) throw new Error('FiscalUI.createTimePicker: container not found');

    var state = {
      value: parseHM(options.value) ? String(options.value) : '',
      disabled: !!options.disabled,
      open: false,
    };
    var onChange = typeof options.onChange === 'function' ? options.onChange : function () {};
    var popupEl = null;
    var hourSel = null;
    var minuteSel = null;

    function placeholderText() {
      if (typeof options.getPlaceholder === 'function') return options.getPlaceholder() || '';
      return options.placeholder || 'HH:MM';
    }

    root.classList.add('fiscal-time-picker');
    root.innerHTML =
      '<button type="button" class="fiscal-time-picker__trigger" data-role="trigger">' +
      '<span data-role="label"></span></button>';

    var trigger = root.querySelector('[data-role="trigger"]');
    var labelEl = root.querySelector('[data-role="label"]');

    function paintTrigger() {
      var has = !!parseHM(state.value);
      labelEl.textContent = has ? state.value : placeholderText();
      labelEl.classList.toggle('is-placeholder', !has);
      trigger.disabled = state.disabled;
      trigger.setAttribute('aria-expanded', state.open ? 'true' : 'false');
      trigger.classList.toggle('field-invalid', root.classList.contains('field-invalid'));
    }

    function close() {
      if (!state.open) return;
      state.open = false;
      if (popupEl && popupEl.parentNode) popupEl.parentNode.removeChild(popupEl);
      popupEl = null;
      hourSel = null;
      minuteSel = null;
      document.removeEventListener('mousedown', onDocDown, true);
      window.removeEventListener('scroll', reposition, true);
      window.removeEventListener('resize', reposition);
      paintTrigger();
    }

    function reposition() {
      if (!popupEl || !state.open) return;
      var c = computePopupCoords(root, popupEl);
      popupEl.style.top = c.top + 'px';
      popupEl.style.left = c.left + 'px';
      popupEl.style.minWidth = c.width + 'px';
      popupEl.style.visibility = 'visible';
    }

    function onDocDown(e) {
      var t = e.target;
      if (root.contains(t) || (popupEl && popupEl.contains(t))) return;
      close();
    }

    function applyFromSelects() {
      if (!hourSel || !minuteSel) return;
      var hm = pad2(Number(hourSel.value)) + ':' + pad2(Number(minuteSel.value));
      state.value = hm;
      paintTrigger();
      onChange(hm);
    }

    function paintPopup() {
      if (!popupEl) return;
      var cur = parseHM(state.value) || { h: 12, m: 0 };
      var hOpts = '';
      var mOpts = '';
      var i;
      for (i = 0; i < 24; i++) {
        hOpts +=
          '<option value="' +
          i +
          '"' +
          (i === cur.h ? ' selected' : '') +
          '>' +
          pad2(i) +
          '</option>';
      }
      for (i = 0; i < 60; i++) {
        mOpts +=
          '<option value="' +
          i +
          '"' +
          (i === cur.m ? ' selected' : '') +
          '>' +
          pad2(i) +
          '</option>';
      }
      popupEl.innerHTML =
        '<div class="fiscal-time-picker__row">' +
        '<label class="fiscal-time-picker__col"><span>HH</span><select data-role="hour">' +
        hOpts +
        '</select></label>' +
        '<span class="fiscal-time-picker__sep">:</span>' +
        '<label class="fiscal-time-picker__col"><span>MM</span><select data-role="minute">' +
        mOpts +
        '</select></label>' +
        '</div>';
      hourSel = popupEl.querySelector('[data-role="hour"]');
      minuteSel = popupEl.querySelector('[data-role="minute"]');
      hourSel.addEventListener('change', applyFromSelects);
      minuteSel.addEventListener('change', applyFromSelects);
    }

    function open() {
      if (state.disabled) return;
      if (state.open) {
        close();
        return;
      }
      state.open = true;
      popupEl = document.createElement('div');
      popupEl.className = 'fiscal-time-picker__popup';
      popupEl.style.visibility = 'hidden';
      document.body.appendChild(popupEl);
      paintPopup();
      paintTrigger();
      reposition();
      document.addEventListener('mousedown', onDocDown, true);
      window.addEventListener('scroll', reposition, true);
      window.addEventListener('resize', reposition);
    }

    trigger.addEventListener('click', open);
    paintTrigger();

    return {
      getValue: function () {
        return parseHM(state.value) ? state.value : '';
      },
      setValue: function (hm) {
        state.value = parseHM(hm) ? String(hm) : '';
        paintTrigger();
        if (state.open) paintPopup();
      },
      setDisabled: function (disabled) {
        state.disabled = !!disabled;
        if (state.disabled) close();
        paintTrigger();
      },
      relabel: function () {
        paintTrigger();
      },
      focus: function () {
        try {
          trigger.focus();
        } catch (_) { /* */ }
        root.classList.add('field-invalid');
        paintTrigger();
      },
      clearInvalid: function () {
        root.classList.remove('field-invalid');
        paintTrigger();
      },
      close: close,
    };
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.createTimePicker = createTimePicker;
  global.FiscalUI.hmInTimezone = hmInTimezone;
})(typeof window !== 'undefined' ? window : globalThis);
