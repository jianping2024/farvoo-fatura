/* FiscalUI DatePicker — restaurant @mesa/ui DatePicker analogue (portal month grid).
 * API: FiscalUI.createDatePicker(container, options) -> { getValue, setValue, setDisabled, relabel, close }
 * ONLY shared single-date picker for Admin (date-range mounts two of these).
 */
(function (global) {
  'use strict';

  var POPUP_GAP = 6;
  var POPUP_MIN_WIDTH = 280;
  var VIEWPORT_PAD = 8;
  var WEEKDAYS = {
    zh: ['日', '一', '二', '三', '四', '五', '六'],
    en: ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa'],
    pt: ['Do', 'Se', 'Te', 'Qu', 'Qu', 'Se', 'Sá'],
  };
  var MONTHS = {
    zh: ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'],
    en: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'],
    pt: ['Jan', 'Fev', 'Mar', 'Abr', 'Mai', 'Jun', 'Jul', 'Ago', 'Set', 'Out', 'Nov', 'Dez'],
  };

  function resolveContainer(container) {
    if (typeof container === 'string') return document.querySelector(container);
    return container;
  }

  function parseYMD(ymd) {
    var p = String(ymd || '').split('-');
    if (p.length !== 3) return null;
    var y = Number(p[0]);
    var m = Number(p[1]);
    var d = Number(p[2]);
    if (!y || !m || !d) return null;
    return { y: y, m: m, d: d };
  }

  function toYMD(y, m, d) {
    return y + '-' + String(m).padStart(2, '0') + '-' + String(d).padStart(2, '0');
  }

  function daysInMonth(y, m) {
    return new Date(y, m, 0).getDate();
  }

  function formatDisplay(ymd, loc) {
    var p = parseYMD(ymd);
    if (!p) return '';
    if (loc === 'zh') return p.y + '/' + p.m + '/' + p.d;
    return toYMD(p.y, p.m, p.d);
  }

  function computePopupCoords(anchor, popup) {
    var anchorRect = anchor.getBoundingClientRect();
    var popupHeight = popup.offsetHeight;
    var popupWidth = Math.max(POPUP_MIN_WIDTH, popup.offsetWidth);
    var spaceBelow = window.innerHeight - anchorRect.bottom;
    var spaceAbove = anchorRect.top;
    var openUpward = spaceBelow < popupHeight + POPUP_GAP && spaceAbove > spaceBelow;
    var top = openUpward ? anchorRect.top - popupHeight - POPUP_GAP : anchorRect.bottom + POPUP_GAP;
    var left = Math.max(
      VIEWPORT_PAD,
      Math.min(anchorRect.left, window.innerWidth - popupWidth - VIEWPORT_PAD)
    );
    top = Math.max(VIEWPORT_PAD, Math.min(top, window.innerHeight - popupHeight - VIEWPORT_PAD));
    return { top: top, left: left };
  }

  function createDatePicker(container, options) {
    options = options || {};
    var root = resolveContainer(container);
    if (!root) throw new Error('FiscalUI.createDatePicker: container not found');

    var state = {
      value: String(options.value || ''),
      disabled: !!options.disabled,
      open: false,
      viewY: 0,
      viewM: 0,
    };
    var onChange = typeof options.onChange === 'function' ? options.onChange : function () {};
    var popupEl = null;

    function locale() {
      var loc = typeof options.getLocale === 'function' ? options.getLocale() : 'zh';
      return loc === 'pt' || loc === 'en' ? loc : 'zh';
    }

    function placeholderText() {
      if (typeof options.getPlaceholder === 'function') return options.getPlaceholder() || '';
      return options.placeholder || '';
    }

    function isDisabledDay(ymd) {
      var min = options.min ? String(options.min) : '';
      var max = options.max ? String(options.max) : '';
      if (min && ymd < min) return true;
      if (max && ymd > max) return true;
      return false;
    }

    root.classList.add('fiscal-date-picker');
    root.innerHTML =
      '<button type="button" class="fiscal-date-picker__trigger" data-role="trigger">' +
      '<span data-role="label"></span></button>';

    var trigger = root.querySelector('[data-role="trigger"]');
    var labelEl = root.querySelector('[data-role="label"]');

    function paintTrigger() {
      var has = !!parseYMD(state.value);
      labelEl.textContent = has ? formatDisplay(state.value, locale()) : placeholderText();
      labelEl.classList.toggle('is-placeholder', !has);
      trigger.disabled = state.disabled;
      trigger.setAttribute('aria-expanded', state.open ? 'true' : 'false');
    }

    function close() {
      if (!state.open) return;
      state.open = false;
      if (popupEl && popupEl.parentNode) popupEl.parentNode.removeChild(popupEl);
      popupEl = null;
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
      popupEl.style.visibility = 'visible';
    }

    function onDocDown(e) {
      var t = e.target;
      if (root.contains(t) || (popupEl && popupEl.contains(t))) return;
      close();
    }

    function onPopupClick(e) {
      var nav = e.target.closest('[data-nav]');
      if (nav) {
        var delta = Number(nav.getAttribute('data-nav')) || 0;
        state.viewM += delta;
        if (state.viewM < 1) {
          state.viewM = 12;
          state.viewY -= 1;
        }
        if (state.viewM > 12) {
          state.viewM = 1;
          state.viewY += 1;
        }
        paintPopup();
        reposition();
        return;
      }
      var day = e.target.closest('[data-day]');
      if (!day || day.disabled) return;
      var ymd = day.getAttribute('data-day');
      if (!ymd || isDisabledDay(ymd)) return;
      state.value = ymd;
      paintTrigger();
      close();
      onChange(ymd);
    }

    function paintPopup() {
      if (!popupEl) return;
      var loc = locale();
      var weeks = WEEKDAYS[loc] || WEEKDAYS.zh;
      var months = MONTHS[loc] || MONTHS.zh;
      var y = state.viewY;
      var m = state.viewM;
      var firstDow = new Date(y, m - 1, 1).getDay();
      var dim = daysInMonth(y, m);
      var html =
        '<div class="fiscal-date-picker__nav">' +
        '<button type="button" class="secondary" data-nav="-1" aria-label="prev">‹</button>' +
        '<div class="fiscal-date-picker__caption">' +
        (loc === 'zh' ? y + '年' + months[m - 1] : months[m - 1] + ' ' + y) +
        '</div>' +
        '<button type="button" class="secondary" data-nav="1" aria-label="next">›</button>' +
        '</div>' +
        '<div class="fiscal-date-picker__weekdays">' +
        weeks
          .map(function (w) {
            return '<span>' + w + '</span>';
          })
          .join('') +
        '</div>' +
        '<div class="fiscal-date-picker__grid">';
      var i;
      for (i = 0; i < firstDow; i++) html += '<span class="fiscal-date-picker__pad"></span>';
      for (var d = 1; d <= dim; d++) {
        var ymd = toYMD(y, m, d);
        var cls = 'fiscal-date-picker__day';
        if (ymd === state.value) cls += ' is-selected';
        if (isDisabledDay(ymd)) cls += ' is-disabled';
        html +=
          '<button type="button" class="' +
          cls +
          '" data-day="' +
          ymd +
          '"' +
          (isDisabledDay(ymd) ? ' disabled' : '') +
          '>' +
          d +
          '</button>';
      }
      html += '</div>';
      popupEl.innerHTML = html;
    }

    function open() {
      if (state.disabled) return;
      if (state.open) {
        close();
        return;
      }
      var p = parseYMD(state.value) || parseYMD(options.max) || parseYMD(options.min);
      var now = new Date();
      state.viewY = p ? p.y : now.getFullYear();
      state.viewM = p ? p.m : now.getMonth() + 1;
      state.open = true;
      popupEl = document.createElement('div');
      popupEl.className = 'fiscal-date-picker__popup';
      popupEl.style.visibility = 'hidden';
      popupEl.style.minWidth = POPUP_MIN_WIDTH + 'px';
      popupEl.addEventListener('click', onPopupClick);
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
        return state.value;
      },
      setValue: function (ymd) {
        state.value = String(ymd || '');
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
        if (state.open) paintPopup();
      },
      close: close,
    };
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.createDatePicker = createDatePicker;
})(typeof window !== 'undefined' ? window : globalThis);
