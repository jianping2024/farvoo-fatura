/* Fiscal Admin date range — presets + always-on from/to DatePickers (no expand layout shift).
 * API: FiscalUI.createDateRangeFilter(container, options) -> { getRange, setPreset, relabel }
 *       FiscalUI.dateRangePresetToDates(preset, timezone)
 * Labels: options.getLabels() is the ONLY live copy path (re-read on paint/relabel).
 * Pickers: ONLY FiscalUI.createDatePicker (mesa DatePicker analogue).
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
    } catch (_) {
      /* ignore */
    }
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

  function createDateRangeFilter(container, options) {
    options = options || {};
    var el = resolveContainer(container);
    if (!el) throw new Error('FiscalUI.createDateRangeFilter: container not found');
    if (!global.FiscalUI || typeof global.FiscalUI.createDatePicker !== 'function') {
      throw new Error('FiscalUI.createDateRangeFilter requires FiscalUI.createDatePicker');
    }
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
      '<div class="fiscal-date-custom" data-role="custom">' +
      '  <div class="fiscal-date-picker-mount" data-role="from-mount"></div>' +
      '  <span class="fiscal-date-sep" aria-hidden="true">—</span>' +
      '  <div class="fiscal-date-picker-mount" data-role="to-mount"></div>' +
      '  <button type="button" class="secondary fiscal-date-apply" data-role="apply"></button>' +
      '</div>' +
      '<p class="fiscal-date-error hidden" data-role="error"></p>';

    var presetRoot = el.querySelector('[data-role="presets"]');
    var customPanel = el.querySelector('[data-role="custom"]');
    var fromMount = el.querySelector('[data-role="from-mount"]');
    var toMount = el.querySelector('[data-role="to-mount"]');
    var applyBtn = el.querySelector('[data-role="apply"]');
    var errEl = el.querySelector('[data-role="error"]');
    var state = { preset: options.defaultPreset || 'today', from: '', to: '' };
    var labels = resolveLabels();
    var syncingPickers = false;

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

    function getLocale() {
      if (typeof options.getLocale === 'function') return options.getLocale();
      return 'zh';
    }

    var fromPicker = global.FiscalUI.createDatePicker(fromMount, {
      getLocale: getLocale,
      getPlaceholder: function () {
        return resolveLabels().from;
      },
      onChange: function (ymd) {
        if (syncingPickers) return;
        state.preset = 'custom';
        state.from = ymd;
        paintPresets();
        syncCustomEnabled();
      },
    });
    var toPicker = global.FiscalUI.createDatePicker(toMount, {
      getLocale: getLocale,
      getPlaceholder: function () {
        return resolveLabels().to;
      },
      onChange: function (ymd) {
        if (syncingPickers) return;
        state.preset = 'custom';
        state.to = ymd;
        paintPresets();
        syncCustomEnabled();
      },
    });

    function syncPickersFromState() {
      syncingPickers = true;
      fromPicker.setValue(state.from);
      toPicker.setValue(state.to);
      syncingPickers = false;
    }

    function syncCustomEnabled() {
      var isCustom = state.preset === 'custom';
      customPanel.classList.toggle('is-custom', isCustom);
      applyBtn.disabled = !isCustom;
      applyBtn.classList.toggle('hidden', !isCustom);
    }

    function paintPresets() {
      labels = resolveLabels();
      presetRoot.setAttribute('aria-label', labels.groupAria);
      applyBtn.textContent = labels.apply;
      presetRoot.innerHTML = PRESET_IDS.map(function (id) {
        var cls = id === state.preset ? ' active' : '';
        return (
          '<button type="button" class="fiscal-date-preset' +
          cls +
          '" data-preset="' +
          id +
          '">' +
          labels[id] +
          '</button>'
        );
      }).join('');
    }

    function paint() {
      paintPresets();
      syncRangeFromPreset();
      syncPickersFromState();
      syncCustomEnabled();
      errEl.classList.add('hidden');
      errEl.textContent = '';
      fromPicker.relabel();
      toPicker.relabel();
    }

    function validateCustom() {
      var from = fromPicker.getValue();
      var to = toPicker.getValue();
      labels = resolveLabels();
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
      if (preset !== 'custom') {
        syncRangeFromPreset();
      } else if (!state.from || !state.to) {
        var fallback = dateRangePresetToDates('today', tz);
        if (!state.from) state.from = fallback.from;
        if (!state.to) state.to = fallback.to;
      }
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
      relabel: function () {
        paint();
      },
    };
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.createDateRangeFilter = createDateRangeFilter;
  global.FiscalUI.dateRangePresetToDates = dateRangePresetToDates;
})(typeof window !== 'undefined' ? window : globalThis);
