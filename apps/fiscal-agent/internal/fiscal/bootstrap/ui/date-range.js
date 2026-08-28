/* Fiscal Admin date range — ONLY shared date filter (preset + native type=date from/to).
 * API: FiscalUI.createDateRangeFilter(container, options)
 *       FiscalUI.dateRangePresetToDates(preset, timezone)
 */
(function (global) {
  'use strict';

  var PRESETS = [
    { id: 'today', label: '今天' },
    { id: 'yesterday', label: '昨天' },
    { id: 'last7', label: '近7天' },
    { id: 'month', label: '本月' },
    { id: 'custom', label: '自定义' },
  ];

  function ymdInTZ(date, tz) {
    return new Intl.DateTimeFormat('en-CA', {
      timeZone: tz,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(date);
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

  function addDaysYMD(ymd, delta, tz) {
    var p = parseYMD(ymd);
    if (!p) return ymd;
    var utc = Date.UTC(p.y, p.m - 1, p.d + delta, 12, 0, 0);
    return ymdInTZ(new Date(utc), tz);
  }

  /** ONLY preset → {from,to} calculator (invoice_date filter). */
  function dateRangePresetToDates(preset, timezone) {
    var tz = timezone || 'Europe/Lisbon';
    var today = ymdInTZ(new Date(), tz);
    if (preset === 'today') return { preset: 'today', from: today, to: today };
    if (preset === 'yesterday') {
      var y = addDaysYMD(today, -1, tz);
      return { preset: 'yesterday', from: y, to: y };
    }
    if (preset === 'last7') {
      return { preset: 'last7', from: addDaysYMD(today, -6, tz), to: today };
    }
    if (preset === 'month') {
      var parts = parseYMD(today);
      var monthStart = parts.y + '-' + String(parts.m).padStart(2, '0') + '-01';
      return { preset: 'month', from: monthStart, to: today };
    }
    return { preset: 'custom', from: today, to: today };
  }

  function resolveContainer(container) {
    if (typeof container === 'string') return document.querySelector(container);
    return container;
  }

  function loadStored(storageKey) {
    try {
      var raw = localStorage.getItem(storageKey);
      if (!raw) return null;
      return JSON.parse(raw);
    } catch (_) {
      return null;
    }
  }

  function saveStored(storageKey, state) {
    try {
      localStorage.setItem(storageKey, JSON.stringify(state));
    } catch (_) { /* ignore */ }
  }

  /**
   * @param {string|Element} container
   * @param {{ storageKey?: string, timezone?: string, defaultPreset?: string, onChange?: function }} options
   */
  function createDateRangeFilter(container, options) {
    options = options || {};
    var el = resolveContainer(container);
    if (!el) throw new Error('FiscalUI.createDateRangeFilter: container not found');
    var tz = options.timezone || 'Europe/Lisbon';
    var storageKey = options.storageKey || 'fiscal_date_range';
    var onChange = typeof options.onChange === 'function' ? options.onChange : function () {};

    el.classList.add('fiscal-date-range');
    el.innerHTML =
      '<div class="fiscal-date-presets" role="group" aria-label="日期筛选"></div>' +
      '<div class="fiscal-date-custom hidden">' +
      '  <div class="field"><label>从</label><input type="date" class="fiscal-date-from" /></div>' +
      '  <div class="field"><label>至</label><input type="date" class="fiscal-date-to" /></div>' +
      '  <button type="button" class="secondary fiscal-date-apply">应用</button>' +
      '</div>' +
      '<p class="fiscal-date-error hidden"></p>';

    var presetRoot = el.querySelector('.fiscal-date-presets');
    var customPanel = el.querySelector('.fiscal-date-custom');
    var fromInput = el.querySelector('.fiscal-date-from');
    var toInput = el.querySelector('.fiscal-date-to');
    var errEl = el.querySelector('.fiscal-date-error');
    var state = { preset: options.defaultPreset || 'today', from: '', to: '' };

    var stored = loadStored(storageKey);
    if (stored && stored.preset) state.preset = stored.preset;
    if (stored && stored.preset === 'custom' && stored.from && stored.to) {
      state.from = stored.from;
      state.to = stored.to;
    }

    function syncRangeFromPreset() {
      if (state.preset === 'custom') return;
      var r = dateRangePresetToDates(state.preset, tz);
      state.from = r.from;
      state.to = r.to;
    }

    function paintPresets() {
      presetRoot.innerHTML = PRESETS.map(function (p) {
        var cls = p.id === state.preset ? ' active' : '';
        return '<button type="button" data-preset="' + p.id + '"' + cls + '>' + p.label + '</button>';
      }).join('');
      customPanel.classList.toggle('hidden', state.preset !== 'custom');
      if (state.preset === 'custom') {
        fromInput.value = state.from || '';
        toInput.value = state.to || '';
      }
      errEl.classList.add('hidden');
      errEl.textContent = '';
    }

    function validateCustom() {
      var from = fromInput.value;
      var to = toInput.value;
      if (!from || !to) {
        errEl.textContent = '请选择起止日期';
        errEl.classList.remove('hidden');
        return false;
      }
      if (from > to) {
        errEl.textContent = '起始日期不能晚于结束日期';
        errEl.classList.remove('hidden');
        return false;
      }
      state.from = from;
      state.to = to;
      errEl.classList.add('hidden');
      return true;
    }

    function emitChange() {
      saveStored(storageKey, { preset: state.preset, from: state.from, to: state.to });
      onChange(getRange());
    }

    function setPreset(preset) {
      state.preset = preset;
      syncRangeFromPreset();
      paintPresets();
      if (preset !== 'custom') emitChange();
    }

    function getRange() {
      if (state.preset !== 'custom') syncRangeFromPreset();
      return { preset: state.preset, from: state.from, to: state.to, timezone: tz };
    }

    presetRoot.addEventListener('click', function (e) {
      var btn = e.target.closest('[data-preset]');
      if (!btn) return;
      setPreset(btn.getAttribute('data-preset'));
    });

    el.querySelector('.fiscal-date-apply').addEventListener('click', function () {
      if (!validateCustom()) return;
      emitChange();
    });

    syncRangeFromPreset();
    paintPresets();
    if (state.preset !== 'custom') {
      saveStored(storageKey, { preset: state.preset, from: state.from, to: state.to });
    }

    return {
      getRange: getRange,
      setPreset: setPreset,
    };
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.createDateRangeFilter = createDateRangeFilter;
  global.FiscalUI.dateRangePresetToDates = dateRangePresetToDates;
})(typeof window !== 'undefined' ? window : globalThis);
