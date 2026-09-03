/* Fiscal Admin UI i18n — ONLY dictionary for Admin **screen** chrome: nav, settings, pay labels (scheme A).
 * Does NOT drive Mesa thermal ticket fixed labels; those follow payload.locale → printTicketLabels. */
(function (global) {
  'use strict';

  const BUNDLES = {
    zh: {
      'nav.home': '工作台',
      'nav.orders': '手工开票',
      'nav.bills': '收银账单',
      'nav.invoices': '发票',
      'nav.products': '商品',
      'nav.customers': '客户',
      'nav.settings': '设置',
      'settings.title': '设置',
      'settings.subtitle': '门店与设备',
      'settings.overview': '就绪概览',
      'settings.refresh': '刷新状态',
      'settings.nav.overview': '概览',
      'settings.nav.store': '门店与税务',
      'settings.nav.series': '系列与授权',
      'settings.nav.operators': '人员',
      'settings.nav.audit': '操作记录',
      'settings.nav.printers': '设备',
      'settings.nav.saft': '合规',
      'settings.nav.advanced': '高级',
      'lang.title': '界面语言',
      'lang.hint': '中文界面时发票为葡语；English / Português 时界面与发票同语言。认证句始终葡语。',
      'lang.zh': '中文',
      'lang.en': 'English',
      'lang.pt': 'Português',
      'lang.saved': '界面语言已保存',
      'lang.invoice_pt': '发票语言：葡语',
      'lang.invoice_en': '发票语言：英语',
      'home.title': '工作台',
      'pay.CASH': '现金',
      'pay.CARD': '刷卡',
      'pay.MBWAY': 'MB WAY',
      'pay.MULTIBANCO': 'Multibanco',
      'pay.MIXED': '混合',
      'pay.OTHER': '其他'
    },
    en: {
      'nav.home': 'Home',
      'nav.orders': 'Manual issue',
      'nav.bills': 'POS bills',
      'nav.invoices': 'Invoices',
      'nav.products': 'Products',
      'nav.customers': 'Customers',
      'nav.settings': 'Settings',
      'settings.title': 'Settings',
      'settings.subtitle': 'Store & devices',
      'settings.overview': 'Ready overview',
      'settings.refresh': 'Refresh',
      'settings.nav.overview': 'Overview',
      'settings.nav.store': 'Store & tax',
      'settings.nav.series': 'Series',
      'settings.nav.operators': 'Staff',
      'settings.nav.audit': 'Audit',
      'settings.nav.printers': 'Devices',
      'settings.nav.saft': 'Compliance',
      'settings.nav.advanced': 'Advanced',
      'lang.title': 'Interface language',
      'lang.hint': 'Chinese UI → Portuguese invoice; English / Português match the invoice. Certification line stays Portuguese.',
      'lang.zh': '中文',
      'lang.en': 'English',
      'lang.pt': 'Português',
      'lang.saved': 'Language saved',
      'lang.invoice_pt': 'Invoice language: Portuguese',
      'lang.invoice_en': 'Invoice language: English',
      'home.title': 'Home',
      'pay.CASH': 'Cash',
      'pay.CARD': 'Card',
      'pay.MBWAY': 'MB WAY',
      'pay.MULTIBANCO': 'Multibanco',
      'pay.MIXED': 'Mixed',
      'pay.OTHER': 'Other'
    },
    pt: {
      'nav.home': 'Início',
      'nav.orders': 'Emissão manual',
      'nav.bills': 'Contas POS',
      'nav.invoices': 'Faturas',
      'nav.products': 'Produtos',
      'nav.customers': 'Clientes',
      'nav.settings': 'Definições',
      'settings.title': 'Definições',
      'settings.subtitle': 'Loja e equipamentos',
      'settings.overview': 'Estado',
      'settings.refresh': 'Atualizar',
      'settings.nav.overview': 'Estado',
      'settings.nav.store': 'Loja e impostos',
      'settings.nav.series': 'Séries',
      'settings.nav.operators': 'Pessoal',
      'settings.nav.audit': 'Auditoria',
      'settings.nav.printers': 'Equipamentos',
      'settings.nav.saft': 'Conformidade',
      'settings.nav.advanced': 'Avançado',
      'lang.title': 'Idioma da interface',
      'lang.hint': 'UI chinês → fatura em português; English / Português alinhados com a fatura. Certificação sempre em português.',
      'lang.zh': '中文',
      'lang.en': 'English',
      'lang.pt': 'Português',
      'lang.saved': 'Idioma guardado',
      'lang.invoice_pt': 'Idioma da fatura: português',
      'lang.invoice_en': 'Idioma da fatura: inglês',
      'home.title': 'Início',
      'pay.CASH': 'Numerário',
      'pay.CARD': 'Cartão',
      'pay.MBWAY': 'MB WAY',
      'pay.MULTIBANCO': 'Multibanco',
      'pay.MIXED': 'Misto',
      'pay.OTHER': 'Outro'
    }
  };

  let current = 'zh';

  function normalize(raw) {
    const s = String(raw || '').toLowerCase();
    if (s === 'en' || s === 'english') return 'en';
    if (s === 'pt' || s.indexOf('portug') === 0) return 'pt';
    return 'zh';
  }

  function t(key) {
    const b = BUNDLES[current] || BUNDLES.zh;
    if (b[key] != null) return b[key];
    const zh = BUNDLES.zh[key];
    return zh != null ? zh : key;
  }

  function apply(root) {
    const scope = root || document;
    scope.querySelectorAll('[data-i18n]').forEach((el) => {
      const key = el.getAttribute('data-i18n');
      if (!key) return;
      const val = t(key);
      if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
        if (el.getAttribute('data-i18n-attr') === 'placeholder') el.placeholder = val;
        else el.value = val;
      } else {
        el.textContent = val;
      }
    });
    scope.querySelectorAll('[data-i18n-html]').forEach((el) => {
      const key = el.getAttribute('data-i18n-html');
      if (!key) return;
      el.innerHTML = t(key);
    });
    document.documentElement.lang = current === 'zh' ? 'zh-Hans' : current;
  }

  function setLocale(loc) {
    current = normalize(loc);
    apply(document);
    return current;
  }

  function getLocale() {
    return current;
  }

  global.FiscalAdminI18n = { t, apply, setLocale, getLocale, normalize };
})(typeof window !== 'undefined' ? window : globalThis);
