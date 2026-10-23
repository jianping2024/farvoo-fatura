/* Shared printer-mapping wizard helpers (configure + setup). */
(function (global) {
  'use strict';

  var DEFAULT_TEST_BLOCK_IDS = [
    'testStationLabel', 'testStation', 'testPrintLocaleLabel', 'testPrintLocale', 'testPrintBtn', 'testErr', 'testSent',
    'testConfirmBlock', 'testDone',
  ];

  function t(ui, key) {
    return ui && ui[key] ? ui[key] : key;
  }

  function fmt(ui, key, val) {
    return t(ui, key).replace('%s', val);
  }

  function fmt2(ui, key, a, b) {
    var s = t(ui, key);
    s = s.replace('%s', a);
    s = s.replace('%s', b);
    return s;
  }

  function isStationMappingConflict(res) {
    return !!(res && res.j && res.j.error_code === 'station_mapping_conflict');
  }

  function isAgentAuthFailed(res) {
    if (!res) return false;
    if (res.status === 401) return true;
    var j = res.j || {};
    if (j.error_code === 'agent_revoked' || j.error_code === 'unauthorized') return true;
    return /HTTP 401|\bunauthorized\b/i.test(String(j.error || ''));
  }

  function formatSaveError(ui, payload, status) {
    var j = payload || {};
    if (isAgentAuthFailed({ status: status || 0, j: j })) {
      return t(ui, 'save_auth_revoked');
    }
    if (j.error_code === 'station_mapping_conflict' && Array.isArray(j.conflicts) && j.conflicts.length) {
      var lines = j.conflicts.map(function (c) {
        var dev = (c.other_device_label && String(c.other_device_label).trim()) ||
          String(c.other_device_id || '').slice(0, 8);
        return fmt2(ui, 'save_station_conflict_line', c.station_label || c.station_id || '?', dev);
      });
      return t(ui, 'save_station_conflict_title') + '\n' + lines.join('\n') +
        '\n' + t(ui, 'save_station_conflict_takeover_hint');
    }
    return j.error || t(ui, 'save_failed');
  }

  /** Non-empty station_id -> printer target from .station-row / .station-printer selects. */
  function collectFromRows(root, opts) {
    opts = opts || {};
    var out = {};
    var scope = root || document;
    scope.querySelectorAll('.station-printer').forEach(function (sel) {
      var row = sel.closest('.station-row');
      if (!row) return;
      var sid = row.dataset.stationId;
      if (!sid) return;
      if (opts.draft) opts.draft[sid] = sel.value;
      if (sel.value) out[sid] = sel.value;
    });
    return out;
  }

  function mappingCount(maps) {
    return Object.keys(maps || {}).length;
  }

  function isMappingCleared(maps) {
    return mappingCount(maps) === 0;
  }

  function saveOkMessage(ui, maps) {
    return isMappingCleared(maps) ? t(ui, 'save_cleared_ok') : t(ui, 'save_ok');
  }

  function postSetup(stationPrinters, opts) {
    opts = opts || {};
    var body = { station_printers: stationPrinters || {} };
    if (opts.forceTakeover) body.force_takeover = true;
    return fetch('/api/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then(function (r) {
      return r.json().then(function (j) {
        return { ok: r.ok, status: r.status, j: j };
      });
    });
  }

  /** Thermal text_encoding choices; 'auto' first (recommended default). */
  var TEXT_ENCODING_OPTIONS = ['auto', 'han_firmware', 'utf8', 'latin'];

  /** Server sends normalizeTextEncoding(); anything unknown (legacy bitmap, '') shows as auto. */
  function normalizeTextEncodingValue(v) {
    v = String(v || '').trim().toLowerCase();
    return TEXT_ENCODING_OPTIONS.indexOf(v) < 0 ? 'auto' : v;
  }

  /** Fill a <select> with text_encoding_* labels and select current (unknown -> auto). */
  function fillTextEncodingSelect(sel, ui, current) {
    if (!sel) return;
    var cur = normalizeTextEncodingValue(current);
    sel.innerHTML = '';
    TEXT_ENCODING_OPTIONS.forEach(function (v) {
      var o = document.createElement('option');
      o.value = v;
      o.textContent = t(ui, 'text_encoding_' + v);
      if (v === cur) o.selected = true;
      sel.appendChild(o);
    });
  }

  /** Persist text_encoding via /api/ui-locale (saves config + applyThermalEncodingFromConfig). */
  function postTextEncoding(enc) {
    return fetch('/api/ui-locale', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text_encoding: normalizeTextEncodingValue(enc) }),
    }).then(function (r) {
      return r.json().then(function (j) {
        return { ok: r.ok, status: r.status, j: j };
      });
    });
  }

  /**
   * Save-on-change for the text_encoding <select>. opts: getUI(), getCurrent(), setCurrent(v),
   * errEl, okEl, log(msg) optional. Reverts the select when the save fails.
   */
  function bindTextEncodingSelect(sel, opts) {
    if (!sel) return;
    sel.addEventListener('change', function () {
      var ui = opts.getUI();
      var prev = opts.getCurrent();
      if (opts.errEl) opts.errEl.textContent = '';
      if (opts.okEl) opts.okEl.textContent = '';
      sel.disabled = true;
      if (opts.log) opts.log('phase text_encoding save value=' + sel.value);
      function fail(msg) {
        if (opts.errEl) opts.errEl.textContent = msg;
        fillTextEncodingSelect(sel, ui, prev);
      }
      postTextEncoding(sel.value)
        .then(function (res) {
          if (!res.ok) {
            fail((res.j && res.j.error) || t(ui, 'save_failed'));
            return;
          }
          opts.setCurrent(res.j.text_encoding);
          fillTextEncodingSelect(sel, ui, res.j.text_encoding);
          if (opts.okEl) opts.okEl.textContent = t(ui, 'text_encoding_saved');
        })
        .catch(function () { fail(t(ui, 'pair_service_unreachable')); })
        .finally(function () { sel.disabled = false; });
    });
  }

  function setTakeoverVisible(btn, show) {
    if (!btn) return;
    btn.classList.toggle('hidden', !show);
    if (!show) btn.onclick = null;
  }

  function setTestBlockVisible(show, ids) {
    (ids || DEFAULT_TEST_BLOCK_IDS).forEach(function (id) {
      var el = document.getElementById(id);
      if (el) el.classList.toggle('hidden', !show);
    });
  }

  /** Display label for a discovered/saved printer addr (LAN / winspool / raw). */
  function printerDisplayName(p, ui) {
    var addr = (p && p.addr) || '';
    if (addr.indexOf('tcp:') === 0) return fmt(ui, 'printer_lan_label', addr.slice(4));
    if (addr.indexOf('winspool:') === 0) return addr.slice(9);
    return (p && p.label) || addr;
  }

  function trimStationName(v) {
    return String(v == null ? '' : v).trim();
  }

  /**
   * Merge API row with optional namesById (configure keeps id-only rows for saved maps).
   * Sole name resolution for mapping UI — do not reimplement in configure/setup HTML.
   */
  function resolveStationNames(station, namesById) {
    var s = station || {};
    var id = trimStationName(s.id);
    var known = namesById && id ? namesById[id] : null;
    if (known) {
      return {
        id: id,
        name_zh: known.name_zh,
        name_en: known.name_en,
        name_pt: known.name_pt,
      };
    }
    return s;
  }

  /**
   * Sole primary station title on printer-mapping pages (configure + setup + test dropdown).
   * Follows agent UI locale: zh → name_zh|pt|en; pt → name_pt|en|zh; else → name_en|pt|zh.
   */
  function stationPrimaryLabel(station, uiLocale, namesById) {
    var s = resolveStationNames(station, namesById);
    var zh = trimStationName(s.name_zh);
    var en = trimStationName(s.name_en);
    var pt = trimStationName(s.name_pt);
    var id = trimStationName(s.id) || '?';
    var loc = String(uiLocale || '').trim().toLowerCase();
    if (loc === 'zh') return zh || pt || en || id;
    if (loc === 'pt') return pt || en || zh || id;
    return en || pt || zh || id;
  }

  /**
   * Sole alt line under the title: other filled languages that differ from title.
   * Format: "ZH 后厨 · EN Kitchen · PT Cozinha" (omit empty / same-as-title).
   */
  function stationAltLine(station, title, namesById) {
    var s = resolveStationNames(station, namesById);
    var titleNorm = trimStationName(title);
    var parts = [];
    [
      ['ZH', s.name_zh],
      ['EN', s.name_en],
      ['PT', s.name_pt],
    ].forEach(function (pair) {
      var value = trimStationName(pair[1]);
      if (!value || value === titleNorm) return;
      parts.push(pair[0] + ' ' + value);
    });
    return parts.join(' · ');
  }

  /** Unique printer addrs from a station_printers map. */
  function savedPrinterAddrs(maps) {
    maps = maps || {};
    var seen = {};
    var out = [];
    Object.keys(maps).forEach(function (sid) {
      var addr = String(maps[sid] || '').trim();
      if (!addr || seen[addr]) return;
      seen[addr] = true;
      out.push(addr);
    });
    return out;
  }

  /**
   * Merge scanned printers with saved addrs for one prefix (tcp:/winspool:/null=any).
   * Returns { list, addedFromSaved, skippedDup }.
   */
  function mergePrinterGroup(scanned, savedAddrs, prefix) {
    var byAddr = {};
    (scanned || []).forEach(function (p) {
      var a = String(p && p.addr || '').trim();
      if (a) byAddr[a] = p;
    });
    var addedFromSaved = 0;
    var skippedDup = 0;
    (savedAddrs || []).forEach(function (addr) {
      if (prefix != null && addr.indexOf(prefix) !== 0) return;
      if (byAddr[addr]) {
        skippedDup++;
        return;
      }
      byAddr[addr] = { addr: addr };
      addedFromSaved++;
    });
    return {
      list: Object.keys(byAddr).map(function (k) { return byAddr[k]; }),
      addedFromSaved: addedFromSaved,
      skippedDup: skippedDup
    };
  }

  global.MesaWizardUI = {
    DEFAULT_TEST_BLOCK_IDS: DEFAULT_TEST_BLOCK_IDS,
    t: t,
    fmt: fmt,
    fmt2: fmt2,
    formatSaveError: formatSaveError,
    isStationMappingConflict: isStationMappingConflict,
    isAgentAuthFailed: isAgentAuthFailed,
    collectFromRows: collectFromRows,
    mappingCount: mappingCount,
    isMappingCleared: isMappingCleared,
    saveOkMessage: saveOkMessage,
    postSetup: postSetup,
    setTakeoverVisible: setTakeoverVisible,
    setTestBlockVisible: setTestBlockVisible,
    printerDisplayName: printerDisplayName,
    resolveStationNames: resolveStationNames,
    stationPrimaryLabel: stationPrimaryLabel,
    stationAltLine: stationAltLine,
    savedPrinterAddrs: savedPrinterAddrs,
    mergePrinterGroup: mergePrinterGroup,
    TEXT_ENCODING_OPTIONS: TEXT_ENCODING_OPTIONS,
    normalizeTextEncodingValue: normalizeTextEncodingValue,
    fillTextEncodingSelect: fillTextEncodingSelect,
    postTextEncoding: postTextEncoding,
    bindTextEncodingSelect: bindTextEncodingSelect,
  };
})(typeof window !== 'undefined' ? window : globalThis);
