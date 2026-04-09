import { api, formJson } from './http.js';
import {
  applyRoleGuards,
  els,
  extractErrMessage,
  notify,
  out,
  renderList,
  renderSessionInfo,
  setSectionFromHash,
  withBusy,
} from './ui.js';
import {
  clearSession,
  hasActiveSession,
  hasBackofficeRole,
  hasRichiedenteRole,
  role,
  saveApiBase,
  setSession,
  state,
} from './session.js';

const FALLBACK_COUNTRIES = [
  { code: 'IT', label: 'Italia', flag: '🇮🇹' },
  { code: 'US', label: 'Stati Uniti', flag: '🇺🇸' },
  { code: 'GB', label: 'Regno Unito', flag: '🇬🇧' },
  { code: 'CA', label: 'Canada', flag: '🇨🇦' },
  { code: 'AU', label: 'Australia', flag: '🇦🇺' },
  { code: 'JP', label: 'Giappone', flag: '🇯🇵' },
  { code: 'AE', label: 'Emirati Arabi Uniti', flag: '🇦🇪' },
  { code: 'SG', label: 'Singapore', flag: '🇸🇬' },
];

const PRACTICE_HISTORY_KEY = 'visto_easy_practice_history_v1';
const RICH_ACTIVE_TAB_KEY = 'visto_easy_rich_active_tab_v1';
const BO_ACTIVE_TAB_KEY = 'visto_easy_bo_active_tab_v1';
const BO_PRACTICES_VIEW_KEY = 'visto_easy_bo_pratiche_view_v1';
const BO_PAGE_SIZE_KEY = 'visto_easy_bo_page_size_v1';
const FINAL_PRACTICE_STATUSES = new Set(['APPROVATA', 'RIFIUTATA', 'ANNULLATA', 'COMPLETATA', 'CHIUSA']);
const activePracticeFilters = {
  status: 'ALL',
  query: '',
  sort: 'date_desc',
};

const boState = {
  scope: 'mine',
  activeTab: 'pratiche',
  activePraticheView: 'all',
  activeUsersView: 'clienti',
  page: 1,
  pageSize: 20,
  query: '',
  stato: '',
  priorita: '',
  paese_dest: '',
  tipo_visto: '',
  items: [],
  operators: [],
  selectedPraticaId: '',
  activeOp: 'state',
};

const settingsState = {
  activeTab: 'sicurezza',
};

const richState = {
  activeTab: 'create',
};

function parseTs(value) {
  const ts = Date.parse(value || '');
  return Number.isFinite(ts) ? ts : 0;
}

function readPracticeHistory() {
  try {
    const raw = window.localStorage.getItem(PRACTICE_HISTORY_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch (_err) {
    return [];
  }
}

function writePracticeHistory(items) {
  try {
    window.localStorage.setItem(PRACTICE_HISTORY_KEY, JSON.stringify(items.slice(0, 300)));
  } catch (_err) {
    // Ignore storage errors to avoid blocking the main workflow.
  }
}

function readRichActiveTab() {
  try {
    const raw = String(window.localStorage.getItem(RICH_ACTIVE_TAB_KEY) || '').toLowerCase();
    return ['create', 'documents', 'active', 'history'].includes(raw) ? raw : 'create';
  } catch (_err) {
    return 'create';
  }
}

function writeRichActiveTab(tabName) {
  try {
    window.localStorage.setItem(RICH_ACTIVE_TAB_KEY, String(tabName || 'create'));
  } catch (_err) {
    // Ignore storage errors to avoid blocking the UI.
  }
}

function readBOPraticheView() {
  try {
    const raw = String(window.localStorage.getItem(BO_PRACTICES_VIEW_KEY) || '').toLowerCase();
    return raw === 'new' || raw === 'all' ? raw : 'all';
  } catch (_err) {
    return 'all';
  }
}

function writeBOPraticheView(viewName) {
  try {
    window.localStorage.setItem(BO_PRACTICES_VIEW_KEY, String(viewName || 'all'));
  } catch (_err) {
    // Ignore storage errors to avoid blocking the UI.
  }
}

function readBOActiveTab() {
  try {
    const raw = String(window.localStorage.getItem(BO_ACTIVE_TAB_KEY) || '').toLowerCase();
    return ['pratiche', 'utenti', 'audit', 'operazioni'].includes(raw) ? raw : 'pratiche';
  } catch (_err) {
    return 'pratiche';
  }
}

function writeBOActiveTab(tabName) {
  try {
    window.localStorage.setItem(BO_ACTIVE_TAB_KEY, String(tabName || 'pratiche'));
  } catch (_err) {
    // Ignore storage errors to avoid blocking the UI.
  }
}

function readBOPageSize() {
  try {
    const n = Number(window.localStorage.getItem(BO_PAGE_SIZE_KEY));
    return [10, 20, 30, 50, 100].includes(n) ? n : 20;
  } catch (_err) {
    return 20;
  }
}

function writeBOPageSize(pageSize) {
  try {
    window.localStorage.setItem(BO_PAGE_SIZE_KEY, String(pageSize || 20));
  } catch (_err) {
    // Ignore storage errors to avoid blocking the UI.
  }
}

function removePracticeFromHistory(praticaID) {
  if (!praticaID) return;
  const next = readPracticeHistory().filter((item) => item?.id !== praticaID);
  writePracticeHistory(next);
}

function normalizePracticeEntry(p) {
  return {
    id: p.id || '',
    codice: p.codice || p.id || '-',
    stato: p.stato || '-',
    tipo_visto: p.tipo_visto || '-',
    paese_dest: p.paese_dest || '-',
    creato_il: p.creato_il || new Date().toISOString(),
  };
}

function mergePracticeHistory(pratiche) {
  const current = Array.isArray(pratiche) ? pratiche.map(normalizePracticeEntry) : [];
  const byID = {};
  readPracticeHistory().forEach((item) => {
    if (item && item.id) byID[item.id] = item;
  });
  current.forEach((item) => {
    if (!item.id) return;
    byID[item.id] = { ...byID[item.id], ...item };
  });
  const merged = Object.values(byID).sort((a, b) => parseTs(b.creato_il) - parseTs(a.creato_il));
  writePracticeHistory(merged);
  return merged;
}

function isHistoricalPractice(p) {
  const stato = String(p.stato || '').toUpperCase();
  if (FINAL_PRACTICE_STATUSES.has(stato)) return true;

  const ts = parseTs(p.creato_il);
  if (!ts) return false;
  const startOfToday = new Date();
  startOfToday.setHours(0, 0, 0, 0);
  return ts < startOfToday.getTime();
}

function isDraftPractice(p) {
  return String(p?.stato || '').toUpperCase() === 'BOZZA';
}

function filterActivePractices(items) {
  const statusFilter = String(activePracticeFilters.status || 'ALL').toUpperCase();
  const queryFilter = String(activePracticeFilters.query || '').trim().toLowerCase();
  return (items || []).filter((p) => {
    const currentStatus = String(p.stato || '').toUpperCase();
    if (statusFilter !== 'ALL' && currentStatus !== statusFilter) return false;
    if (!queryFilter) return true;
    const blob = `${p.codice || ''} ${p.tipo_visto || ''} ${p.paese_dest || ''}`.toLowerCase();
    return blob.includes(queryFilter);
  });
}

function sortActivePractices(items) {
  const mode = String(activePracticeFilters.sort || 'date_desc');
  const rows = [...(items || [])];
  const byDateDesc = (a, b) => parseTs(b.creato_il) - parseTs(a.creato_il);
  const statusRank = {
    BOZZA: 1,
    INVIATA: 2,
    IN_LAVORAZIONE: 3,
    INTEGRAZIONE_RICHIESTA: 4,
    SOSPESA: 5,
  };

  if (mode === 'date_asc') {
    return rows.sort((a, b) => parseTs(a.creato_il) - parseTs(b.creato_il));
  }
  if (mode === 'status') {
    return rows.sort((a, b) => {
      const da = statusRank[String(a.stato || '').toUpperCase()] || 99;
      const db = statusRank[String(b.stato || '').toUpperCase()] || 99;
      if (da !== db) return da - db;
      return byDateDesc(a, b);
    });
  }
  if (mode === 'country') {
    return rows.sort((a, b) => {
      const pa = String(a.paese_dest || '').toLowerCase();
      const pb = String(b.paese_dest || '').toLowerCase();
      const cmp = pa.localeCompare(pb, 'it');
      if (cmp !== 0) return cmp;
      return byDateDesc(a, b);
    });
  }
  return rows.sort(byDateDesc);
}

function wireActivePracticeFilters() {
  const statusEl = document.getElementById('activeStatusFilter');
  const queryEl = document.getElementById('activeQueryFilter');
  const sortEl = document.getElementById('activeSortFilter');
  if (!statusEl || !queryEl || !sortEl) return;

  statusEl.addEventListener('change', async (ev) => {
    activePracticeFilters.status = ev.currentTarget.value || 'ALL';
    await loadMiePratiche();
  });

  queryEl.addEventListener('input', async (ev) => {
    activePracticeFilters.query = ev.currentTarget.value || '';
    await loadMiePratiche();
  });

  sortEl.addEventListener('change', async (ev) => {
    activePracticeFilters.sort = ev.currentTarget.value || 'date_desc';
    await loadMiePratiche();
  });
}

function renderPracticeHistory(items) {
  const container = document.getElementById('storicoPratiche');
  if (!container) return;

  const historical = (items || []).filter(isHistoricalPractice);
  renderList(container, historical, (p) => `
    <article class="list-item">
      <h3>${p.codice || p.id} - ${p.stato || '-'}</h3>
      <p>${p.tipo_visto || '-'} | ${p.paese_dest || '-'} | ${new Date(p.creato_il).toLocaleString()}</p>
    </article>
  `);
}

function toCountryOption(item) {
  const code = (item.cca2 || '').toUpperCase();
  const label = item.name?.common || code;
  const flag = item.flag || '';
  return { code, label, flag };
}

async function loadCountriesIntoSelect() {
  const select = document.getElementById('paeseSelect');
  if (!select) return;

  let countries = [];
  try {
    const res = await fetch('https://restcountries.com/v3.1/all?fields=cca2,name,flag');
    if (!res.ok) throw new Error(`restcountries ${res.status}`);
    const rows = await res.json();
    countries = rows.map(toCountryOption).filter((c) => c.code && c.label);
  } catch (_err) {
    countries = FALLBACK_COUNTRIES;
  }

  countries.sort((a, b) => a.label.localeCompare(b.label, 'it'));
  select.innerHTML = [
    '<option value="" disabled selected>Seleziona paese</option>',
    ...countries.map((c) => `<option value="${c.code}">${c.flag} ${c.label} (${c.code})</option>`),
  ].join('');
}

function wireDocUploadMetadata() {
  const form = document.getElementById('formDocUpload');
  if (!form) return;
  const fileInput = form.querySelector('input[name="file_upload"]');
  const nameInput = form.querySelector('input[name="nome_file"]');
  const mimeInput = form.querySelector('input[name="mime_type"]');
  const sizeInput = form.querySelector('input[name="dimensione"]');
  if (!fileInput || !nameInput || !mimeInput || !sizeInput) return;

  fileInput.addEventListener('change', () => {
    const file = fileInput.files?.[0];
    if (!file) {
      nameInput.value = '';
      mimeInput.value = '';
      sizeInput.value = '';
      return;
    }
    nameInput.value = file.name || 'documento';
    mimeInput.value = file.type || 'application/octet-stream';
    sizeInput.value = String(file.size || 0);
  });
}

function wireTabs() {
  document.querySelectorAll('#authTabs .tab').forEach((btn) => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('#authTabs .tab').forEach((t) => t.classList.remove('active'));
      document.querySelectorAll('[data-pane]').forEach((p) => p.classList.remove('active'));
      btn.classList.add('active');
      const pane = document.querySelector(`[data-pane="${btn.dataset.tab}"]`);
      if (pane) pane.classList.add('active');
    });
  });
}

function showRichTab(tabName) {
  const active = ['create', 'documents', 'active', 'history'].includes(String(tabName || '').toLowerCase())
    ? String(tabName || '').toLowerCase()
    : 'create';
  richState.activeTab = active;
  writeRichActiveTab(active);

  document.querySelectorAll('[data-rich-tab]').forEach((btn) => {
    const isActive = btn.dataset.richTab === active;
    btn.classList.toggle('active', isActive);
    btn.setAttribute('aria-selected', isActive ? 'true' : 'false');
  });

  document.querySelectorAll('[data-rich-panel]').forEach((panel) => {
    const isActive = panel.dataset.richPanel === active;
    panel.classList.toggle('hidden', !isActive);
    panel.hidden = !isActive;
  });
}

function wireRichTabs() {
  const tabs = document.querySelectorAll('[data-rich-tab]');
  if (!tabs.length) return;
  richState.activeTab = readRichActiveTab();
  tabs.forEach((btn) => {
    btn.addEventListener('click', (ev) => {
      ev.preventDefault();
      showRichTab(btn.dataset.richTab || 'create');
    });
  });
  showRichTab(richState.activeTab || 'create');
}

function showAuthView(view) {
  const target = String(view || 'login').toLowerCase();
  document.querySelectorAll('#auth .auth-view[data-auth-view]').forEach((el) => {
    const isTarget = String(el.dataset.authView || '').toLowerCase() === target;
    if (isTarget) {
      el.classList.remove('hidden');
      el.hidden = false;
      el.classList.add('active');
    } else {
      el.classList.add('hidden');
      el.hidden = true;
      el.classList.remove('active');
    }
  });
}

function wireAuthViews() {
  const controls = document.querySelectorAll('#auth .auth-mobile-switch [data-auth-view], #auth .auth-desktop-actions [data-auth-view], #auth .auth-back[data-auth-view]');
  controls.forEach((el) => {
    el.addEventListener('click', (ev) => {
      const view = ev.currentTarget.dataset.authView;
      showAuthView(view);
      document.querySelectorAll('#auth .auth-mobile-switch [data-auth-view]').forEach((btn) => {
        btn.classList.toggle('active', btn.dataset.authView === view);
      });
    });
  });
  showAuthView('login');
}

function populateDocPraticaSelect(pratiche, preferredID = '') {
  const select = document.getElementById('docPraticaSelect');
  if (!select) return;

  if (!pratiche || pratiche.length === 0) {
    select.innerHTML = '<option value="" disabled selected>Nessuna pratica disponibile</option>';
    return;
  }

  select.innerHTML = [
    '<option value="" disabled>Seleziona una pratica</option>',
    ...pratiche.map((p) => `<option value="${p.id}">${p.codice || p.id} - ${p.stato || '-'}</option>`),
  ].join('');

  const currentID = preferredID || select.value;
  const selected = pratiche.find((p) => p.id === currentID)?.id || pratiche[0].id;
  select.value = selected;
}

async function loadMiePratiche(preferredPraticaID = '') {
  if (!hasRichiedenteRole()) {
    notify('err', 'Azione riservata al ruolo RICHIEDENTE');
    return;
  }
  const data = await api('/api/pratiche/');
  const history = mergePracticeHistory(data);
  renderPracticeHistory(history);

  const operative = data.filter((p) => !isHistoricalPractice(p));
  const filteredOperative = filterActivePractices(operative);
  const sortedOperative = sortActivePractices(filteredOperative);
  populateDocPraticaSelect(operative.length > 0 ? operative : data, preferredPraticaID);
  renderList(els.miePratiche, sortedOperative, (p) => `
    <article class="list-item">
      <h3>${p.codice || p.id} - ${p.stato}</h3>
      <p>${p.tipo_visto || '-'} | ${p.paese_dest || '-'} | ${new Date(p.creato_il).toLocaleString()}</p>
      <div class="inline-actions" data-pratica-id="${p.id}">
        ${isDraftPractice(p) ? '<button class="btn btn-ghost" type="button" data-action="submit">Invia</button>' : ''}
        ${isDraftPractice(p) ? '<button class="btn btn-ghost" type="button" data-action="delete">Elimina</button>' : ''}
        <button class="btn btn-ghost" type="button" data-action="docs">Documenti</button>
      </div>
    </article>
  `);

  els.miePratiche.querySelectorAll('[data-pratica-id]').forEach((bar) => {
    const praticaID = bar.dataset.praticaId;
    const submitBtn = bar.querySelector('[data-action="submit"]');
    if (submitBtn) submitBtn.addEventListener('click', async (ev) => {
      try {
        await withBusy(ev.currentTarget, async () => {
          const res = await api(`/api/pratiche/${praticaID}/submit`, { method: 'POST' });
          out('Pratica submit', res);
          notify('ok', 'Pratica inviata');
          await loadMiePratiche();
        });
      } catch (err) {
        notify('err', extractErrMessage(err));
      }
    });

    const deleteBtn = bar.querySelector('[data-action="delete"]');
    if (deleteBtn) deleteBtn.addEventListener('click', async (ev) => {
      try {
        await withBusy(ev.currentTarget, async () => {
          await api(`/api/pratiche/${praticaID}`, { method: 'DELETE' });
          removePracticeFromHistory(praticaID);
          out('Pratica eliminata', { id: praticaID });
          notify('ok', 'Pratica eliminata');
          await loadMiePratiche();
        });
      } catch (err) {
        notify('err', extractErrMessage(err));
      }
    });

    bar.querySelector('[data-action="docs"]').addEventListener('click', async (ev) => {
      try {
        await withBusy(ev.currentTarget, async () => {
          const docs = await api(`/api/pratiche/${praticaID}/documenti`);
          const select = document.getElementById('docPraticaSelect');
          if (select) select.value = praticaID;
          out(`Documenti pratica ${praticaID}`, docs);
          notify('ok', `Pratica selezionata per upload documenti: ${praticaID}`);
        });
      } catch (err) {
        notify('err', extractErrMessage(err));
      }
    });
  });

  out('Pratiche caricate', { operative: operative.length, filtrate: filteredOperative.length, storico: history.length, sort: activePracticeFilters.sort });
}

function isBackofficeRoleName(roleName) {
  const role = String(roleName || '').toUpperCase();
  return role === 'OPERATORE' || role === 'SUPERVISORE' || role === 'ADMIN';
}

function getBackofficeUserId() {
  return state.user?.id || '';
}

function isUnassignedPractice(p) {
  return !String(p?.operatore_id || '').trim();
}

function normalizeBackofficeText(value) {
  return String(value || '').trim().toLowerCase();
}

function matchesBackofficeFilters(p) {
  const scope = String(boState.scope || 'mine');
  const userID = getBackofficeUserId();
  if (scope === 'mine' && userID && p.operatore_id !== userID) return false;
  if (scope === 'new' && !isUnassignedPractice(p)) return false;
  if (boState.stato && normalizeBackofficeText(p.stato) !== normalizeBackofficeText(boState.stato)) return false;
  if (boState.priorita && normalizeBackofficeText(p.priorita) !== normalizeBackofficeText(boState.priorita)) return false;
  if (boState.paese_dest && !normalizeBackofficeText(p.paese_dest).includes(normalizeBackofficeText(boState.paese_dest))) return false;
  if (boState.tipo_visto && !normalizeBackofficeText(p.tipo_visto).includes(normalizeBackofficeText(boState.tipo_visto))) return false;
  if (boState.query) {
    const blob = [p.codice, p.stato, p.tipo_visto, p.paese_dest, p.note_interne, p.note_richiedente, p.richiedente?.email].map(normalizeBackofficeText).join(' ');
    if (!blob.includes(normalizeBackofficeText(boState.query))) return false;
  }
  return true;
}

function sortBackofficePratiche(items) {
  return [...items].sort((a, b) => parseTs(b.aggiornato_il || b.creato_il) - parseTs(a.aggiornato_il || a.creato_il));
}

function getOperatorLabel(p) {
  if (!p.operatore_id) return 'Non assegnata';
  const op = boState.operators.find((item) => item.id === p.operatore_id);
  if (op) {
    const fullName = `${op.nome || ''} ${op.cognome || ''}`.trim();
    if (p.operatore_id === getBackofficeUserId()) return `${fullName || op.email} (io)`;
    return fullName || op.email;
  }
  if (p.operatore_id === getBackofficeUserId()) return 'Assegnata a me';
  return 'Assegnata';
}

function renderBOAssignOperators() {
  const select = document.getElementById('boAssignOperatorSelect');
  if (!select) return;
  const current = String(select.value || '');
  const options = boState.operators
    .map((op) => {
      const fullName = `${op.nome || ''} ${op.cognome || ''}`.trim();
      const label = fullName ? `${fullName} (${op.email})` : op.email;
      return `<option value="${op.id}">${label}</option>`;
    })
    .join('');
  select.innerHTML = '<option value="" selected disabled>Seleziona operatore disponibile</option>' + options;
  select.disabled = boState.operators.length === 0;
  if (boState.operators.length === 0) {
    select.innerHTML = '<option value="" selected disabled>Nessun operatore disponibile</option>';
  }
  if (current && boState.operators.some((op) => op.id === current)) {
    select.value = current;
  }
}

function showBOUsersView(viewName) {
  const active = String(viewName || 'clienti').toLowerCase() === 'operatori' ? 'operatori' : 'clienti';
  boState.activeUsersView = active;
  document.querySelectorAll('[data-bo-users-view]').forEach((btn) => {
    const isActive = btn.dataset.boUsersView === active;
    btn.classList.toggle('active', isActive);
    btn.setAttribute('aria-selected', isActive ? 'true' : 'false');
  });
  document.querySelectorAll('[data-bo-users-panel]').forEach((panel) => {
    const isActive = panel.dataset.boUsersPanel === active;
    panel.classList.toggle('hidden', !isActive);
    panel.hidden = !isActive;
  });
}

function wireBOUsersTabs() {
  const tabs = document.querySelectorAll('[data-bo-users-view]');
  if (!tabs.length) return;
  tabs.forEach((btn) => {
    btn.addEventListener('click', (ev) => {
      ev.preventDefault();
      showBOUsersView(btn.dataset.boUsersView || 'clienti');
    });
  });
  showBOUsersView(boState.activeUsersView || 'clienti');
}

function setBOInviteOperatorBox(open) {
  const box = document.getElementById('boInviteOperatorBox');
  const button = document.getElementById('btnBOToggleInviteOperator');
  if (!box || !button) return;
  const isOpen = Boolean(open);
  box.classList.toggle('hidden', !isOpen);
  box.hidden = !isOpen;
  button.setAttribute('aria-expanded', isOpen ? 'true' : 'false');
  button.textContent = isOpen ? 'Chiudi invito operatore' : 'Invita nuovo operatore';
}

function wireBOInviteOperatorToggle() {
  const button = document.getElementById('btnBOToggleInviteOperator');
  if (!button) return;
  setBOInviteOperatorBox(false);
  button.addEventListener('click', () => {
    const expanded = button.getAttribute('aria-expanded') === 'true';
    setBOInviteOperatorBox(!expanded);
  });
}

function getRichiedenteLabel(p) {
  const nome = String(p.richiedente?.nome || '').trim();
  const cognome = String(p.richiedente?.cognome || '').trim();
  const email = String(p.richiedente?.email || '').trim();
  const fullName = `${nome} ${cognome}`.trim();
  if (fullName && email) return `${fullName} (${email})`;
  if (fullName) return fullName;
  if (email) return email;
  return '-';
}

function getStatusBadgeClass(status) {
  const s = String(status || '').toUpperCase();
  const known = {
    BOZZA: 'bo-status--bozza',
    INVIATA: 'bo-status--inviata',
    IN_LAVORAZIONE: 'bo-status--in_lavorazione',
    INTEGRAZIONE_RICHIESTA: 'bo-status--integrazione_richiesta',
    SOSPESA: 'bo-status--sospesa',
    APPROVATA: 'bo-status--approvata',
    RIFIUTATA: 'bo-status--rifiutata',
    ATTENDE_PAGAMENTO: 'bo-status--attende_pagamento',
    PAGAMENTO_RICEVUTO: 'bo-status--pagamento_ricevuto',
    VISTO_IN_ELABORAZIONE: 'bo-status--visto_in_elaborazione',
    VISTO_EMESSO: 'bo-status--visto_emesso',
    COMPLETATA: 'bo-status--completata',
    ANNULLATA: 'bo-status--annullata',
  };
  if (known[s]) return `bo-status-badge ${known[s]}`;
  return 'bo-status-badge';
}

function setBackofficeFormIds(praticaID) {
  ['formBOChangeState', 'formBOAddNote', 'formBOAssign', 'formBORequestDoc', 'formBOPaymentLink'].forEach((formId) => {
    const input = document.querySelector(`#${formId} input[name="id"]`);
    if (input) input.value = praticaID || '';
  });
}

function resolveBOPraticaID(inputID) {
  const explicitID = String(inputID || '').trim();
  if (explicitID) return explicitID;
  return String(boState.selectedPraticaId || '').trim();
}

async function loadBOPraticaDocumenti(praticaID) {
  const docsContainer = document.getElementById('boDetailDocumenti');
  const previewContainer = document.getElementById('boDocPreview');
  if (!docsContainer || !praticaID) return;
  docsContainer.innerHTML = '<p class="helper-text">Caricamento documenti...</p>';
  try {
    const docs = await api(`/api/pratiche/${praticaID}/documenti`);
    if (!Array.isArray(docs) || !docs.length) {
      docsContainer.innerHTML = '<p class="helper-text">Nessun documento disponibile.</p>';
      if (previewContainer) previewContainer.innerHTML = '<p class="helper-text">Anteprima non disponibile.</p>';
      return;
    }
    docsContainer.innerHTML = docs.map((d) => `
      <article class="bo-doc-item">
        <h4>${d.nome_file || 'Documento'}</h4>
        <p>${d.tipo || '-'} | ${(d.mime_type || '-').toLowerCase()} | ${d.dimensione || 0} bytes</p>
        <div class="inline-actions">
          <button class="btn btn-ghost" type="button" data-doc-preview-id="${d.id}" data-doc-preview-mime="${d.mime_type || ''}" data-doc-preview-name="${d.nome_file || ''}">Anteprima</button>
          <a class="btn btn-ghost" href="/api/pratiche/${praticaID}/documenti/${d.id}/download" target="_blank" rel="noopener noreferrer">Scarica</a>
        </div>
      </article>
    `).join('');

    docsContainer.querySelectorAll('[data-doc-preview-id]').forEach((button) => {
      button.addEventListener('click', () => {
        const docID = button.dataset.docPreviewId;
        const mime = button.dataset.docPreviewMime || '';
        const fileName = button.dataset.docPreviewName || 'Documento';
        renderBODocumentPreview(praticaID, docID, mime, fileName);
      });
    });

    const first = docs[0];
    renderBODocumentPreview(praticaID, first.id, first.mime_type || '', first.nome_file || 'Documento');
  } catch (err) {
    docsContainer.innerHTML = `<p class="helper-text">${extractErrMessage(err)}</p>`;
    if (previewContainer) previewContainer.innerHTML = '<p class="helper-text">Anteprima non disponibile.</p>';
  }
}

function renderBODocumentPreview(praticaID, docID, mimeType, fileName) {
  const previewContainer = document.getElementById('boDocPreview');
  if (!previewContainer) return;

  const safeMime = String(mimeType || '').toLowerCase();
  const previewURL = `/api/pratiche/${praticaID}/documenti/${docID}/preview`;
  const downloadURL = `/api/pratiche/${praticaID}/documenti/${docID}/download`;

  if (safeMime.startsWith('image/')) {
    previewContainer.innerHTML = `
      <figure class="bo-doc-preview-figure">
        <img src="${previewURL}" alt="Anteprima ${fileName}" loading="lazy">
      </figure>
    `;
    return;
  }

  if (safeMime.includes('pdf')) {
    previewContainer.innerHTML = `
      <iframe class="bo-doc-preview-frame" src="${previewURL}" title="Anteprima ${fileName}"></iframe>
    `;
    return;
  }

  previewContainer.innerHTML = `
    <p class="helper-text">Anteprima non disponibile per questo formato.</p>
    <a class="btn btn-ghost" href="${downloadURL}" target="_blank" rel="noopener noreferrer">Apri documento</a>
  `;
}

function renderBackofficeDetail(pratica) {
  const detail = document.getElementById('boPraticaDetail');
  if (!detail) return;
  if (!pratica) {
    detail.innerHTML = '<p class="helper-text">Seleziona una pratica dalla tabella per vedere i dettagli.</p>';
    detail.classList.add('hidden');
    detail.classList.remove('focus');
    return;
  }

  detail.classList.remove('hidden');
  detail.classList.add('focus');
  detail.innerHTML = `
    <div class="bo-detail-head">
      <div>
        <h3>${pratica.codice || pratica.id}</h3>
        <p>${pratica.tipo_visto || '-'} | ${pratica.paese_dest || '-'}</p>
        <p class="helper-text">Richiedente: ${getRichiedenteLabel(pratica)}</p>
      </div>
      <button class="btn btn-ghost" type="button" data-bo-action="back">Torna elenco</button>
    </div>
    <p><span class="${getStatusBadgeClass(pratica.stato)}">${pratica.stato || '-'}</span> <span class="bo-status-badge">${getOperatorLabel(pratica)}</span></p>
    <p class="helper-text">Aggiornata ${new Date(pratica.aggiornato_il || pratica.creato_il).toLocaleString()}</p>
    <div class="bo-detail-actions">
      <button class="btn btn-solid" type="button" data-bo-action="assign-me">Assegna a me</button>
      <button class="btn btn-ghost" type="button" data-bo-action="show-docs">Vedi documenti</button>
    </div>
    <section class="bo-detail-grid">
      <article class="bo-detail-card">
        <h4>Dati pratica</h4>
        <p><strong>ID:</strong> ${pratica.id || '-'}</p>
        <p><strong>Priorita:</strong> ${pratica.priorita || '-'}</p>
        <p><strong>Operatore:</strong> ${pratica.operatore_id || 'non assegnata'}</p>
      </article>
      <article class="bo-detail-card">
        <h4>Note richiedente</h4>
        <p>${pratica.note_richiedente || 'Nessuna nota richiedente'}</p>
      </article>
      <article class="bo-detail-card">
        <h4>Note interne</h4>
        <p>${pratica.note_interne || 'Nessuna nota interna'}</p>
        <div class="bo-inline-note-editor">
          <label>Nuova nota interna
            <textarea id="boDetailInternalNote" rows="3" placeholder="Aggiungi una nota interna operativa"></textarea>
          </label>
          <button class="btn btn-ghost" type="button" data-bo-action="add-internal-note">Salva nota interna</button>
        </div>
      </article>
    </section>
    <section class="bo-detail-card">
      <h4>Documenti</h4>
      <div id="boDetailDocumenti"><p class="helper-text">Premi "Vedi documenti" per caricare l'elenco.</p></div>
      <div id="boDocPreview" class="bo-doc-preview"><p class="helper-text">Seleziona un documento per l'anteprima.</p></div>
    </section>
  `;

  detail.querySelector('[data-bo-action="back"]').addEventListener('click', () => {
    boState.selectedPraticaId = '';
    setBackofficeFormIds('');
    renderBackofficeDetail(null);
    renderBOPraticheTable();
  });

  detail.querySelector('[data-bo-action="assign-me"]').addEventListener('click', async (ev) => {
    try {
      await withBusy(ev.currentTarget, async () => {
        const userID = getBackofficeUserId();
        if (!userID) {
          notify('err', 'Utente backoffice non disponibile');
          return;
        }
        const data = await api(`/api/bo/pratiche/${pratica.id}/assegna`, {
          method: 'POST',
          body: JSON.stringify({ operatore_id: userID }),
        });
        out('BO assegna a me', data);
        notify('ok', 'Pratica assegnata a te');
        await loadBOPratiche();
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  detail.querySelector('[data-bo-action="show-docs"]').addEventListener('click', async () => {
    await loadBOPraticaDocumenti(pratica.id);
  });

  detail.querySelector('[data-bo-action="add-internal-note"]').addEventListener('click', async (ev) => {
    const input = detail.querySelector('#boDetailInternalNote');
    const messaggio = String(input?.value || '').trim();
    if (!messaggio) {
      notify('err', 'Inserisci un testo per la nota interna.');
      return;
    }
    try {
      await withBusy(ev.currentTarget, async () => {
        const data = await api(`/api/bo/pratiche/${pratica.id}/note`, {
          method: 'POST',
          body: JSON.stringify({ messaggio, interna: true }),
        });
        out('BO aggiungi nota interna dettaglio', data);
        notify('ok', 'Nota interna aggiunta');
        await loadBOPratiche();
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  setBackofficeFormIds(pratica.id);
}

function renderBOPraticheTable() {
  const container = els.boPratiche;
  const filteredRows = sortBackofficePratiche(boState.items.filter(matchesBackofficeFilters));
  const selectedId = boState.selectedPraticaId;
  if (!container) return;

  if (!filteredRows.length) {
    container.innerHTML = '<div class="list-item"><p>Nessuna pratica trovata</p></div>';
    const pager = document.getElementById('boPagination');
    if (pager) pager.innerHTML = '<span class="pager-info">0 risultati</span>';
    renderBackofficeDetail(null);
    return;
  }

  const pageSize = Number(boState.pageSize) || 20;
  const totalRows = filteredRows.length;
  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));
  boState.page = Math.min(Math.max(boState.page || 1, 1), totalPages);
  const start = (boState.page - 1) * pageSize;
  const rows = filteredRows.slice(start, start + pageSize);

  container.innerHTML = `
    <table class="bo-table">
      <thead>
        <tr>
          <th>Codice</th>
          <th>Richiedente</th>
          <th>Stato</th>
          <th>Assegnazione</th>
          <th>Tipo visto</th>
          <th>Paese</th>
          <th>Aggiornata</th>
        </tr>
      </thead>
      <tbody>
        ${rows.map((p) => `
          <tr data-pratica-id="${p.id}" class="${p.id === selectedId ? 'selected' : ''}">
            <td>${p.codice || p.id}</td>
            <td>${getRichiedenteLabel(p)}</td>
            <td><span class="${getStatusBadgeClass(p.stato)}">${p.stato || '-'}</span></td>
            <td>${getOperatorLabel(p)}</td>
            <td>${p.tipo_visto || '-'}</td>
            <td>${p.paese_dest || '-'}</td>
            <td>${new Date(p.aggiornato_il || p.creato_il).toLocaleString()}</td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;

  const pager = document.getElementById('boPagination');
  if (pager) {
    pager.innerHTML = `
      <button class="btn btn-ghost" type="button" data-bo-page="prev" ${boState.page <= 1 ? 'disabled' : ''}>Precedente</button>
      <span class="pager-info">Pagina ${boState.page} di ${totalPages} (${totalRows} risultati)</span>
      <button class="btn btn-ghost" type="button" data-bo-page="next" ${boState.page >= totalPages ? 'disabled' : ''}>Successiva</button>
    `;
    pager.querySelector('[data-bo-page="prev"]')?.addEventListener('click', () => {
      if (boState.page <= 1) return;
      boState.page -= 1;
      renderBOPraticheTable();
    });
    pager.querySelector('[data-bo-page="next"]')?.addEventListener('click', () => {
      if (boState.page >= totalPages) return;
      boState.page += 1;
      renderBOPraticheTable();
    });
  }

  const pageSizeSelect = document.getElementById('boPageSize');
  if (pageSizeSelect) {
    pageSizeSelect.value = String(pageSize);
    pageSizeSelect.onchange = () => {
      const next = Number(pageSizeSelect.value);
      boState.pageSize = [10, 20, 30, 50, 100].includes(next) ? next : 20;
      boState.page = 1;
      writeBOPageSize(boState.pageSize);
      renderBOPraticheTable();
    };
  }

  container.querySelectorAll('[data-pratica-id]').forEach((row) => {
    row.addEventListener('click', () => {
      const praticaID = row.dataset.praticaId;
      boState.selectedPraticaId = praticaID;
      const pratica = boState.items.find((item) => item.id === praticaID);
      renderBackofficeDetail(pratica);
      renderBOPraticheTable();
    });
  });

  if (selectedId) {
    renderBackofficeDetail(boState.items.find((item) => item.id === selectedId) || null);
  } else {
    renderBackofficeDetail(null);
  }
}

function renderBONewPratiche() {
  const container = document.getElementById('boNewPratiche');
  if (!container) return;
  const newRows = boState.items.filter((p) => isUnassignedPractice(p) && (normalizeBackofficeText(p.stato) === 'bozza' || normalizeBackofficeText(p.stato) === 'inviata'));
  if (!newRows.length) {
    container.innerHTML = '<div class="list-item"><p>Nessuna pratica nuova da assegnare</p></div>';
    return;
  }

  container.innerHTML = newRows.slice(0, 6).map((p) => `
    <article class="bo-card">
      <div class="bo-card-head">
        <div>
          <h3>${p.codice || p.id}</h3>
          <p>${p.tipo_visto || '-'} | ${p.paese_dest || '-'}</p>
          <p class="helper-text">${getRichiedenteLabel(p)}</p>
        </div>
        <span class="${getStatusBadgeClass(p.stato)}">${p.stato || '-'}</span>
      </div>
      <p class="helper-text">${new Date(p.aggiornato_il || p.creato_il).toLocaleString()}</p>
      <button class="btn btn-solid" type="button" data-assign-id="${p.id}">Assegna a me</button>
    </article>
  `).join('');

  container.querySelectorAll('[data-assign-id]').forEach((button) => {
    button.addEventListener('click', async (ev) => {
      try {
        await withBusy(ev.currentTarget, async () => {
          const userID = getBackofficeUserId();
          const praticaID = button.dataset.assignId;
          const data = await api(`/api/bo/pratiche/${praticaID}/assegna`, {
            method: 'POST',
            body: JSON.stringify({ operatore_id: userID }),
          });
          out('BO assegna nuova pratica', data);
          notify('ok', 'Pratica assegnata');
          await loadBOPratiche();
        });
      } catch (err) {
        notify('err', extractErrMessage(err));
      }
    });
  });
}

function wireBackofficeMenu() {
  document.querySelectorAll('[data-bo-scope]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      boState.scope = btn.dataset.boScope || 'mine';
      boState.page = 1;
      document.querySelectorAll('[data-bo-scope]').forEach((other) => other.classList.toggle('active', other === btn));
      await loadBOPratiche();
    });
  });
}

function showBackofficeTab(tabName) {
  const active = String(tabName || 'pratiche');
  boState.activeTab = active;
  writeBOActiveTab(active);
  document.querySelectorAll('[data-bo-tab]').forEach((btn) => {
    const isActive = btn.dataset.boTab === active;
    btn.classList.toggle('active', isActive);
    btn.setAttribute('aria-selected', isActive ? 'true' : 'false');
  });
  document.querySelectorAll('[data-bo-panel]').forEach((panel) => {
    const isActive = panel.dataset.boPanel === active;
    panel.classList.toggle('show', isActive);
    panel.classList.toggle('active', isActive);
    panel.classList.toggle('hidden', !isActive);
    panel.hidden = !isActive;
  });
}

function wireBackofficeTabs() {
  boState.activeTab = readBOActiveTab();
  document.querySelectorAll('[data-bo-tab]').forEach((btn) => {
    btn.addEventListener('click', (ev) => {
      ev.preventDefault();
      showBackofficeTab(btn.dataset.boTab || 'pratiche');
    });
  });
  showBackofficeTab(boState.activeTab || 'pratiche');
}

function showBOPraticheView(viewName) {
  const active = String(viewName || 'all').toLowerCase() === 'new' ? 'new' : 'all';
  boState.activePraticheView = active;
  writeBOPraticheView(active);
  document.querySelectorAll('[data-bo-pratiche-view]').forEach((btn) => {
    const isActive = btn.dataset.boPraticheView === active;
    btn.classList.toggle('active', isActive);
    btn.setAttribute('aria-selected', isActive ? 'true' : 'false');
  });
  document.querySelectorAll('[data-bo-pratiche-panel]').forEach((panel) => {
    const isActive = panel.dataset.boPratichePanel === active;
    panel.classList.toggle('hidden', !isActive);
    panel.hidden = !isActive;
  });

  const detail = document.getElementById('boPraticaDetail');
  if (detail && active !== 'all') {
    detail.classList.add('hidden');
  }
  if (active === 'all' && boState.selectedPraticaId) {
    const pratica = boState.items.find((item) => item.id === boState.selectedPraticaId) || null;
    renderBackofficeDetail(pratica);
  }
}

function wireBOPraticheTabs() {
  const tabs = document.querySelectorAll('[data-bo-pratiche-view]');
  if (!tabs.length) return;
  boState.activePraticheView = readBOPraticheView();
  tabs.forEach((btn) => {
    btn.addEventListener('click', (ev) => {
      ev.preventDefault();
      showBOPraticheView(btn.dataset.boPraticheView || 'all');
    });
  });
  showBOPraticheView(boState.activePraticheView || 'all');
}

function showBackofficeOp(opName) {
  const active = String(opName || 'state');
  boState.activeOp = active;
  document.querySelectorAll('[data-bo-op]').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.boOp === active);
  });
  document.querySelectorAll('[data-bo-op-panel]').forEach((panel) => {
    const isTarget = panel.dataset.boOpPanel === active;
    panel.classList.toggle('hidden', !isTarget);
    panel.hidden = !isTarget;
  });
}

function wireBackofficeOpsMenu() {
  const menu = document.getElementById('boOpsMenu');
  if (!menu) return;
  menu.querySelectorAll('[data-bo-op]').forEach((btn) => {
    btn.addEventListener('click', () => showBackofficeOp(btn.dataset.boOp || 'state'));
  });
  showBackofficeOp(boState.activeOp || 'state');
}

function wireBackofficeFiltersToggle() {
  const toggle = document.getElementById('btnBOToggleAdvancedFilters');
  const panel = document.getElementById('boAdvancedFilters');
  if (!toggle || !panel) return;

  const sync = () => {
    const expanded = !panel.classList.contains('hidden');
    toggle.setAttribute('aria-expanded', expanded ? 'true' : 'false');
    toggle.textContent = expanded ? 'Nascondi filtri avanzati' : 'Filtri avanzati';
  };

  sync();
  toggle.addEventListener('click', () => {
    panel.classList.toggle('hidden');
    panel.hidden = panel.classList.contains('hidden');
    sync();
  });
}

function showSettingsTab(tabName) {
  const active = String(tabName || 'sicurezza');
  settingsState.activeTab = active;
  document.querySelectorAll('[data-settings-tab]').forEach((btn) => {
    const isActive = btn.dataset.settingsTab === active;
    btn.classList.toggle('active', isActive);
    btn.setAttribute('aria-selected', isActive ? 'true' : 'false');
  });
  document.querySelectorAll('[data-settings-panel]').forEach((panel) => {
    const isActive = panel.dataset.settingsPanel === active;
    panel.classList.toggle('show', isActive);
    panel.classList.toggle('active', isActive);
    panel.classList.toggle('hidden', !isActive);
    panel.hidden = !isActive;
  });
}

function wireSettingsTabs() {
  const tabs = document.querySelectorAll('[data-settings-tab]');
  if (!tabs.length) return;
  tabs.forEach((btn) => {
    btn.addEventListener('click', (ev) => {
      ev.preventDefault();
      showSettingsTab(btn.dataset.settingsTab || 'sicurezza');
    });
  });
  showSettingsTab(settingsState.activeTab || 'sicurezza');
}

function renderSettingsAccountInfo() {
  const roleEl = document.getElementById('settingsRoleValue');
  const userEl = document.getElementById('settingsUserValue');
  const twoFAStatus = document.getElementById('settings2FAStatus');
  const twoFAHint = document.getElementById('settings2FAHint');
  const totpEnabled = Boolean(state.user?.totp_enabled ?? state.user?.totpEnabled);
  if (roleEl) roleEl.textContent = role() || 'ospite';
  if (userEl) userEl.textContent = state.user?.email || 'non autenticato';
  if (twoFAStatus) {
    twoFAStatus.classList.toggle('settings-pill--ok', totpEnabled);
    twoFAStatus.classList.toggle('settings-pill--warn', !totpEnabled);
    twoFAStatus.textContent = totpEnabled ? 'Attivo' : 'Non attivo';
  }
  if (twoFAHint) {
    twoFAHint.textContent = totpEnabled
      ? 'Il secondo fattore e attivo. Mantieni al sicuro i codici della tua app autenticatore.'
      : 'Il secondo fattore non e ancora abilitato su questo account.';
  }
}

function syncBoFiltersFromForm(payload) {
  boState.query = payload.q || '';
  boState.stato = payload.stato || '';
  boState.priorita = payload.priorita || '';
  boState.paese_dest = payload.paese_dest || '';
  boState.tipo_visto = payload.tipo_visto || '';
}

function computeStatusBreakdown(items) {
  const acc = {};
  items.forEach((row) => {
    const s = row.pratica?.stato || 'UNKNOWN';
    acc[s] = (acc[s] || 0) + 1;
  });
  return acc;
}

function computeTopActions(items, take) {
  const counts = {};
  items.forEach((e) => {
    const key = e.action || 'UNKNOWN';
    counts[key] = (counts[key] || 0) + 1;
  });
  return Object.entries(counts)
    .sort((a, b) => b[1] - a[1])
    .slice(0, take)
    .map(([k, v]) => `${k}:${v}`)
    .join(' | ');
}

async function loadBOPratiche(query = {}) {
  if (!hasBackofficeRole()) {
    notify('err', 'Azione riservata al backoffice');
    return [];
  }
  if (query && Object.keys(query).length > 0) {
    syncBoFiltersFromForm(query);
  }
  const params = new URLSearchParams();
  if (boState.query) params.set('q', boState.query);
  if (boState.stato) params.set('stato', boState.stato);
  if (boState.priorita) params.set('priorita', boState.priorita);
  if (boState.paese_dest) params.set('paese_dest', boState.paese_dest);
  if (boState.tipo_visto) params.set('tipo_visto', boState.tipo_visto);
  const data = await api(`/api/bo/pratiche?${params.toString()}`);
  const items = (data.items || []).map((row) => ({ ...row.pratica, richiedente: row.richiedente }));
  boState.items = items;
  if (boState.selectedPraticaId && !boState.items.some((item) => item.id === boState.selectedPraticaId)) {
    boState.selectedPraticaId = '';
  }
  els.kpiPratiche.textContent = String(data.total || 0);
  renderBOPraticheTable();
  renderBONewPratiche();
  out('BO pratiche caricate', {
    total: data.total || 0,
    by_status: computeStatusBreakdown(items.map((pratica) => ({ pratica }))),
    scope: boState.scope,
  });
  return items;
}

async function loadBOUsers() {
  if (!hasBackofficeRole()) {
    notify('err', 'Azione riservata al backoffice');
    return [];
  }
  const data = await api('/api/bo/utenti?page=1&page_size=200');
  const items = data.items || [];
  const clienti = items.filter((u) => String(u.ruolo || '').toUpperCase() === 'RICHIEDENTE');
  const operatori = items.filter((u) => String(u.ruolo || '').toUpperCase() === 'OPERATORE');
  const operatoriDisponibili = operatori.filter((u) => Boolean(u.attivo) && Boolean(u.email_verificata));
  const invitiPendenti = operatori.filter((u) => !u.email_verificata);
  boState.operators = operatoriDisponibili;

  const clientiContainer = document.getElementById('boUtentiClienti');
  if (clientiContainer) renderList(clientiContainer, clienti, (u) => `
    <article class="list-item">
      <h3>${u.email}</h3>
      <p>${u.nome || ''} ${u.cognome || ''}</p>
    </article>
  `);

  const operatoriContainer = document.getElementById('boUtentiOperatori');
  if (operatoriContainer) renderList(operatoriContainer, operatoriDisponibili, (u) => `
    <article class="list-item">
      <h3>${u.email}</h3>
      <p>${u.nome || ''} ${u.cognome || ''} | ${u.ruolo}</p>
    </article>
  `);

  const pendingContainer = document.getElementById('boUtentiOperatoriPending');
  if (pendingContainer) renderList(pendingContainer, invitiPendenti, (u) => `
    <article class="list-item">
      <h3>${u.email}</h3>
      <p>${u.nome || ''} ${u.cognome || ''} | Invito in attesa</p>
    </article>
  `);

  renderBOAssignOperators();
  renderBOPraticheTable();
  els.kpiUtenti.textContent = String(data.total || 0);
  out('BO utenti caricati', {
    total: data.total || 0,
    clienti: clienti.length,
    operatori_disponibili: operatoriDisponibili.length,
    inviti_pendenti: invitiPendenti.length,
  });
  return items;
}

async function loadBOAudit() {
  if (!hasBackofficeRole()) {
    notify('err', 'Azione riservata al backoffice');
    return [];
  }
  const data = await api('/api/bo/audit-events?page=1&page_size=30');
  const items = data.items || [];
  renderList(els.boAudit, items, (e) => `
    <article class="list-item">
      <h3>${e.action}</h3>
      <p>${e.actor_role || '-'} | ${e.resource} ${e.resource_id || ''}</p>
    </article>
  `);
  els.kpiAudit.textContent = String(data.total || 0);
  out('Audit eventi caricati', { total: data.total || 0, top_actions: computeTopActions(items, 3) });
  return items;
}

async function loadBODashboard() {
  const [pratiche, utenti, audit] = await Promise.all([loadBOPratiche(), loadBOUsers(), loadBOAudit()]);
  out('Dashboard BO aggiornata', {
    pratiche: pratiche.length,
    utenti: utenti.length,
    audit: audit.length,
  });
}

function wireForms() {
  document.getElementById('formLogin').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const data = await api('/api/auth/login', { method: 'POST', body: JSON.stringify(payload) });
        setSession({ accessToken: data.access_token, refreshToken: data.refresh_token, user: data.user });
        if (data.user?.ruolo === 'RICHIEDENTE') window.location.hash = '#richiedente';
        if (isBackofficeRoleName(data.user?.ruolo)) window.location.hash = '#backoffice';
        renderSessionInfo();
        applyRoleGuards();
        out('Login completato', { user: data.user });
        notify('ok', 'Login completato');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formRegister').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const data = await api('/api/auth/register', { method: 'POST', body: JSON.stringify(payload) });
        out('Registrazione completata', data);
        notify('ok', 'Registrazione completata');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formForgot').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const data = await api('/api/auth/forgot-password', { method: 'POST', body: JSON.stringify(payload) });
        out('Forgot password', data);
        notify('ok', 'Richiesta reset inviata');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formReset').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const data = await api('/api/auth/reset-password', { method: 'POST', body: JSON.stringify(payload) });
        out('Reset password', data);
        notify('ok', 'Password aggiornata');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('btn2FASetup').addEventListener('click', async (ev) => {
    try {
      await withBusy(ev.currentTarget, async () => {
        const data = await api('/api/auth/2fa/setup', { method: 'POST', body: '{}' });
        els.twofaOutput.textContent = JSON.stringify(data, null, 2);
        out('2FA setup', data);
        notify('ok', '2FA setup completato');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('form2FAEnable').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const data = await api('/api/auth/2fa/enable', { method: 'POST', body: JSON.stringify(payload) });
        state.user = { ...(state.user || {}), totp_enabled: true };
        renderSettingsAccountInfo();
        out('2FA enable', data);
        notify('ok', '2FA abilitato');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('form2FADisable').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const data = await api('/api/auth/2fa/disable', { method: 'POST', body: JSON.stringify(payload) });
        state.user = { ...(state.user || {}), totp_enabled: false };
        renderSettingsAccountInfo();
        out('2FA disable', data);
        notify('ok', '2FA disabilitato');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formCreatePratica').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    const feedback = document.getElementById('richCreateFeedback');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const data = await api('/api/pratiche/', {
          method: 'POST',
          body: JSON.stringify({
            tipo_visto: payload.tipo_visto,
            paese_dest: payload.paese_dest,
            dati_anagrafici: {},
            dati_passaporto: {},
          }),
        });
        out('Pratica creata', data);
        notify('ok', 'Pratica creata');
        const createdID = data.id || data.pratica?.id || '';
        if (feedback) {
          const code = data.codice || data.pratica?.codice || createdID || 'nuova pratica';
          feedback.hidden = false;
          feedback.textContent = `Pratica creata con successo: ${code}`;
        }
        showRichTab('active');
        await loadMiePratiche(createdID);
      });
    } catch (err) {
      if (feedback) {
        feedback.hidden = false;
        feedback.textContent = `Errore creazione pratica: ${extractErrMessage(err)}`;
      }
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('btnLoadMiePratiche').addEventListener('click', async (ev) => {
    try {
      await withBusy(ev.currentTarget, () => loadMiePratiche());
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formDocUpload').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        if (!payload.id) {
          notify('err', 'Seleziona prima una pratica');
          return;
        }
        const data = await api(`/api/pratiche/${payload.id}/documenti`, {
          method: 'POST',
          body: JSON.stringify({
            tipo: payload.tipo,
            nome_file: payload.nome_file,
            mime_type: payload.mime_type,
            dimensione: Number(payload.dimensione),
          }),
        });
        out('Documento aggiunto', data);
        notify('ok', 'Documento registrato');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formBOFilters').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        boState.page = 1;
        await loadBOPratiche({ q: payload.q, stato: payload.stato, priorita: payload.priorita, paese_dest: payload.paese_dest, tipo_visto: payload.tipo_visto });
        notify('ok', 'Filtri applicati');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('btnLoadUsers').addEventListener('click', async (ev) => {
    try {
      await withBusy(ev.currentTarget, () => loadBOUsers());
      notify('ok', 'Utenti aggiornati');
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  const inviteForm = document.getElementById('formBOInviteOperator');
  if (inviteForm) {
    inviteForm.addEventListener('submit', async (ev) => {
      ev.preventDefault();
      const submit = ev.currentTarget.querySelector('button[type="submit"]');
      const output = document.getElementById('boInviteOperatorOutput');
      try {
        await withBusy(submit, async () => {
          const payload = formJson(ev.currentTarget);
          const data = await api('/api/bo/operatori/inviti', {
            method: 'POST',
            body: JSON.stringify({
              email: payload.email,
              nome: payload.nome,
              cognome: payload.cognome,
            }),
          });
          if (output) {
            output.textContent = data.invite_url
              ? `Invito creato: ${data.invite_url}`
              : `Invito creato. Token: ${data.invite_token || '-'}`;
          }
          out('BO invito operatore', data);
          notify('ok', 'Invito operatore inviato');
          ev.currentTarget.reset();
          setBOInviteOperatorBox(false);
          showBOUsersView('operatori');
          await loadBOUsers();
        });
      } catch (err) {
        if (output) output.textContent = extractErrMessage(err);
        notify('err', extractErrMessage(err));
      }
    });
  }

  document.getElementById('btnLoadAudit').addEventListener('click', async (ev) => {
    try {
      await withBusy(ev.currentTarget, () => loadBOAudit());
      notify('ok', 'Audit aggiornato');
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formBOChangeState').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const praticaID = resolveBOPraticaID(payload.id);
        if (!praticaID) throw new Error('Seleziona prima una pratica.');
        const data = await api(`/api/bo/pratiche/${praticaID}/stato`, {
          method: 'PATCH',
          body: JSON.stringify({ stato: payload.stato, nota: payload.nota }),
        });
        out('BO cambio stato', data);
        notify('ok', 'Stato pratica aggiornato');
        await loadBOPratiche();
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formBOAddNote').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const praticaID = resolveBOPraticaID(payload.id);
        if (!praticaID) throw new Error('Seleziona prima una pratica.');
        const data = await api(`/api/bo/pratiche/${praticaID}/note`, {
          method: 'POST',
          body: JSON.stringify({
            messaggio: payload.messaggio,
            interna: payload.interna === 'true',
          }),
        });
        out('BO aggiungi nota', data);
        notify('ok', 'Nota aggiunta');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formBOAssign').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const praticaID = resolveBOPraticaID(payload.id);
        if (!praticaID) throw new Error('Seleziona prima una pratica.');
        const data = await api(`/api/bo/pratiche/${praticaID}/assegna`, {
          method: 'POST',
          body: JSON.stringify({ operatore_id: payload.operatore_id }),
        });
        out('BO assegna operatore', data);
        notify('ok', 'Operatore assegnato');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formBORequestDoc').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const praticaID = resolveBOPraticaID(payload.id);
        if (!praticaID) throw new Error('Seleziona prima una pratica.');
        const data = await api(`/api/bo/pratiche/${praticaID}/richiedi-doc`, {
          method: 'POST',
          body: JSON.stringify({ documento: payload.documento, nota: payload.nota }),
        });
        out('BO richiedi documento', data);
        notify('ok', 'Richiesta documento inviata');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  document.getElementById('formBOPaymentLink').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const submit = ev.currentTarget.querySelector('button[type="submit"]');
    try {
      await withBusy(submit, async () => {
        const payload = formJson(ev.currentTarget);
        const praticaID = resolveBOPraticaID(payload.id);
        if (!praticaID) throw new Error('Seleziona prima una pratica.');
        const data = await api(`/api/bo/pratiche/${praticaID}/link-pagamento`, {
          method: 'POST',
          body: JSON.stringify({ provider: payload.provider || 'stripe', importo: Number(payload.importo) }),
        });
        out('BO crea link pagamento', data);
        notify('ok', 'Link pagamento creato');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });
}

function wireSessionButtons() {
  els.btnSaveApiBase.addEventListener('click', () => {
    saveApiBase(els.apiBaseInput.value);
    renderSessionInfo();
    notify('ok', state.apiBase ? `API base salvata: ${state.apiBase}` : 'API base reimpostata su locale');
    out('API base aggiornata', { api_base: state.apiBase || '(relativa)' });
  });

  const runLogout = async (buttonEl) => {
    await withBusy(buttonEl, async () => {
      if (!state.refreshToken) {
        clearSession();
        renderSessionInfo();
        applyRoleGuards();
        out('Logout locale', { status: 'no_refresh_token' });
        notify('ok', 'Sessione locale chiusa');
        return;
      }
      const data = await api('/api/auth/logout', {
        method: 'POST',
        body: JSON.stringify({ refresh_token: state.refreshToken }),
      });
      clearSession();
      window.location.hash = '#auth';
      renderSessionInfo();
      applyRoleGuards();
      out('Logout server', data);
      notify('ok', 'Logout completato');
    });
  };

  els.btnLogout.addEventListener('click', async (ev) => {
    try {
      await runLogout(ev.currentTarget);
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

  if (els.boBtnLogout) {
    els.boBtnLogout.addEventListener('click', async (ev) => {
      try {
        await runLogout(ev.currentTarget);
      } catch (err) {
        notify('err', extractErrMessage(err));
      }
    });
  }

  if (els.btnRefreshSession) {
    els.btnRefreshSession.addEventListener('click', async (ev) => {
      try {
        await withBusy(ev.currentTarget, async () => {
          if (!state.refreshToken) {
            out('Refresh saltato', { reason: 'missing refresh token' });
            notify('err', 'Refresh token mancante');
            return;
          }
          const data = await api('/api/auth/refresh', {
            method: 'POST',
            body: JSON.stringify({ refresh_token: state.refreshToken }),
          });
          setSession({ accessToken: data.access_token, refreshToken: data.refresh_token, user: state.user });
          renderSessionInfo();
          applyRoleGuards();
          out('Sessione aggiornata', { status: 'ok' });
          notify('ok', 'Sessione aggiornata');
        });
      } catch (err) {
        notify('err', extractErrMessage(err));
      }
    });
  }

  if (els.btnSettings) {
    const goSettings = () => {
      if (!hasActiveSession()) return;
      window.location.hash = '#profilo';
      renderSettingsAccountInfo();
      showSettingsTab('sicurezza');
      setSectionFromHash();
      applyRoleGuards();
    };
    els.btnSettings.addEventListener('click', goSettings);
    if (els.boBtnSettings) els.boBtnSettings.addEventListener('click', goSettings);
  }

  const btnSettingsBack = document.getElementById('btnSettingsBack');
  if (btnSettingsBack) {
    btnSettingsBack.addEventListener('click', () => {
      window.location.hash = hasBackofficeRole() ? '#backoffice' : '#richiedente';
      setSectionFromHash();
      applyRoleGuards();
    });
  }
}

export function initApp(bootMessage) {
  if (!hasActiveSession() && (state.accessToken || state.refreshToken || state.user)) {
    clearSession();
  }

  const wantsBackofficePath = window.location.pathname.toLowerCase().startsWith('/backoffice');
  let deniedBackofficeByRole = false;
  if (wantsBackofficePath) {
    if (hasBackofficeRole()) {
      window.location.hash = '#backoffice';
    } else if (state.accessToken && hasRichiedenteRole()) {
      window.location.hash = '#richiedente';
      deniedBackofficeByRole = true;
    } else {
      window.location.hash = '#auth';
    }
  }

  renderSessionInfo();
  boState.pageSize = readBOPageSize();
  renderSettingsAccountInfo();
  wireTabs();
  wireRichTabs();
  wireAuthViews();
  wireSettingsTabs();
  wireBackofficeTabs();
  wireBOPraticheTabs();
  wireBOUsersTabs();
  wireBOInviteOperatorToggle();
  wireBackofficeMenu();
  wireBackofficeOpsMenu();
  wireBackofficeFiltersToggle();
  wireForms();
  wireSessionButtons();
  wireActivePracticeFilters();
  wireDocUploadMetadata();
  loadCountriesIntoSelect().catch(() => {});
  setSectionFromHash();
  applyRoleGuards();
  if (deniedBackofficeByRole) {
    notify('err', 'Il tuo account non ha accesso al backoffice.');
  }
  window.addEventListener('hashchange', () => {
    setSectionFromHash();
    applyRoleGuards();
  });

  if (hasBackofficeRole()) {
    boState.scope = role() === 'ADMIN' ? 'all' : 'mine';
    document.querySelectorAll('[data-bo-scope]').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.boScope === boState.scope);
    });
    loadBODashboard().catch((err) => notify('err', extractErrMessage(err)));
  }

  if (hasRichiedenteRole()) {
    loadMiePratiche().catch((err) => notify('err', extractErrMessage(err)));
  }

  out(bootMessage, {
    has_token: Boolean(state.accessToken),
    role: role() || 'ospite',
    api_base: state.apiBase || '(relativa)',
  });
}
