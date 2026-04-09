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
const FINAL_PRACTICE_STATUSES = new Set(['APPROVATA', 'RIFIUTATA', 'ANNULLATA', 'COMPLETATA', 'CHIUSA']);
const activePracticeFilters = {
  status: 'ALL',
  query: '',
  sort: 'date_desc',
};

const boState = {
  scope: 'mine',
  query: '',
  stato: '',
  priorita: '',
  paese_dest: '',
  tipo_visto: '',
  items: [],
  selectedPraticaId: '',
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
  if (p.operatore_id === getBackofficeUserId()) return 'Assegnata a me';
  return 'Assegnata';
}

function getStatusBadgeClass(status) {
  const s = String(status || '').toUpperCase();
  if (s === 'BOZZA' || s === 'INVIATA') return 'bo-status-badge';
  if (s.includes('RIFIUT')) return 'bo-status-badge err';
  if (s.includes('APPROV') || s.includes('VISTO_EMESSO') || s.includes('COMPLETATA')) return 'bo-status-badge ok';
  return 'bo-status-badge';
}

function setBackofficeFormIds(praticaID) {
  ['formBOChangeState', 'formBOAddNote', 'formBOAssign', 'formBORequestDoc', 'formBOPaymentLink'].forEach((formId) => {
    const input = document.querySelector(`#${formId} input[name="id"]`);
    if (input) input.value = praticaID || '';
  });
}

function renderBackofficeDetail(pratica) {
  const detail = document.getElementById('boPraticaDetail');
  if (!detail) return;
  if (!pratica) {
    detail.innerHTML = '<p class="helper-text">Seleziona una pratica dalla tabella per vedere i dettagli.</p>';
    detail.classList.add('hidden');
    return;
  }

  detail.classList.remove('hidden');
  detail.innerHTML = `
    <div class="bo-detail-head">
      <div>
        <h3>${pratica.codice || pratica.id}</h3>
        <p>${pratica.tipo_visto || '-'} | ${pratica.paese_dest || '-'}</p>
      </div>
      <button class="btn btn-ghost" type="button" data-bo-action="back">Torna elenco</button>
    </div>
    <p><span class="bo-status-badge">${pratica.stato || '-'}</span> <span class="bo-status-badge">${getOperatorLabel(pratica)}</span></p>
    <p class="helper-text">Aggiornata ${new Date(pratica.aggiornato_il || pratica.creato_il).toLocaleString()}</p>
    <div class="bo-detail-actions">
      <button class="btn btn-solid" type="button" data-bo-action="assign-me">Assegna a me</button>
      <button class="btn btn-ghost" type="button" data-bo-action="prefill-state">Usa nei controlli</button>
    </div>
    <pre class="mono-box">${JSON.stringify({
      id: pratica.id,
      codice: pratica.codice,
      stato: pratica.stato,
      priorita: pratica.priorita,
      operatore_id: pratica.operatore_id || '',
      tipo_visto: pratica.tipo_visto,
      paese_dest: pratica.paese_dest,
      note_interne: pratica.note_interne || '',
      note_richiedente: pratica.note_richiedente || '',
      creato_il: pratica.creato_il,
      aggiornato_il: pratica.aggiornato_il,
    }, null, 2)}</pre>
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

  detail.querySelector('[data-bo-action="prefill-state"]').addEventListener('click', () => {
    setBackofficeFormIds(pratica.id);
    notify('ok', `Pratica ${pratica.codice || pratica.id} pronta per operazioni`);
  });

  setBackofficeFormIds(pratica.id);
}

function renderBOPraticheTable() {
  const container = els.boPratiche;
  const rows = sortBackofficePratiche(boState.items.filter(matchesBackofficeFilters));
  const selectedId = boState.selectedPraticaId;
  if (!container) return;

  if (!rows.length) {
    container.innerHTML = '<div class="list-item"><p>Nessuna pratica trovata</p></div>';
    renderBackofficeDetail(null);
    return;
  }

  container.innerHTML = `
    <table class="bo-table">
      <thead>
        <tr>
          <th>Codice</th>
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
        </div>
        <span class="bo-status-badge">${p.stato || '-'}</span>
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
      document.querySelectorAll('[data-bo-scope]').forEach((other) => other.classList.toggle('active', other === btn));
      await loadBOPratiche();
    });
  });
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
  const data = await api('/api/bo/utenti?page=1&page_size=20');
  const items = data.items || [];
  renderList(els.boUtenti, items, (u) => `
    <article class="list-item">
      <h3>${u.email}</h3>
      <p>${u.nome || ''} ${u.cognome || ''} | ${u.ruolo}</p>
    </article>
  `);
  els.kpiUtenti.textContent = String(data.total || 0);
  out('BO utenti caricati', { total: data.total || 0 });
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
        await loadMiePratiche(createdID);
      });
    } catch (err) {
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
        const data = await api(`/api/bo/pratiche/${payload.id}/stato`, {
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
        const data = await api(`/api/bo/pratiche/${payload.id}/note`, {
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
        const data = await api(`/api/bo/pratiche/${payload.id}/assegna`, {
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
        const data = await api(`/api/bo/pratiche/${payload.id}/richiedi-doc`, {
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
        const data = await api(`/api/bo/pratiche/${payload.id}/link-pagamento`, {
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

  els.btnLogout.addEventListener('click', async (ev) => {
    try {
      await withBusy(ev.currentTarget, async () => {
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
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });

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
    els.btnSettings.addEventListener('click', () => {
      if (!hasActiveSession()) return;
      window.location.hash = '#profilo';
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
  wireTabs();
  wireAuthViews();
  wireBackofficeMenu();
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
