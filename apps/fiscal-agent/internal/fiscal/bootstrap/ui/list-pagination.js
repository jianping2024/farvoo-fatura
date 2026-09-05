/* Fiscal Admin list pagination — mirrors restaurant-ordering ListPaginationBar.tsx
 * API: FiscalUI.createListPaginationBar(container, options) -> { update, setDisabled, relabel }
 * Labels: options.getLabels() is the ONLY live copy path (re-read on paint/relabel).
 * Nav: first / prev / next / last — ONLY here (all 5 Admin lists).
 * Visible nav controls are icon-only; pageFirst/Prev/Next/Last are aria-label + title only.
 */
(function (global) {
  'use strict';

  var LIST_PAGE_SIZES = [10, 20];
  var LIST_DEFAULT_PAGE_SIZE = 10;

  /* ONLY pager chevron SVGs (restaurant ListPaginationBar paths). */
  var PAGER_ICON_SVG = {
    first:
      '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
      '<path d="M11 18l-6-6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
      '<path d="M18 18l-6-6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
      '</svg>',
    prev:
      '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
      '<path d="M15 18l-6-6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
      '</svg>',
    next:
      '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
      '<path d="M9 18l6-6-6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
      '</svg>',
    last:
      '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true">' +
      '<path d="M6 18l6-6-6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
      '<path d="M13 18l6-6-6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>' +
      '</svg>'
  };

  function totalPages(total, pageSize) {
    var size = Math.max(1, pageSize || LIST_DEFAULT_PAGE_SIZE);
    return Math.max(1, Math.ceil((total || 0) / size));
  }

  function createListPaginationBar(container, options) {
    options = options || {};
    function resolveLabels() {
      var extra = {};
      if (typeof options.getLabels === 'function') extra = options.getLabels() || {};
      else extra = options.labels || {};
      return {
        pageInfo: extra.pageInfo || '第 {page} / {totalPages} 页 · 共 {total} 条',
        pageSizeLabel: extra.pageSizeLabel || '每页',
        pageFirst: extra.pageFirst || '第一页',
        pagePrev: extra.pagePrev || '上一页',
        pageNext: extra.pageNext || '下一页',
        pageLast: extra.pageLast || '最后一页'
      };
    }
    var labels = resolveLabels();
    var pageInfoTpl = labels.pageInfo;
    var pageSizeLabel = labels.pageSizeLabel;
    var pageFirst = labels.pageFirst;
    var pagePrev = labels.pagePrev;
    var pageNext = labels.pageNext;
    var pageLast = labels.pageLast;
    var pageSizes = options.pageSizes || LIST_PAGE_SIZES.slice();
    var onPageChange = typeof options.onPageChange === 'function' ? options.onPageChange : function () {};
    var onPageSizeChange = typeof options.onPageSizeChange === 'function' ? options.onPageSizeChange : function () {};

    var root = typeof container === 'string' ? document.querySelector(container) : container;
    if (!root) throw new Error('list pagination container not found');

    root.className = (root.className ? root.className + ' ' : '') + 'fiscal-list-pagination';
    root.innerHTML =
      '<div class="fiscal-list-pagination__meta">' +
      '<p class="fiscal-list-pagination__info" data-role="info"></p>' +
      '<label class="fiscal-list-pagination__size">' +
      '<span data-role="size-label"></span>' +
      '<select data-role="size" aria-label="' + pageSizeLabel + '"></select>' +
      '</label>' +
      '</div>' +
      '<div class="fiscal-list-pagination__nav" data-role="nav">' +
      '<button type="button" class="secondary fiscal-list-pagination__icon-btn" data-role="first" aria-label="' +
      pageFirst +
      '" title="' +
      pageFirst +
      '">' +
      PAGER_ICON_SVG.first +
      '</button>' +
      '<button type="button" class="secondary fiscal-list-pagination__icon-btn" data-role="prev" aria-label="' +
      pagePrev +
      '" title="' +
      pagePrev +
      '">' +
      PAGER_ICON_SVG.prev +
      '</button>' +
      '<button type="button" class="secondary fiscal-list-pagination__icon-btn" data-role="next" aria-label="' +
      pageNext +
      '" title="' +
      pageNext +
      '">' +
      PAGER_ICON_SVG.next +
      '</button>' +
      '<button type="button" class="secondary fiscal-list-pagination__icon-btn" data-role="last" aria-label="' +
      pageLast +
      '" title="' +
      pageLast +
      '">' +
      PAGER_ICON_SVG.last +
      '</button>' +
      '</div>';

    var infoEl = root.querySelector('[data-role="info"]');
    var sizeLabelEl = root.querySelector('[data-role="size-label"]');
    var sizeEl = root.querySelector('[data-role="size"]');
    var navEl = root.querySelector('[data-role="nav"]');
    var firstBtn = root.querySelector('[data-role="first"]');
    var prevBtn = root.querySelector('[data-role="prev"]');
    var nextBtn = root.querySelector('[data-role="next"]');
    var lastBtn = root.querySelector('[data-role="last"]');

    sizeLabelEl.textContent = pageSizeLabel;
    sizeEl.innerHTML = pageSizes
      .map(function (size) {
        return '<option value="' + size + '">' + size + '</option>';
      })
      .join('');

    var state = { page: 1, totalPages: 1, total: 0, pageSize: LIST_DEFAULT_PAGE_SIZE, disabled: false };

    function paint() {
      var live = resolveLabels();
      pageInfoTpl = live.pageInfo;
      pageSizeLabel = live.pageSizeLabel;
      pageFirst = live.pageFirst;
      pagePrev = live.pagePrev;
      pageNext = live.pageNext;
      pageLast = live.pageLast;
      sizeLabelEl.textContent = pageSizeLabel;
      sizeEl.setAttribute('aria-label', pageSizeLabel);
      firstBtn.setAttribute('aria-label', pageFirst);
      firstBtn.setAttribute('title', pageFirst);
      prevBtn.setAttribute('aria-label', pagePrev);
      prevBtn.setAttribute('title', pagePrev);
      nextBtn.setAttribute('aria-label', pageNext);
      nextBtn.setAttribute('title', pageNext);
      lastBtn.setAttribute('aria-label', pageLast);
      lastBtn.setAttribute('title', pageLast);
      infoEl.textContent = pageInfoTpl
        .replace('{page}', String(state.page))
        .replace('{totalPages}', String(state.totalPages))
        .replace('{total}', String(state.total));
      sizeEl.value = String(state.pageSize);
      var atStart = state.disabled || state.page <= 1;
      var atEnd = state.disabled || state.page >= state.totalPages;
      firstBtn.disabled = atStart;
      prevBtn.disabled = atStart;
      nextBtn.disabled = atEnd;
      lastBtn.disabled = atEnd;
      navEl.style.display = state.totalPages > 1 ? '' : 'none';
      root.style.display = state.total === 0 ? 'none' : '';
      sizeEl.disabled = !!state.disabled;
    }

    firstBtn.addEventListener('click', function () {
      if (state.disabled || state.page <= 1) return;
      onPageChange(1);
    });
    prevBtn.addEventListener('click', function () {
      if (state.disabled || state.page <= 1) return;
      onPageChange(state.page - 1);
    });
    nextBtn.addEventListener('click', function () {
      if (state.disabled || state.page >= state.totalPages) return;
      onPageChange(state.page + 1);
    });
    lastBtn.addEventListener('click', function () {
      if (state.disabled || state.page >= state.totalPages) return;
      onPageChange(state.totalPages);
    });
    sizeEl.addEventListener('change', function () {
      var next = Number(sizeEl.value);
      if (pageSizes.indexOf(next) === -1) return;
      onPageSizeChange(next);
    });

    return {
      update: function (next) {
        state.page = next.page || 1;
        state.pageSize = next.pageSize || LIST_DEFAULT_PAGE_SIZE;
        state.total = next.total || 0;
        state.totalPages = totalPages(state.total, state.pageSize);
        paint();
      },
      setDisabled: function (disabled) {
        state.disabled = !!disabled;
        paint();
      },
      relabel: function () { paint(); },
    };
  }

  var FiscalUI = global.FiscalUI || {};
  FiscalUI.LIST_PAGE_SIZES = LIST_PAGE_SIZES;
  FiscalUI.LIST_DEFAULT_PAGE_SIZE = LIST_DEFAULT_PAGE_SIZE;
  FiscalUI.createListPaginationBar = createListPaginationBar;
  global.FiscalUI = FiscalUI;
})(typeof window !== 'undefined' ? window : globalThis);
