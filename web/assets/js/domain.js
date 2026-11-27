import { t, getLang } from './i18n.js';

// ---------- Stati pratica ----------

export const STATI = [
  'BOZZA', 'INVIATA', 'IN_LAVORAZIONE', 'INTEGRAZIONE_RICHIESTA', 'SOSPESA', 'APPROVATA', 'RIFIUTATA',
  'ATTENDE_PAGAMENTO', 'PAGAMENTO_RICEVUTO', 'VISTO_IN_ELABORAZIONE', 'VISTO_EMESSO', 'COMPLETATA', 'ANNULLATA',
];

export const FINAL_STATI = new Set(['RIFIUTATA', 'COMPLETATA', 'ANNULLATA']);

// Transizioni consentite dal backend (internal/store: allowedTransitions).
export const TRANSIZIONI = {
  BOZZA: ['ANNULLATA', 'INVIATA'],
  INVIATA: ['IN_LAVORAZIONE', 'RIFIUTATA'],
  IN_LAVORAZIONE: ['SOSPESA', 'INTEGRAZIONE_RICHIESTA', 'APPROVATA', 'RIFIUTATA'],
  INTEGRAZIONE_RICHIESTA: ['APPROVATA', 'IN_LAVORAZIONE'],
  SOSPESA: ['IN_LAVORAZIONE', 'RIFIUTATA'],
  APPROVATA: ['ATTENDE_PAGAMENTO'],
  ATTENDE_PAGAMENTO: ['PAGAMENTO_RICEVUTO'],
  PAGAMENTO_RICEVUTO: ['VISTO_IN_ELABORAZIONE'],
  VISTO_IN_ELABORAZIONE: ['VISTO_EMESSO'],
  VISTO_EMESSO: ['COMPLETATA'],
};

const TONO = {
  BOZZA: 'neutral',
  INVIATA: 'info',
  IN_LAVORAZIONE: 'info',
  INTEGRAZIONE_RICHIESTA: 'attention',
  SOSPESA: 'warning',
  APPROVATA: 'success',
  RIFIUTATA: 'danger',
  ATTENDE_PAGAMENTO: 'warning',
  PAGAMENTO_RICEVUTO: 'success',
  VISTO_IN_ELABORAZIONE: 'info',
  VISTO_EMESSO: 'success',
  COMPLETATA: 'success',
  ANNULLATA: 'neutral',
};

export function statoLabel(stato) {
  return t(`stato.${stato}`) || stato || '-';
}

export function statoTone(stato) {
  return TONO[stato] || 'neutral';
}

// Gruppo per i filtri del richiedente: bozze, in corso, concluse.
export function statoGroup(stato) {
  if (stato === 'BOZZA') return 'bozze';
  if (FINAL_STATI.has(stato)) return 'concluse';
  return 'in_corso';
}

// Le sei tappe mostrate al richiedente.
export const TAPPE = ['inviata', 'presa', 'verifica', 'pagamento', 'elaborazione', 'emesso'];

export function tappaIndex(stato) {
  switch (stato) {
    case 'BOZZA': return -1;
    case 'INVIATA': return 0;
    case 'IN_LAVORAZIONE': case 'SOSPESA': return 1;
    case 'INTEGRAZIONE_RICHIESTA': case 'APPROVATA': return 2;
    case 'ATTENDE_PAGAMENTO': case 'PAGAMENTO_RICEVUTO': return 3;
    case 'VISTO_IN_ELABORAZIONE': return 4;
    case 'VISTO_EMESSO': case 'COMPLETATA': return 5;
    default: return -1;
  }
}

export function progressPct(stato) {
  if (stato === 'BOZZA') return 10;
  if (stato === 'COMPLETATA' || stato === 'VISTO_EMESSO') return 100;
  const idx = tappaIndex(stato);
  if (idx < 0) return 0;
  return Math.round(((idx + 1) / (TAPPE.length + 1)) * 100);
}

// Prossimo passo dal punto di vista del richiedente.
export function prossimoPasso(p) {
  return t(`next.${p.stato}`) || '';
}

// Azione da fare per l'operatore.
export function cosaFare(p) {
  if (p.stato === 'INVIATA' && !p.operatore_id) return t('todo.INVIATA_UNASSIGNED');
  return t(`todo.${p.stato}`) || '';
}

export function isWaitingOnClient(stato) {
  return stato === 'INTEGRAZIONE_RICHIESTA' || stato === 'ATTENDE_PAGAMENTO' || stato === 'SOSPESA';
}

// ---------- Scadenza indicativa (SLA) ----------
// Calcolata nel client da stato, date e priorità: è un ordine di lavoro,
// non un termine contrattuale.
const SLA_ORE = {
  INVIATA: 48,
  IN_LAVORAZIONE: 72,
  INTEGRAZIONE_RICHIESTA: 168,
  SOSPESA: 168,
  APPROVATA: 24,
  ATTENDE_PAGAMENTO: 168,
  PAGAMENTO_RICEVUTO: 24,
  VISTO_IN_ELABORAZIONE: 120,
  VISTO_EMESSO: 24,
};

export function scadenzaIndicativa(p) {
  const ore = SLA_ORE[p.stato];
  if (!ore) return null;
  const baseRaw = p.stato === 'INVIATA' ? (p.inviato_il || p.aggiornato_il) : (p.aggiornato_il || p.inviato_il);
  const base = parseDate(baseRaw);
  if (!base) return null;
  let factor = 1;
  if (p.priorita === 'URGENTE') factor = 0.5;
  else if (p.priorita === 'ALTA') factor = 0.75;
  return new Date(base.getTime() + ore * factor * 3600 * 1000);
}

export function scadenzaTone(deadline, now = new Date()) {
  if (!deadline) return 'neutral';
  const diffH = (deadline.getTime() - now.getTime()) / 3600000;
  if (diffH < 0) return 'danger';
  if (diffH < 24) return 'attention';
  if (diffH < 48) return 'warning';
  return 'neutral';
}

// ---------- Date ----------

export function parseDate(value) {
  if (!value) return null;
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime()) || d.getFullYear() < 1900) return null;
  return d;
}

function locale() {
  return getLang() === 'en' ? 'en-GB' : 'it-IT';
}

export function fmtDate(value) {
  const d = parseDate(value);
  if (!d) return '-';
  return d.toLocaleDateString(locale(), { day: 'numeric', month: 'short', year: 'numeric' });
}

export function fmtShort(value) {
  const d = parseDate(value);
  if (!d) return '-';
  return d.toLocaleDateString(locale(), { day: 'numeric', month: 'short' });
}

export function fmtDateTime(value) {
  const d = parseDate(value);
  if (!d) return '-';
  return d.toLocaleString(locale(), { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export function fmtRelativeDeadline(deadline, now = new Date()) {
  if (!deadline) return '-';
  const sameDay = deadline.toDateString() === now.toDateString();
  const tomorrow = new Date(now.getTime() + 86400000).toDateString() === deadline.toDateString();
  const time = deadline.toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' });
  if (deadline < now) return t('deadline.overdue', { date: fmtShort(deadline) });
  if (sameDay) return t('deadline.today', { time });
  if (tomorrow) return t('deadline.tomorrow');
  return fmtShort(deadline);
}

// Date di compilazione: in archivio ISO (aaaa-mm-gg), a video gg/mm/aaaa.
export function isoToDisplay(iso) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(iso || ''));
  return m ? `${m[3]}/${m[2]}/${m[1]}` : String(iso || '');
}

export function displayToIso(text) {
  const clean = String(text || '').trim();
  if (!clean) return '';
  if (/^\d{4}-\d{2}-\d{2}$/.test(clean)) return clean;
  const m = /^(\d{1,2})[/.\-\s](\d{1,2})[/.\-\s](\d{4})$/.exec(clean);
  if (!m) return null;
  const day = Number(m[1]);
  const month = Number(m[2]);
  const year = Number(m[3]);
  const d = new Date(Date.UTC(year, month - 1, day));
  if (d.getUTCFullYear() !== year || d.getUTCMonth() !== month - 1 || d.getUTCDate() !== day) return null;
  return `${String(year).padStart(4, '0')}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

export function isoToDate(iso) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(iso || ''));
  return m ? new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]))) : null;
}

// ---------- Paesi ----------
// Elenco ISO 3166-1 locale: i nomi vengono da Intl.DisplayNames, senza
// chiamate esterne (prima si usava restcountries.com).
const COUNTRY_CODES = (
  'AD AE AF AG AL AM AO AR AT AU AZ BA BB BD BE BF BG BH BI BJ BN BO BR BS BT BW BY BZ CA CD CF CG CH CI CL CM CN CO CR CU CV CY CZ '
  + 'DE DJ DK DM DO DZ EC EE EG ER ES ET FI FJ FM FR GA GB GD GE GH GM GN GQ GR GT GW GY HK HN HR HT HU ID IE IL IN IQ IR IS IT JM JO JP '
  + 'KE KG KH KI KM KN KP KR KW KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MG MH MK ML MM MN MO MR MT MU MV MW MX MY MZ NA NE NG NI NL NO NP NR NZ '
  + 'OM PA PE PG PH PK PL PS PT PW PY QA RO RS RU RW SA SB SC SD SE SG SI SK SL SM SN SO SR SS ST SV SY SZ TD TG TH TJ TL TM TN TO TR TT TV TW TZ '
  + 'UA UG US UY UZ VA VC VE VN VU WS XK YE ZA ZM ZW'
).split(' ');

export const SCHENGEN = new Set([
  'AT', 'BE', 'BG', 'CH', 'CZ', 'DE', 'DK', 'EE', 'ES', 'FI', 'FR', 'GR', 'HR', 'HU', 'IS',
  'IT', 'LI', 'LT', 'LU', 'LV', 'MT', 'NL', 'NO', 'PL', 'PT', 'RO', 'SE', 'SI', 'SK',
]);

let namesCache = { lang: '', names: null };

function countryNames() {
  const lang = getLang();
  if (namesCache.lang === lang && namesCache.names) return namesCache.names;
  const names = {};
  let dn = null;
  try {
    dn = new Intl.DisplayNames([lang], { type: 'region' });
  } catch {
    dn = null;
  }
  COUNTRY_CODES.forEach((code) => {
    let name = code;
    try {
      name = (dn && dn.of(code)) || code;
    } catch {
      name = code;
    }
    names[code] = name;
  });
  namesCache = { lang, names };
  return names;
}

export function countryName(code) {
  const raw = String(code || '').trim();
  if (!raw) return '-';
  return countryNames()[raw.toUpperCase()] || raw;
}

export function flag(code) {
  const c = String(code || '').toUpperCase();
  if (!/^[A-Z]{2}$/.test(c) || c === 'XK') return '';
  return String.fromCodePoint(...[...c].map((ch) => 0x1f1e6 + ch.charCodeAt(0) - 65));
}

export function countryOptions() {
  const names = countryNames();
  const collator = new Intl.Collator(getLang());
  return COUNTRY_CODES
    .map((code) => ({ code, name: names[code] }))
    .sort((a, b) => collator.compare(a.name, b.name));
}

export function tipoVistoLabel(tipo) {
  return t(`opt.3.02.${tipo}`) || tipo || '-';
}

// ---------- Documenti ----------

export const TIPI_DOCUMENTO = [
  'PASSAPORTO', 'FOTO_TESSERA', 'PRENOTAZIONE_VOLO', 'PRENOTAZIONE_ALLOGGIO', 'ESTRATTO_CONTO',
  'ASSICURAZIONE', 'LETTERA_INVITO', 'LETTERA_AMMISSIONE', 'CONTRATTO_LAVORO', 'ALTRO',
];

export function documentiConsigliati(tipoVisto) {
  const base = ['PASSAPORTO', 'FOTO_TESSERA'];
  switch (tipoVisto) {
    case 'TURISMO': return [...base, 'PRENOTAZIONE_VOLO', 'PRENOTAZIONE_ALLOGGIO', 'ESTRATTO_CONTO', 'ASSICURAZIONE'];
    case 'STUDIO': return [...base, 'LETTERA_AMMISSIONE', 'ESTRATTO_CONTO', 'ASSICURAZIONE'];
    case 'LAVORO': return [...base, 'CONTRATTO_LAVORO', 'ASSICURAZIONE'];
    case 'RICONGIUNGIMENTO': return [...base, 'LETTERA_INVITO', 'ESTRATTO_CONTO'];
    case 'TRANSITO': return [...base, 'PRENOTAZIONE_VOLO'];
    default: return [...base, 'PRENOTAZIONE_VOLO', 'ESTRATTO_CONTO'];
  }
}

export function docTipoLabel(tipo) {
  return t(`doc.${tipo}`) || String(tipo || '-').replaceAll('_', ' ');
}

export function docStatoTone(stato) {
  switch (String(stato || '').toUpperCase()) {
    case 'APPROVATO': case 'VALIDO': case 'OK': return 'success';
    case 'RIFIUTATO': case 'NON_VALIDO': return 'danger';
    default: return 'info';
  }
}

export function docStatoLabel(stato) {
  return t(`docstato.${String(stato || 'PENDING').toUpperCase()}`) || stato;
}

export function fmtBytes(n) {
  const v = Number(n) || 0;
  if (v < 1024) return `${v} B`;
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(0)} KB`;
  return `${(v / 1024 / 1024).toFixed(1)} MB`;
}

export function fmtMoney(amount, valuta = 'EUR') {
  const v = Number(amount);
  if (!v) return '-';
  try {
    return new Intl.NumberFormat(locale(), { style: 'currency', currency: valuta || 'EUR' }).format(v);
  } catch {
    return `${v.toFixed(2)} ${valuta || ''}`;
  }
}

export function initials(user) {
  const n = `${user?.nome || ''} ${user?.cognome || ''}`.trim() || user?.email || '?';
  return n.split(/\s+/).map((s) => s[0]).join('').slice(0, 2).toUpperCase();
}

export function fullName(user) {
  const n = `${user?.nome || ''} ${user?.cognome || ''}`.trim();
  return n || user?.email || '-';
}

export function cap(text) {
  const s = String(text || '');
  return s ? s[0].toUpperCase() + s.slice(1) : s;
}
