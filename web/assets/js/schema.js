// Schema dei dati di una pratica visto. I codici (1.01, 2.03, ...) sono gli
// stessi usati finora: restano la chiave con cui i dati vengono salvati in
// dati_anagrafici.attributi_compilazione (tutto tranne 2.x) e in
// dati_passaporto (2.x), quindi le pratiche esistenti restano leggibili.
import { SCHENGEN, displayToIso, isoToDate } from './domain.js';
import { t } from './i18n.js';

const up = (v) => String(v || '').toUpperCase();

export const FIELDS = [
  // Passo 1: viaggio
  { code: '3.01', step: 1, type: 'country', required: true },
  { code: '3.02', step: 1, type: 'select', required: true, options: ['TURISMO', 'LAVORO', 'STUDIO', 'TRANSITO', 'RICONGIUNGIMENTO', 'DIPLOMATICO', 'ALTRO'] },
  { code: '3.03', step: 1, type: 'select', required: true, options: ['A', 'B', 'C', 'D'] },
  { code: '3.04', step: 1, type: 'select', required: true, options: ['SINGOLO', 'DOPPIO', 'MULTIPLO'] },
  { code: '3.05', step: 1, type: 'date', required: true },
  { code: '3.06', step: 1, type: 'date', required: true },
  { code: '3.07', step: 1, type: 'number', required: true, computed: true },
  { code: '3.08', step: 1, type: 'country', required: true },
  { code: '3.09', step: 1, type: 'country', required: true },
  { code: '3.10', step: 1, type: 'textarea', required: true, wide: true },
  { code: '3.11', step: 1, type: 'textarea', required: false, wide: true, requiredIf: (v) => up(v['3.03']) === 'C' && SCHENGEN.has(up(v['3.01'])) },

  // Passo 2: dati personali
  { code: '1.01', step: 2, type: 'text', required: true, autocomplete: 'family-name' },
  { code: '1.02', step: 2, type: 'text', required: true, autocomplete: 'given-name' },
  { code: '1.03', step: 2, type: 'date', required: true, autocomplete: 'bday' },
  { code: '1.04', step: 2, type: 'text', required: true },
  { code: '1.05', step: 2, type: 'country', required: true },
  { code: '1.06', step: 2, type: 'country', required: false },
  { code: '1.07', step: 2, type: 'select', required: true, options: ['M', 'F', 'X'] },
  { code: '1.08', step: 2, type: 'select', required: true, options: ['NUBILE_CELIBE', 'CONIUGATO', 'DIVORZIATO', 'VEDOVO'] },
  { code: '1.09', step: 2, type: 'text', required: false, requiredIf: (v) => ['CONIUGATO', 'DIVORZIATO'].includes(up(v['1.08'])), showIf: (v) => ['CONIUGATO', 'DIVORZIATO', 'VEDOVO'].includes(up(v['1.08'])) },
  { code: '1.10', step: 2, type: 'text', required: true, autocomplete: 'organization-title' },
  { code: '1.11', step: 2, type: 'text', required: true, autocomplete: 'organization' },
  { code: '1.12', step: 2, type: 'textarea', required: true, wide: true, autocomplete: 'street-address' },
  { code: '1.13', step: 2, type: 'email', required: true, autocomplete: 'email' },

  // Passo 3: passaporto
  { code: '2.01', step: 3, type: 'select', required: true, options: ['ORDINARIO', 'SERVIZIO', 'DIPLOMATICO', 'EMERGENZA'] },
  { code: '2.02', step: 3, type: 'text', required: true, transform: 'upper' },
  { code: '2.03', step: 3, type: 'country', required: true },
  { code: '2.04', step: 3, type: 'text', required: true },
  { code: '2.05', step: 3, type: 'date', required: true },
  { code: '2.06', step: 3, type: 'date', required: true },
  { code: '2.07', step: 3, type: 'textarea', required: false, optional: true, wide: true, transform: 'upper' },
  { code: '2.08', step: 3, type: 'text', required: false, optional: true },
  { code: '2.09', step: 3, type: 'number', required: false, optional: true },

  // Passo 4: soggiorno e mezzi
  { code: '4.01', step: 4, type: 'select', required: true, options: ['HOTEL', 'ABITAZIONE_PRIVATA', 'OSTELLO', 'RESIDENZA_UNIVERSITARIA', 'ALTRO'] },
  { code: '4.02', step: 4, type: 'textarea', required: true, wide: true },
  { code: '4.03', step: 4, type: 'text', required: false },
  { code: '4.04', step: 4, type: 'text', required: false, requiredIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA', showIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA' },
  { code: '4.05', step: 4, type: 'textarea', required: false, wide: true, requiredIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA', showIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA' },
  { code: '4.06', step: 4, type: 'tel', required: false, requiredIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA', showIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA' },
  { code: '4.07', step: 4, type: 'select', required: false, options: ['FAMILIARE', 'AMICO', 'SOCIO_COMMERCIALE', 'ALTRO'], requiredIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA', showIf: (v) => up(v['4.01']) === 'ABITAZIONE_PRIVATA' },
  { code: '4.08', step: 4, type: 'text', required: false },
  { code: '5.01', step: 4, type: 'select', required: true, options: ['FONDI_PROPRI', 'DATORE_LAVORO', 'GARANTE', 'BORSA_STUDIO', 'ALTRO'] },
  { code: '5.02', step: 4, type: 'text', required: true },
  { code: '5.03', step: 4, type: 'text', required: false, requiredIf: (v) => up(v['5.01']) === 'FONDI_PROPRI', showIf: (v) => up(v['5.01']) === 'FONDI_PROPRI' },
  { code: '5.04', step: 4, type: 'text', required: false, requiredIf: (v) => up(v['5.01']) === 'FONDI_PROPRI', showIf: (v) => up(v['5.01']) === 'FONDI_PROPRI' },
  { code: '5.05', step: 4, type: 'select', required: false, options: ['RICHIEDENTE', 'GARANTE', 'AZIENDA', 'ALTRO'] },
  { code: '5.06', step: 4, type: 'text', required: false, optional: true },
  { code: '5.07', step: 4, type: 'text', required: false, optional: true },

  // Passo 5: precedenti e dichiarazioni
  { code: '6.01', step: 5, type: 'textarea', required: true, wide: true },
  { code: '6.02', step: 5, type: 'textarea', required: true, wide: true },
  { code: '6.03', step: 5, type: 'textarea', required: true, wide: true },
  { code: '6.04', step: 5, type: 'textarea', required: true, wide: true },
  { code: '6.05', step: 5, type: 'textarea', required: false, optional: true, wide: true },
  { code: '6.06', step: 5, type: 'textarea', required: false, optional: true, wide: true },
  { code: '6.07', step: 5, type: 'textarea', required: false, optional: true, wide: true },
  { code: '6.08', step: 5, type: 'text', required: false, optional: true },
  { code: '6.09', step: 5, type: 'text', required: false, optional: true },
  { code: '7.01', step: 5, type: 'textarea', required: true, wide: true },
  { code: '7.02', step: 5, type: 'textarea', required: true, wide: true },
  { code: '7.03', step: 5, type: 'textarea', required: true, wide: true },
  { code: '7.05', step: 5, type: 'text', required: false, optional: true },

  // Campi compilati dall'operatore: non compaiono nel percorso del richiedente.
  { code: '7.04', step: 0, type: 'text', required: false, operator: true },
  { code: '7.06', step: 0, type: 'text', required: false, operator: true },
  { code: '7.07', step: 0, type: 'textarea', required: false, operator: true },
];

export const FIELD_BY_CODE = Object.fromEntries(FIELDS.map((f) => [f.code, f]));

export const STEPS = [
  { n: 1, key: 'viaggio' },
  { n: 2, key: 'persona' },
  { n: 3, key: 'passaporto' },
  { n: 4, key: 'soggiorno' },
  { n: 5, key: 'precedenti' },
  { n: 6, key: 'documenti' },
  { n: 7, key: 'riepilogo' },
];

export const SECTIONS = [
  { key: 'viaggio', codes: FIELDS.filter((f) => f.code.startsWith('3.')).map((f) => f.code) },
  { key: 'persona', codes: FIELDS.filter((f) => f.code.startsWith('1.')).map((f) => f.code) },
  { key: 'passaporto', codes: FIELDS.filter((f) => f.code.startsWith('2.')).map((f) => f.code) },
  { key: 'alloggio', codes: FIELDS.filter((f) => f.code.startsWith('4.')).map((f) => f.code) },
  { key: 'mezzi', codes: FIELDS.filter((f) => f.code.startsWith('5.')).map((f) => f.code) },
  { key: 'storico', codes: FIELDS.filter((f) => f.code.startsWith('6.')).map((f) => f.code) },
  { key: 'sicurezza', codes: FIELDS.filter((f) => f.code.startsWith('7.')).map((f) => f.code) },
];

export function fieldLabel(code) {
  return t(`f.${code}`) || code;
}

export function optionLabel(code, value) {
  return t(`opt.${code}.${value}`) || String(value || '').replaceAll('_', ' ');
}

export function isVisible(field, values) {
  return typeof field.showIf !== 'function' || field.showIf(values);
}

export function isRequired(field, values) {
  if (field.operator) return false;
  if (field.required) return true;
  if (typeof field.requiredIf === 'function' && field.requiredIf(values)) return true;
  if (SCHENGEN.has(up(values['3.01'])) && field.code === '3.11') return true;
  return false;
}

// Valori da una pratica salvata (dati_anagrafici + dati_passaporto).
export function valuesFromPratica(p) {
  const values = {};
  const attrs = p?.dati_anagrafici?.attributi_compilazione || {};
  Object.entries(attrs).forEach(([k, v]) => { values[k] = String(v ?? ''); });
  Object.entries(p?.dati_passaporto || {}).forEach(([k, v]) => { values[k] = String(v ?? ''); });
  if (!values['3.01'] && p?.paese_dest) values['3.01'] = p.paese_dest;
  if (!values['3.02'] && p?.tipo_visto) values['3.02'] = p.tipo_visto;
  return values;
}

// Corpo per POST/PATCH della pratica.
export function payloadFromValues(values) {
  const attributi = {};
  const passaporto = {};
  Object.entries(values).forEach(([code, v]) => {
    if (!FIELD_BY_CODE[code]) return;
    const value = String(v ?? '').trim();
    if (code.startsWith('2.')) passaporto[code] = value;
    else attributi[code] = value;
  });
  return {
    tipo_visto: values['3.02'] || '',
    paese_dest: values['3.01'] || '',
    dati_anagrafici: { attributi_compilazione: attributi },
    dati_passaporto: passaporto,
  };
}

export function durationDays(values) {
  const a = isoToDate(values['3.05']);
  const b = isoToDate(values['3.06']);
  if (!a || !b || b < a) return '';
  return String(Math.round((b - a) / 86400000) + 1);
}

function addMonths(date, months) {
  const d = new Date(date.getTime());
  d.setUTCMonth(d.getUTCMonth() + months);
  return d;
}

// Errori per campo: { code: messaggio }. Se step è indicato valida solo quel passo.
export function validate(values, step = null) {
  const errors = {};
  const today = new Date();
  today.setUTCHours(0, 0, 0, 0);
  FIELDS.forEach((f) => {
    if (f.operator || (step !== null && f.step !== step)) return;
    if (!isVisible(f, values)) return;
    const v = String(values[f.code] ?? '').trim();
    if (!v) {
      if (isRequired(f, values)) errors[f.code] = t('val.required');
      return;
    }
    if (f.type === 'date' && displayToIso(v) === null) errors[f.code] = t('val.date');
    if (f.type === 'email' && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v)) errors[f.code] = t('val.email');
    if (f.type === 'number' && !/^\d+$/.test(v)) errors[f.code] = t('val.number');
  });

  const inStep = (code) => step === null || FIELD_BY_CODE[code].step === step;
  const entry = isoToDate(values['3.05']);
  const exit = isoToDate(values['3.06']);
  if (entry && inStep('3.05') && entry < today && !errors['3.05']) errors['3.05'] = t('val.entryPast');
  if (entry && exit && exit < entry && inStep('3.06') && !errors['3.06']) errors['3.06'] = t('val.exitBeforeEntry');
  const birth = isoToDate(values['1.03']);
  if (birth && birth > today && inStep('1.03') && !errors['1.03']) errors['1.03'] = t('val.birthFuture');
  const issued = isoToDate(values['2.05']);
  if (issued && issued > today && inStep('2.05') && !errors['2.05']) errors['2.05'] = t('val.issuedFuture');
  const expiry = isoToDate(values['2.06']);
  if (expiry && inStep('2.06') && !errors['2.06']) {
    if (expiry < today) errors['2.06'] = t('val.passportExpired');
    else if (exit && expiry < addMonths(exit, 3)) errors['2.06'] = t('val.passportShort');
  }
  return errors;
}

export function completion(values) {
  let total = 0;
  let done = 0;
  FIELDS.forEach((f) => {
    if (f.operator || !isVisible(f, values) || !isRequired(f, values)) return;
    total += 1;
    if (String(values[f.code] ?? '').trim()) done += 1;
  });
  return { total, done, pct: total ? Math.round((done / total) * 100) : 0 };
}
