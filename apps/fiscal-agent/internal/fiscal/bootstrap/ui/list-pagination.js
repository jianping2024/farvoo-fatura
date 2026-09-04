/* Fiscal Admin list pagination — mirrors restaurant-ordering ListPaginationBar.tsx
 * API: FiscalUI.createListPaginationBar(container, options) -> { update, setDisabled, relabel }
 * Labels: options.getLabels() is the ONLY live copy path (re-read on paint/relabel).
 */
(function (global) {
  'use strict';

  var LIST_PAGE_SIZES = [10, 20];
  var LIST_DEFAULT_PAGE_SIZE = 10;

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
        pagePrev: extra.pagePrev || '上一页',
        pageNext: extra.pageNext || '下一页'
      };
    }
    var labels = resolveLabels();
    var pageInfoTpl = labels.pageInfo;
    var pageSizeLabel = labels.pageSizeLabel;
    var pagePrev = labels.pagePrev;
    var pageNext = labels.pageNext;
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
      '<button type="button" class="secondary" data-role="prev">' + pagePrev + '</button>' +
      '<button type="button" class="secondary" data-role="next">' + pageNext + '</button>' +
      '</div>';

    var infoEl = root.querySelector('[data-role="info"]');
    var sizeLabelEl = root.querySelector('[data-role="size-label"]');
    var sizeEl = root.querySelector('[data-role="size"]');
    var navEl = root.querySelector('[data-role="nav"]');
    var prevBtn = root.querySelector('[data-role="prev"]');
    var nextBtn = root.querySelector('[data-role="next"]');

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
      pagePrev = live.pagePrev;
      pageNext = live.pageNext;
      sizeLabelEl.textContent = pageSizeLabel;
      sizeEl.setAttribute('aria-label', pageSizeLabel);
      prevBtn.textContent = pagePrev;
      nextBtn.textContent = pageNext;
      infoEl.textContent = pageInfoTpl
        .replace('{page}', String(state.page))
        .replace('{totalPages}', String(state.totalPages))
        .replace('{total}', String(state.total));
      sizeEl.value = String(state.pageSize);
      prevBtn.disabled = state.disabled || state.page <= 1;
      nextBtn.disabled = state.disabled || state.page >= state.totalPages;
      navEl.style.display = state.totalPages > 1 ? '' : 'none';
      root.style.display = state.total === 0 ? 'none' : '';
      sizeEl.disabled = !!state.disabled;
    }

    prevBtn.addEventListener('click', function () {
      if (state.disabled || state.page <= 1) return;
      onPageChange(state.page - 1);
    });
    nextBtn.addEventListener('click', function () {
      if (state.disabled || state.page >= state.totalPages) return;
      onPageChange(state.page + 1);
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
