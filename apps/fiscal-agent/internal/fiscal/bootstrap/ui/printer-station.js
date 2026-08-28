(function (global) {
  'use strict';

  function printerDisplayName(addr) {
    var raw = (addr || '').trim();
    if (raw.indexOf('tcp:') === 0) return raw.slice(4);
    if (raw.indexOf('winspool:') === 0) return raw.slice(9);
    return raw;
  }

  /** Sole formatter for printer station <option> labels in Fiscal Admin. */
  function formatPrinterStationOption(station) {
    var name = ((station && station.label) || '').trim();
    if (!name && station && station.id) {
      var id = String(station.id);
      name = id.length > 8 ? id.slice(0, 8) + '…' : id;
    }
    var printer = printerDisplayName(station && station.printer);
    if (printer) return name + ' · ' + printer;
    return name;
  }

  global.FiscalUI = global.FiscalUI || {};
  global.FiscalUI.printerDisplayName = printerDisplayName;
  global.FiscalUI.formatPrinterStationOption = formatPrinterStationOption;
})(typeof window !== 'undefined' ? window : globalThis);
