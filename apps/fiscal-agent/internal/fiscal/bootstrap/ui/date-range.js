/* Fiscal Admin date range — ONLY shared date filter (preset + native type=date from/to).
 * API: FiscalUI.createDateRangeFilter(container, options) -> { getRange, setPreset, relabel }
 *       FiscalUI.dateRangePresetToDates(preset, timezone)
 * Labels: options.getLabels() is the ONLY live copy path (re-read on paint/relabel).
 */
(function (global) {
  'use strict';

  var PRESET_IDS = ['today', 'yesterday', 'last7', 'month', 'custom'];

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

  function defaultLabels() {
    return {
      today: '今天',
      yesterday: '昨天',
      last7: '近7天',
      month: '本月',
      custom: '自定义',
      groupAria: '日期筛选',
      from: '从',
      to: '至',
      apply: '应用',
      errNeedRange: '请选择起止日期',
      errOrder: '起始日期不能晚于结束日期',
    };
  }

  /**
   * @param {string|Element} container
   * @param {{ storageKey?: string, timezone?: string, defaultPreset?: string, getLabels?: function, labels?: object, onChange?: function }} options
   */
  function createDateRangeFilter(container, options) {
    options = options || {};
    var el = resolveContainer(container);
    if (!el) throw new Error('FiscalUI.createDateRangeFilter: container not found');
    var tz = options.timezone || 'Europe/Lisbon';
    var storageKey = options.storageKey || 'fiscal_date_range';
    var onChange = typeof options.onChange === 'function' ? options.onChange : function () {};

    function resolveLabels() {
      var extra = {};
      if (typeof options.getLabels === 'function') extra = options.getLabels() || {};
      else extra = options.labels || {};
      var base = defaultLabels();
      var out = {};
      Object.keys(base).forEach(function (k) {
        out[k] = extra[k] != null && extra[k] !== '' ? extra[k] : base[k];
      });
      return out;
    }

    el.classList.add('fiscal-date-range');
    el.innerHTML =
      '<div class="fiscal-date-presets" role="group" data-role="presets"></div>' +
      '<div class="fiscal-date-custom hidden" data-role="custom">' +
      '  <div class="field"><label data-role="from-label"></label><input type="date" class="fiscal-date-from" data-role="from" /></div>' +
      '  <div class="field"><label data-role="to-label"></label><input type="date" class="fiscal-date-to" data-role="to" /></div>' +
      '  <button type="button" class="secondary fiscal-date-apply" data-role="apply"></button>' +
      '</div>' +
      '<p class="fiscal-date-error hidden" data-role="error"></p>';

    var presetRoot = el.querySelector('[data-role="presets"]');
    var customPanel = el.querySelector('[data-role="custom"]');
    var fromLabel = el.querySelector('[data-role="from-label"]');
    var toLabel = el.querySelector('[data-role="to-label"]');
    var fromInput = el.querySelector('[data-role="from"]');
    var toInput = el.querySelector('[data-role="to"]');
    var applyBtn = el.querySelector('[data-role="apply"]');
    var errEl = el.querySelector('[data-role="error"]');
    var state = { preset: options.defaultPreset || 'today', from: '', to: '' };
    var labels = resolveLabels();

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

    function paint() {
      labels = resolveLabels();
      presetRoot.setAttribute('aria-label', labels.groupAria);
      fromLabel.textContent = labels.from;
      toLabel.textContent = labels.to;
      applyBtn.textContent = labels.apply;
      presetRoot.innerHTML = PRESET_IDS.map(function (id) {
        var cls = id === state.preset ? ' active' : '';
        return '<button type="button" data-preset="' + id + '"' + cls + '>' + labels[id] + '</button>';
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
        errEl.textContent = labels.errNeedRange;
        errEl.classList.remove('hidden');
        return false;
      }
      if (from > to) {
        errEl.textContent = labels.errOrder;
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
      paint();
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

    applyBtn.addEventListener('click', function () {
      if (!validateCustom()) return;
      emitChange();
    });

    syncRangeFromPreset();
    paint();
    if (state.preset !== 'custom') {
      saveStored(storageKey, { preset: state.preset, from: state.from, to: state.to });
    }

    return {
      getRange: getRange,
      setPreset: setPreset,
      relabel: function () { paint(); },
    };
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.createDateRangeFilter = createDateRangeFilter;
  global.FiscalUI.dateRangePresetToDates = dateRangePresetToDates;
})(typeof window !== 'undefined' ? window : globalThis);
