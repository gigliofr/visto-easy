import { STRINGS } from './strings.js';

const LANG_KEY = 'visto_easy_lang_v1';
const SUPPORTED = ['it', 'en'];

function normalize(value) {
  const v = String(value || '').toLowerCase().slice(0, 2);
  return SUPPORTED.includes(v) ? v : '';
}

let current = (() => {
  try {
    return normalize(localStorage.getItem(LANG_KEY)) || normalize(navigator.language) || 'it';
  } catch {
    return 'it';
  }
})();

export function getLang() {
  return current;
}

export function setLang(lang) {
  const next = normalize(lang) || 'it';
  current = next;
  try {
    localStorage.setItem(LANG_KEY, next);
  } catch {
    // archiviazione non disponibile: la scelta vale per la sessione
  }
  document.documentElement.lang = next;
  applyTranslations(document);
  window.dispatchEvent(new CustomEvent('app:lang-change', { detail: { lang: next } }));
}

// t restituisce il testo nella lingua corrente; stringa vuota se la chiave
// non esiste, così chi chiama può ripiegare su un valore proprio.
export function t(key, params = {}) {
  const entry = STRINGS[key];
  if (!entry) return '';
  const idx = current === 'en' ? 1 : 0;
  const template = entry[idx] || entry[0] || '';
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (_m, name) => (
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : ''
  ));
}

export function applyTranslations(root = document) {
  root.querySelectorAll('[data-i18n]').forEach((el) => {
    const v = t(el.dataset.i18n);
    if (v) el.textContent = v;
  });
  root.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
    const v = t(el.dataset.i18nPlaceholder);
    if (v) el.setAttribute('placeholder', v);
  });
  root.querySelectorAll('[data-i18n-aria]').forEach((el) => {
    const v = t(el.dataset.i18nAria);
    if (v) el.setAttribute('aria-label', v);
  });
}

export function initI18n() {
  document.documentElement.lang = current;
  applyTranslations(document);
}
