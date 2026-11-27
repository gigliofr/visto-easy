import { html, icon } from './dom.js';
import { t } from './i18n.js';
import {
  statoLabel, statoTone, countryOptions, flag, displayToIso, isoToDisplay,
} from './domain.js';
import { fieldLabel, optionLabel, isRequired, isVisible } from './schema.js';

export function badge(label, tone = 'neutral') {
  return html`<span class="badge badge--${tone}">${label}</span>`;
}

export function statoBadge(stato) {
  return badge(statoLabel(stato), statoTone(stato));
}

export function progress(pct, tone = 'brand', label = '') {
  return html`<div class="progress progress--${tone}" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct}" aria-label="${label}"><div class="progress__bar" data-pct="${pct}"></div></div>`;
}

export function emptyState(title, text, action = '') {
  return html`<div class="empty"><div class="empty__icon">${icon('inbox', 28)}</div><p class="empty__title">${title}</p><p class="empty__text">${text}</p>${action}</div>`;
}

export function loading(text) {
  return html`<div class="loading" role="status"><span class="spinner" aria-hidden="true"></span>${text || t('common.loading')}</div>`;
}

export function errorBox(text, retryRoute = '') {
  return html`<div class="callout callout--danger" role="alert"><span class="callout__icon">${icon('alert', 20)}</span><div class="callout__body"><p class="callout__title">${t('common.loadError')}</p><p>${text}</p></div>${retryRoute ? html`<a class="btn btn--secondary" href="${retryRoute}">${t('common.retry')}</a>` : ''}</div>`;
}

export function countrySelectOptions(selected) {
  const sel = String(selected || '').toUpperCase();
  return html`<option value="">${t('common.select')}</option>${countryOptions().map((c) => html`<option value="${c.code}" ${c.code === sel ? 'selected' : ''}>${flag(c.code)} ${c.name}</option>`)}`;
}

// Campo del percorso guidato. values contiene i valori correnti (date in ISO).
export function formField(field, values, error = '') {
  const id = `f_${field.code.replace('.', '_')}`;
  const value = String(values[field.code] ?? '');
  const required = isRequired(field, values);
  const hint = t(`hint.${field.code}`);
  const errId = `${id}_err`;
  const hintId = `${id}_hint`;
  const describedBy = [hint ? hintId : '', error ? errId : ''].filter(Boolean).join(' ');
  const common = {
    id, name: field.code, invalid: error ? 'true' : 'false', describedBy,
  };
  let control;
  switch (field.type) {
    case 'select':
      control = html`<select id="${common.id}" name="${common.name}" data-code="${field.code}" aria-invalid="${common.invalid}" aria-describedby="${common.describedBy}" ${required ? 'required' : ''}>
        <option value="">${t('common.select')}</option>
        ${(field.options || []).map((o) => html`<option value="${o}" ${o === value ? 'selected' : ''}>${optionLabel(field.code, o)}</option>`)}
      </select>`;
      break;
    case 'country':
      control = html`<select id="${common.id}" name="${common.name}" data-code="${field.code}" aria-invalid="${common.invalid}" aria-describedby="${common.describedBy}" ${required ? 'required' : ''}>${countrySelectOptions(value)}</select>`;
      break;
    case 'textarea':
      control = html`<textarea id="${common.id}" name="${common.name}" data-code="${field.code}" rows="3" aria-invalid="${common.invalid}" aria-describedby="${common.describedBy}" ${required ? 'required' : ''}>${value}</textarea>`;
      break;
    case 'date':
      control = html`<input id="${common.id}" name="${common.name}" data-code="${field.code}" data-type="date" type="text" inputmode="numeric" placeholder="${t('common.datePlaceholder')}" autocomplete="${field.autocomplete || 'off'}" value="${isoToDisplay(value)}" aria-invalid="${common.invalid}" aria-describedby="${common.describedBy}" ${required ? 'required' : ''}>`;
      break;
    default: {
      const type = ['email', 'number', 'tel'].includes(field.type) ? field.type : 'text';
      control = html`<input id="${common.id}" name="${common.name}" data-code="${field.code}" type="${type}" ${type === 'number' ? 'inputmode="numeric" min="0"' : ''} autocomplete="${field.autocomplete || 'off'}" value="${value}" aria-invalid="${common.invalid}" aria-describedby="${common.describedBy}" ${field.computed ? 'readonly' : ''} ${required ? 'required' : ''}>`;
    }
  }
  const tag = required ? t('form.required') : t('form.optional');
  return html`<div class="field ${field.wide ? 'field--wide' : ''} ${isVisible(field, values) ? '' : 'is-hidden'}" data-field="${field.code}">
    <label for="${id}">${fieldLabel(field.code)} <span class="field__tag ${required ? 'field__tag--req' : ''}">${field.computed ? t('form.computed') : tag}</span></label>
    ${control}
    ${hint ? html`<p class="field__hint" id="${hintId}">${hint}</p>` : ''}
    ${error ? html`<p class="field__error" id="${errId}">${icon('alert', 14)} ${error}</p>` : ''}
  </div>`;
}

// Legge un valore da un controllo del percorso: le date tornano in ISO.
export function readControl(el) {
  const v = String(el.value ?? '');
  if (el.dataset.type === 'date') {
    const iso = displayToIso(v);
    return iso === null ? v.trim() : iso;
  }
  return v;
}

export function pageHeader(title, subtitle = '', actions = '') {
  return html`<div class="page-head"><div>${subtitle ? html`<p class="page-head__eyebrow">${subtitle}</p>` : ''}<h1>${title}</h1></div>${actions ? html`<div class="page-head__actions">${actions}</div>` : ''}</div>`;
}

export function definitionList(rows) {
  return html`<dl class="deflist">${rows.filter((r) => r).map(([k, v]) => html`<div class="deflist__row"><dt>${k}</dt><dd>${v === '' || v === null || v === undefined ? '-' : v}</dd></div>`)}</dl>`;
}
