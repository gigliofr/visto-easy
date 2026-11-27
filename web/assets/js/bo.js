// Backoffice: coda di lavoro, elenco completo, dettaglio, utenti, audit.
import { api, apiBlob, errorMessage } from './http.js';
import { $, html, icon, on, render, fresh } from './dom.js';
import { t } from './i18n.js';
import { toast, busy } from './toast.js';
import { state } from './session.js';
import {
  STATI, FINAL_STATI, TRANSIZIONI, statoLabel, cosaFare, isWaitingOnClient, scadenzaIndicativa, scadenzaTone,
  fmtRelativeDeadline, fmtDate, fmtDateTime, fmtShort, countryName, flag, tipoVistoLabel, docTipoLabel,
  docStatoLabel, docStatoTone, fmtBytes, fmtMoney, TIPI_DOCUMENTO, fullName, isoToDisplay,
} from './domain.js';
import { SECTIONS, FIELD_BY_CODE, valuesFromPratica, fieldLabel, optionLabel } from './schema.js';
import {
  badge, statoBadge, emptyState, loading, errorBox, pageHeader, definitionList,
} from './components.js';
import { openDocumento } from './upload.js';

const BO_ROLES = new Set(['OPERATORE', 'SUPERVISORE', 'ADMIN']);

const queueState = { filter: 'mine', q: '' };
const allState = { page: 1, stato: '', q: '', pageSize: 25 };

let usersCache = { at: 0, items: [] };

async function loadUsers(force = false) {
  if (!force && usersCache.items.length && Date.now() - usersCache.at < 60000) return usersCache.items;
  const res = await api('/api/bo/utenti?page=1&page_size=200');
  usersCache = { at: Date.now(), items: res?.items || [] };
  return usersCache.items;
}

function operatorsFrom(users) {
  return users.filter((u) => BO_ROLES.has(u.ruolo));
}

function userName(users, id) {
  if (!id) return '';
  if (id === state.user?.id) return t('bo.me');
  const u = users.find((x) => x.id === id);
  return u ? fullName(u) : '';
}

async function loadAllActive() {
  const res = await api('/api/bo/pratiche?page=1&page_size=200&sort_by=aggiornato_il&sort_order=desc');
  return (res?.items || []).map((row) => ({ ...row.pratica, _richiedente: row.richiedente }));
}

function deadlineCell(p) {
  const d = scadenzaIndicativa(p);
  if (!d) return html`<span class="muted">-</span>`;
  const tone = scadenzaTone(d);
  return html`<span class="deadline deadline--${tone}" title="${t('bo.deadlineHint')}">${fmtRelativeDeadline(d)}</span>`;
}

function richiedenteName(p) {
  return fullName(p._richiedente) || p._richiedente?.email || '-';
}

export function queueCount(pratiche) {
  return pratiche.filter((p) => p.stato !== 'BOZZA' && !FINAL_STATI.has(p.stato)).length;
}

// ---------------------------------------------------------------------------
// Coda di lavoro
// ---------------------------------------------------------------------------

export async function viewQueue(root) {
  root = fresh(root);
  render(root, loading());
  let pratiche;
  let users = [];
  try {
    [pratiche, users] = await Promise.all([loadAllActive(), loadUsers().catch(() => [])]);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/bo/coda'));
    return;
  }
  const me = state.user?.id;
  const active = pratiche.filter((p) => p.stato !== 'BOZZA' && !FINAL_STATI.has(p.stato));
  active.forEach((p) => { p._deadline = scadenzaIndicativa(p); });
  active.sort((a, b) => (a._deadline?.getTime() ?? Infinity) - (b._deadline?.getTime() ?? Infinity));

  const now = Date.now();
  const kpi = {
    mine: active.filter((p) => p.operatore_id === me).length,
    due: active.filter((p) => p._deadline && p._deadline.getTime() - now < 48 * 3600000 && !isWaitingOnClient(p.stato)).length,
    waiting: active.filter((p) => isWaitingOnClient(p.stato)).length,
    unassigned: active.filter((p) => !p.operatore_id).length,
  };

  const filters = {
    mine: (p) => p.operatore_id === me,
    unassigned: (p) => !p.operatore_id,
    review: (p) => p.stato === 'INVIATA' || p.stato === 'IN_LAVORAZIONE',
    paid: (p) => p.stato === 'PAGAMENTO_RICEVUTO',
    waiting: (p) => isWaitingOnClient(p.stato),
    all: () => true,
  };
  if (queueState.filter === 'mine' && !kpi.mine && active.length) queueState.filter = 'all';
  const q = queueState.q.trim().toLowerCase();
  const rows = active.filter(filters[queueState.filter] || filters.all).filter((p) => {
    if (!q) return true;
    const hay = [p.codice, richiedenteName(p), p._richiedente?.email, countryName(p.paese_dest), p.paese_dest].join(' ').toLowerCase();
    return hay.includes(q);
  });
  const next = active.find((p) => !p.operatore_id && !isWaitingOnClient(p.stato));

  render(root, html`
    ${pageHeader(t('bo.queue'), t('bo.queueSub'), next ? html`<button type="button" class="btn btn--primary" data-take="${next.id}">${t('bo.takeNext')}</button>` : '')}
    <div class="kpis">
      ${[['mine', t('bo.kpi.mine')], ['due', t('bo.kpi.due')], ['waiting', t('bo.kpi.waiting')], ['unassigned', t('bo.kpi.unassigned')]].map(([k, label]) => html`
        <div class="kpi ${k === 'due' && kpi.due ? 'kpi--attention' : ''}"><p class="kpi__label">${label}</p><p class="kpi__value">${kpi[k]}</p></div>`)}
    </div>
    <div class="toolbar">
      <label class="sr-only" for="boQ">${t('common.search')}</label>
      <input id="boQ" type="search" class="toolbar__search" placeholder="${t('bo.searchPh')}" value="${queueState.q}">
      <div class="pills" role="group" aria-label="${t('bo.filterAria')}">
        ${['mine', 'unassigned', 'review', 'paid', 'waiting', 'all'].map((k) => html`<button type="button" class="pill ${queueState.filter === k ? 'is-active' : ''}" aria-pressed="${queueState.filter === k ? 'true' : 'false'}" data-qfilter="${k}">${t(`bo.f.${k}`)}</button>`)}
      </div>
    </div>
    ${rows.length ? html`<div class="table-wrap card card--flush">
      <table class="table">
        <thead><tr>
          <th scope="col">${t('bo.col.deadline')}</th><th scope="col">${t('bo.col.case')}</th><th scope="col">${t('bo.col.client')}</th>
          <th scope="col">${t('bo.col.dest')}</th><th scope="col">${t('bo.col.state')}</th><th scope="col">${t('bo.col.todo')}</th>
          <th scope="col">${t('bo.col.operator')}</th><th scope="col"><span class="sr-only">${t('bo.col.actions')}</span></th>
        </tr></thead>
        <tbody>
          ${rows.map((p) => html`<tr class="${p.priorita === 'URGENTE' ? 'is-urgent' : ''}">
            <td>${deadlineCell(p)}</td>
            <td class="mono">${p.codice || p.id}${p.priorita && p.priorita !== 'NORMALE' ? html` ${badge(t(`prio.${p.priorita}`), p.priorita === 'URGENTE' ? 'danger' : 'warning')}` : ''}</td>
            <td>${richiedenteName(p)}</td>
            <td>${flag(p.paese_dest)} ${countryName(p.paese_dest)} · ${tipoVistoLabel(p.tipo_visto)}</td>
            <td>${statoBadge(p.stato)}</td>
            <td>${cosaFare(p)}</td>
            <td>${userName(users, p.operatore_id) || html`<span class="muted">${t('bo.unassigned')}</span>`}</td>
            <td class="table__actions"><a class="btn btn--secondary btn--sm" href="#/bo/pratiche/${p.id}">${t('rich.open')}</a></td>
          </tr>`)}
        </tbody>
      </table>
    </div>
    <p class="muted small">${t('bo.deadlineNote')}</p>` : emptyState(t('bo.queueEmpty'), t('bo.queueEmptyText'))}
  `);

  on(root, 'click', '[data-qfilter]', (_ev, btn) => {
    queueState.filter = btn.dataset.qfilter;
    viewQueue(root);
  });
  const search = $('#boQ', root);
  let timer = null;
  search?.addEventListener('input', () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      queueState.q = search.value;
      viewQueue(root).then(() => {
        const s = document.getElementById('boQ');
        if (s) {
          s.focus();
          s.setSelectionRange(s.value.length, s.value.length);
        }
      });
    }, 250);
  });
  on(root, 'click', '[data-take]', async (_ev, btn) => {
    await busy(btn, async () => {
      try {
        await api(`/api/bo/pratiche/${encodeURIComponent(btn.dataset.take)}/assegna`, { method: 'POST', json: { operatore_id: me } });
        window.location.hash = `#/bo/pratiche/${btn.dataset.take}`;
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  });
}

// ---------------------------------------------------------------------------
// Tutte le pratiche
// ---------------------------------------------------------------------------

function allQuery() {
  const params = new URLSearchParams({ page: String(allState.page), page_size: String(allState.pageSize), sort_by: 'aggiornato_il', sort_order: 'desc' });
  if (allState.stato) params.set('stato', allState.stato);
  if (allState.q) params.set('q', allState.q);
  return params;
}

export async function viewAll(root) {
  root = fresh(root);
  render(root, loading());
  let res;
  let users = [];
  try {
    [res, users] = await Promise.all([api(`/api/bo/pratiche?${allQuery()}`), loadUsers().catch(() => [])]);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/bo/pratiche'));
    return;
  }
  const items = (res?.items || []).map((row) => ({ ...row.pratica, _richiedente: row.richiedente }));
  const total = res?.total || 0;
  const pages = Math.max(1, Math.ceil(total / allState.pageSize));

  render(root, html`
    ${pageHeader(t('bo.all'), t('bo.allSub', { n: total }), html`<button type="button" class="btn btn--secondary" data-export>${icon('download', 16)} ${t('bo.exportCsv')}</button>`)}
    <form class="toolbar" id="allFilters">
      <label class="sr-only" for="allQ">${t('common.search')}</label>
      <input id="allQ" name="q" type="search" class="toolbar__search" placeholder="${t('bo.searchPh')}" value="${allState.q}">
      <label class="sr-only" for="allStato">${t('bo.col.state')}</label>
      <select id="allStato" name="stato">
        <option value="">${t('bo.anyState')}</option>
        ${STATI.map((s) => html`<option value="${s}" ${allState.stato === s ? 'selected' : ''}>${statoLabel(s)}</option>`)}
      </select>
      <button type="submit" class="btn btn--secondary">${t('common.filter')}</button>
    </form>
    ${items.length ? html`<div class="table-wrap card card--flush">
      <table class="table">
        <thead><tr>
          <th scope="col">${t('bo.col.case')}</th><th scope="col">${t('bo.col.client')}</th><th scope="col">${t('bo.col.dest')}</th>
          <th scope="col">${t('bo.col.state')}</th><th scope="col">${t('bo.col.operator')}</th><th scope="col">${t('bo.col.updated')}</th>
          <th scope="col"><span class="sr-only">${t('bo.col.actions')}</span></th>
        </tr></thead>
        <tbody>${items.map((p) => html`<tr>
          <td class="mono">${p.codice || p.id}</td>
          <td>${richiedenteName(p)}<br><span class="muted small">${p._richiedente?.email || ''}</span></td>
          <td>${flag(p.paese_dest)} ${countryName(p.paese_dest)} · ${tipoVistoLabel(p.tipo_visto)}</td>
          <td>${statoBadge(p.stato)}</td>
          <td>${userName(users, p.operatore_id) || html`<span class="muted">-</span>`}</td>
          <td>${fmtDateTime(p.aggiornato_il)}</td>
          <td class="table__actions"><a class="btn btn--secondary btn--sm" href="#/bo/pratiche/${p.id}">${t('rich.open')}</a></td>
        </tr>`)}</tbody>
      </table>
    </div>
    <nav class="pager" aria-label="${t('common.pagination')}">
      <button type="button" class="btn btn--secondary btn--sm" data-page="${allState.page - 1}" ${allState.page <= 1 ? 'disabled' : ''}>${icon('back', 14)} ${t('common.prev')}</button>
      <span class="muted">${t('common.pageOf', { n: allState.page, total: pages })}</span>
      <button type="button" class="btn btn--secondary btn--sm" data-page="${allState.page + 1}" ${allState.page >= pages ? 'disabled' : ''}>${t('common.next')} ${icon('next', 14)}</button>
    </nav>` : emptyState(t('bo.noResults'), t('bo.noResultsText'))}
  `);

  $('#allFilters', root)?.addEventListener('submit', (ev) => {
    ev.preventDefault();
    allState.q = $('#allQ', root).value.trim();
    allState.stato = $('#allStato', root).value;
    allState.page = 1;
    viewAll(root);
  });
  on(root, 'click', '[data-page]', (_ev, btn) => {
    allState.page = Number(btn.dataset.page) || 1;
    viewAll(root);
  });
  on(root, 'click', '[data-export]', async (_ev, btn) => {
    await busy(btn, async () => {
      try {
        const params = allQuery();
        params.delete('page');
        params.delete('page_size');
        downloadBlob(await apiBlob(`/api/bo/report.csv?${params}`), 'pratiche.csv');
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  });
}

function downloadBlob(blob, name) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// ---------------------------------------------------------------------------
// Dettaglio pratica (backoffice)
// ---------------------------------------------------------------------------

function isInternalEvt(e) {
  return e.metadata && (e.metadata.scope === 'interna' || e.metadata.interna === true);
}

function evtLabel(e, users) {
  switch (e.tipo_evento) {
    case 'PRATICA_CREATA': return t('evt.created');
    case 'STATO_CAMBIATO': return t('evt.stateFrom', { from: statoLabel(e.da_stato), to: statoLabel(e.a_stato) });
    case 'NOTA_AGGIUNTA': return isInternalEvt(e) ? t('evt.internalNote') : t('evt.message');
    case 'ASSEGNAZIONE_OPERATORE': return e.metadata?.operatore_id === state.user?.id ? t('evt.assignedToMe') : t('evt.assignedTo', { name: userName(users, e.metadata?.operatore_id) || t('bo.operator') });
    default: return String(e.tipo_evento || '').replaceAll('_', ' ').toLowerCase();
  }
}

// Le transizioni verso integrazione e pagamento passano dai rispettivi moduli.
function manualTransitions(stato) {
  return (TRANSIZIONI[stato] || []).filter((s) => s !== 'INTEGRAZIONE_RICHIESTA' && s !== 'ATTENDE_PAGAMENTO' && s !== 'INVIATA');
}

function valueText(code, v) {
  const f = FIELD_BY_CODE[code];
  if (!f || v === '' || v === undefined) return v;
  if (f.type === 'country') return `${flag(v)} ${countryName(v)}`;
  if (f.type === 'select') return optionLabel(code, v);
  if (f.type === 'date') return isoToDisplay(v);
  return v;
}

export async function viewBODettaglio(root, params) {
  root = fresh(root);
  render(root, loading());
  let p;
  let users = [];
  try {
    [p, users] = await Promise.all([
      api(`/api/bo/pratiche/${encodeURIComponent(params.id)}`),
      loadUsers().catch(() => []),
    ]);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/bo/coda'));
    return;
  }
  const richiedente = users.find((u) => u.id === p.utente_id);
  const operators = operatorsFrom(users);
  const values = valuesFromPratica(p);
  const eventi = p.eventi || [];
  const notes = eventi.filter((e) => e.tipo_evento === 'NOTA_AGGIUNTA');
  const transitions = manualTransitions(p.stato);
  const deadline = scadenzaIndicativa(p);
  const me = state.user?.id;
  const filledSections = SECTIONS.filter((sec) => sec.codes.some((c) => String(values[c] || '').trim() && !(c === '3.01' || c === '3.02')));
  // Dati salvati con il formato precedente (chiavi libere, non codici di schema).
  const legacy = [];
  const pushLegacy = (obj) => Object.entries(obj || {}).forEach(([k, v]) => {
    if (k === 'attributi_compilazione' || FIELD_BY_CODE[k] || v === null || v === '' || typeof v === 'object') return;
    legacy.push([k.replaceAll('_', ' '), String(v)]);
  });
  pushLegacy(p.dati_anagrafici);
  pushLegacy(p.dati_passaporto);

  render(root, html`
    <a class="back-link" href="#/bo/coda">${icon('back', 16)} ${t('bo.queue')}</a>
    <div class="detail-head">
      <div>
        <p class="eyebrow mono">${p.codice || p.id}${p.priorita ? html` · ${t(`prio.${p.priorita}`)}` : ''}</p>
        <h1>${flag(p.paese_dest)} ${countryName(p.paese_dest)} · ${tipoVistoLabel(p.tipo_visto)}</h1>
        <p class="muted">${fullName(richiedente)}${richiedente?.email ? ` · ${richiedente.email}` : ''}</p>
      </div>
      <div class="detail-head__side">
        ${statoBadge(p.stato)}
        ${deadline ? html`<p class="small">${t('bo.col.deadline')}: ${deadlineCell(p)}</p>` : ''}
      </div>
    </div>
    ${cosaFare(p) ? html`<div class="callout callout--info"><span class="callout__icon">${icon('list', 20)}</span><div class="callout__body"><p class="callout__title">${t('bo.col.todo')}</p><p>${cosaFare(p)}</p></div></div>` : ''}

    <div class="detail-layout detail-layout--bo">
      <div class="detail-main">
        <section class="card">
          <div class="card__head"><h2 class="card__title">${t('det.docs')}</h2><span class="muted small">${t('det.docsCount', { n: (p.documenti || []).length })}</span></div>
          ${(p.documenti || []).length ? html`<ul class="doc-list">${p.documenti.map((d) => html`<li class="doc-row">
            <span class="doc-row__icon">${icon('file', 18)}</span>
            <div class="doc-row__body"><p class="doc-row__title">${docTipoLabel(d.tipo)}</p><p class="doc-row__meta">${d.nome_file} · ${d.mime_type} · ${fmtBytes(d.dimensione)} · ${fmtShort(d.caricato_il)}</p></div>
            ${badge(docStatoLabel(d.stato_validazione), docStatoTone(d.stato_validazione))}
            <button type="button" class="icon-btn" data-doc-preview="${d.id}" aria-label="${t('bo.preview', { name: d.nome_file })}">${icon('eye', 18)}</button>
            <button type="button" class="icon-btn" data-doc-download="${d.id}" aria-label="${t('det.openDoc', { name: d.nome_file })}">${icon('download', 18)}</button>
          </li>`)}</ul>` : html`<p class="muted">${t('wiz.noDocs')}</p>`}
        </section>

        <section class="card">
          <h2 class="card__title">${t('bo.data')}</h2>
          ${filledSections.length ? filledSections.map((s, i) => html`<details class="data-section" ${i < 3 ? 'open' : ''}>
            <summary>${t(`sec.${s.key}`)}</summary>
            ${definitionList(s.codes.filter((c) => String(values[c] || '').trim()).map((c) => [fieldLabel(c), valueText(c, values[c])]))}
          </details>`) : ''}
          ${legacy.length ? html`<details class="data-section" open>
            <summary>${t('bo.legacyData')}</summary>
            ${definitionList(legacy)}
          </details>` : ''}
          ${!filledSections.length && !legacy.length ? html`<p class="muted">${t('bo.noData')}</p>` : ''}
        </section>

        <section class="card">
          <h2 class="card__title">${t('bo.thread')}</h2>
          <div class="thread">
            ${notes.length ? notes.map((m) => {
    const fromClient = m.attore_id === p.utente_id;
    const internal = isInternalEvt(m);
    return html`<div class="bubble ${fromClient ? '' : 'bubble--mine'} ${internal ? 'bubble--internal' : ''}">
                <p class="bubble__meta">${fromClient ? t('bo.client') : (userName(users, m.attore_id) || t('bo.operator'))} · ${fmtDateTime(m.creato_il)} ${internal ? badge(t('bo.internal'), 'neutral') : ''}</p>
                <p class="bubble__text">${m.messaggio}</p>
              </div>`;
  }) : html`<p class="muted">${t('det.noMessages')}</p>`}
          </div>
          <form class="composer composer--bo" id="noteForm">
            <label class="sr-only" for="noteText">${t('bo.writeNote')}</label>
            <textarea id="noteText" rows="2" placeholder="${t('bo.writeNote')}" required></textarea>
            <div class="composer__row">
              <label class="check"><input type="checkbox" id="noteInternal" checked> <span>${t('bo.internalOnly')}</span></label>
              <button type="submit" class="btn btn--primary">${icon('send', 16)} ${t('det.send')}</button>
            </div>
          </form>
        </section>
      </div>

      <aside class="detail-aside">
        <section class="card card--compact">
          <h2 class="card__title">${t('bo.assignment')}</h2>
          <form id="assignForm" class="stack">
            <label for="assignSel" class="sr-only">${t('bo.operator')}</label>
            <select id="assignSel">
              <option value="">${t('bo.unassigned')}</option>
              ${operators.map((o) => html`<option value="${o.id}" ${o.id === p.operatore_id ? 'selected' : ''}>${fullName(o)} (${t(`role.${o.ruolo}`)})</option>`)}
            </select>
            <div class="btn-row">
              <button type="submit" class="btn btn--secondary btn--sm">${t('bo.assign')}</button>
              ${p.operatore_id !== me ? html`<button type="button" class="btn btn--primary btn--sm" data-assign-me>${t('bo.assignMe')}</button>` : ''}
            </div>
          </form>
        </section>

        ${transitions.length ? html`<section class="card card--compact">
          <h2 class="card__title">${t('bo.changeState')}</h2>
          <form id="stateForm" class="stack">
            <fieldset class="radio-list">
              <legend class="sr-only">${t('bo.changeState')}</legend>
              ${transitions.map((s, i) => html`<label class="radio"><input type="radio" name="stato" value="${s}" ${i === 0 ? 'checked' : ''}> <span>${t(`act.${s}`) || statoLabel(s)}</span></label>`)}
            </fieldset>
            <label for="stateNote">${t('bo.stateNote')}</label>
            <textarea id="stateNote" rows="2"></textarea>
            <button type="submit" class="btn btn--primary btn--sm">${t('bo.apply')}</button>
          </form>
        </section>` : ''}

        ${p.stato === 'IN_LAVORAZIONE' || p.stato === 'SOSPESA' ? html`<section class="card card--compact">
          <h2 class="card__title">${t('bo.requestDoc')}</h2>
          <form id="docReqForm" class="stack">
            <label for="docReqType">${t('bo.document')}</label>
            <select id="docReqType">${TIPI_DOCUMENTO.map((x) => html`<option value="${x}">${docTipoLabel(x)}</option>`)}</select>
            <label for="docReqNote">${t('bo.noteForClient')}</label>
            <textarea id="docReqNote" rows="2" placeholder="${t('bo.docReqPh')}"></textarea>
            <button type="submit" class="btn btn--attention btn--sm">${t('bo.sendRequest')}</button>
          </form>
        </section>` : ''}

        ${p.stato === 'APPROVATA' ? html`<section class="card card--compact">
          <h2 class="card__title">${t('bo.payment')}</h2>
          <form id="payForm" class="stack">
            <label for="payAmount">${t('bo.amount')}</label>
            <input id="payAmount" type="number" min="1" step="0.01" required value="${p.importo_dovuto || ''}">
            <label for="payProvider">${t('bo.provider')}</label>
            <select id="payProvider"><option value="stripe">Stripe</option></select>
            <button type="submit" class="btn btn--primary btn--sm">${t('bo.createLink')}</button>
          </form>
          <p class="muted small" id="payResult"></p>
        </section>` : ''}

        <section class="card card--compact">
          <h2 class="card__title">${t('det.summary')}</h2>
          ${definitionList([
    [t('bo.created'), fmtDateTime(p.creato_il)],
    [t('det.sentOn'), fmtDateTime(p.inviato_il)],
    [t('bo.col.updated'), fmtDateTime(p.aggiornato_il)],
    [t('det.entry'), values['3.05'] ? fmtDate(values['3.05']) : '-'],
    [t('det.fee'), fmtMoney(p.importo_dovuto, p.valuta)],
  ])}
        </section>

        <section class="card card--compact">
          <h2 class="card__title">${t('det.history')}</h2>
          <ol class="timeline">${[...eventi].reverse().map((e) => html`<li class="timeline__item ${isInternalEvt(e) ? 'is-internal' : ''}">
            <p class="timeline__title">${evtLabel(e, users)}</p>
            <p class="timeline__meta">${fmtDateTime(e.creato_il)}${e.attore_id ? ` · ${e.attore_id === p.utente_id ? t('bo.client') : (userName(users, e.attore_id) || '')}` : ''}</p>
            ${e.tipo_evento === 'STATO_CAMBIATO' && e.messaggio ? html`<p class="timeline__note">${e.messaggio}</p>` : ''}
          </li>`)}</ol>
        </section>
      </aside>
    </div>
  `);

  const reload = () => viewBODettaglio(root, params);
  const pid = encodeURIComponent(p.id);
  const act = async (button, fn, okText) => {
    await busy(button, async () => {
      try {
        const r = await fn();
        if (okText) toast('ok', okText);
        if (r !== false) reload();
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  };

  on(root, 'click', '[data-doc-preview]', async (_ev, btn) => {
    try { await openDocumento(p.id, btn.dataset.docPreview, 'preview'); } catch (err) { toast('err', errorMessage(err)); }
  });
  on(root, 'click', '[data-doc-download]', async (_ev, btn) => {
    try { await openDocumento(p.id, btn.dataset.docDownload, 'download'); } catch (err) { toast('err', errorMessage(err)); }
  });
  $('#noteForm', root)?.addEventListener('submit', (ev) => {
    ev.preventDefault();
    const text = $('#noteText', root).value.trim();
    if (!text) return;
    const interna = $('#noteInternal', root).checked;
    act(ev.submitter, () => api(`/api/bo/pratiche/${pid}/note`, { method: 'POST', json: { messaggio: text, interna } }), interna ? t('bo.noteSaved') : t('det.msgSent'));
  });
  $('#assignForm', root)?.addEventListener('submit', (ev) => {
    ev.preventDefault();
    const id = $('#assignSel', root).value;
    if (!id) return;
    act(ev.submitter, () => api(`/api/bo/pratiche/${pid}/assegna`, { method: 'POST', json: { operatore_id: id } }), t('bo.assigned'));
  });
  on(root, 'click', '[data-assign-me]', (_ev, btn) => {
    act(btn, () => api(`/api/bo/pratiche/${pid}/assegna`, { method: 'POST', json: { operatore_id: me } }), t('bo.assigned'));
  });
  $('#stateForm', root)?.addEventListener('submit', (ev) => {
    ev.preventDefault();
    const stato = ev.target.querySelector('input[name="stato"]:checked')?.value;
    if (!stato) return;
    const nota = $('#stateNote', root).value.trim();
    if (stato === 'RIFIUTATA' && !nota) {
      toast('err', t('bo.rejectNeedsNote'));
      $('#stateNote', root).focus();
      return;
    }
    act(ev.submitter, () => api(`/api/bo/pratiche/${pid}/stato`, { method: 'PATCH', json: { stato, nota } }), t('bo.stateChanged', { state: statoLabel(stato) }));
  });
  $('#docReqForm', root)?.addEventListener('submit', (ev) => {
    ev.preventDefault();
    const documento = docTipoLabel($('#docReqType', root).value);
    const nota = $('#docReqNote', root).value.trim();
    act(ev.submitter, () => api(`/api/bo/pratiche/${pid}/richiedi-doc`, { method: 'POST', json: { documento, nota } }), t('bo.requestSent'));
  });
  $('#payForm', root)?.addEventListener('submit', (ev) => {
    ev.preventDefault();
    const importo = Number($('#payAmount', root).value);
    if (!importo || importo <= 0) return;
    act(ev.submitter, async () => {
      const pay = await api(`/api/bo/pratiche/${pid}/link-pagamento`, { method: 'POST', json: { importo, provider: $('#payProvider', root).value } });
      toast('ok', t('bo.linkCreated'));
      if (pay?.link_pagamento) {
        try { await navigator.clipboard.writeText(pay.link_pagamento); toast('info', t('bo.linkCopied')); } catch { /* appunti non disponibili */ }
      }
    });
  });
}

// ---------------------------------------------------------------------------
// Utenti
// ---------------------------------------------------------------------------

export async function viewClienti(root) {
  root = fresh(root);
  render(root, loading());
  let users;
  try {
    users = await loadUsers(true);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/bo/clienti'));
    return;
  }
  const clienti = users.filter((u) => u.ruolo === 'RICHIEDENTE');
  render(root, html`
    ${pageHeader(t('bo.clients'), t('bo.clientsSub', { n: clienti.length }))}
    <div class="toolbar"><label class="sr-only" for="cliQ">${t('common.search')}</label><input id="cliQ" type="search" class="toolbar__search" placeholder="${t('bo.searchUsersPh')}"></div>
    ${clienti.length ? html`<div class="table-wrap card card--flush"><table class="table" id="cliTable">
      <thead><tr><th scope="col">${t('bo.name')}</th><th scope="col">Email</th><th scope="col">${t('bo.emailVerified')}</th><th scope="col">${t('bo.status')}</th><th scope="col">${t('bo.registered')}</th></tr></thead>
      <tbody>${clienti.map((u) => html`<tr data-search="${`${fullName(u)} ${u.email}`.toLowerCase()}">
        <td>${fullName(u)}</td><td>${u.email}</td>
        <td>${u.email_verificata ? badge(t('common.yes'), 'success') : badge(t('common.no'), 'warning')}</td>
        <td>${u.attivo ? badge(t('bo.active'), 'success') : badge(t('bo.inactive'), 'neutral')}</td>
        <td>${fmtDate(u.creato_il)}</td>
      </tr>`)}</tbody></table></div>` : emptyState(t('bo.noClients'), '')}
  `);
  $('#cliQ', root)?.addEventListener('input', (ev) => {
    const q = ev.target.value.trim().toLowerCase();
    root.querySelectorAll('#cliTable tbody tr').forEach((tr) => { tr.hidden = Boolean(q) && !tr.dataset.search.includes(q); });
  });
}

export async function viewOperatori(root) {
  root = fresh(root);
  render(root, loading());
  let users;
  try {
    users = await loadUsers(true);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/bo/operatori'));
    return;
  }
  const ops = operatorsFrom(users);
  const canInvite = state.user?.ruolo === 'ADMIN' || state.user?.ruolo === 'SUPERVISORE';
  render(root, html`
    ${pageHeader(t('bo.operators'), t('bo.operatorsSub', { n: ops.length }))}
    <div class="two-col">
      <div class="table-wrap card card--flush"><table class="table">
        <thead><tr><th scope="col">${t('bo.name')}</th><th scope="col">Email</th><th scope="col">${t('bo.role')}</th><th scope="col">2FA</th><th scope="col">${t('bo.status')}</th></tr></thead>
        <tbody>${ops.map((u) => html`<tr>
          <td>${fullName(u)}</td><td>${u.email}</td><td>${t(`role.${u.ruolo}`)}</td>
          <td>${u.totp_enabled ? badge(t('common.on'), 'success') : badge(t('common.off'), 'warning')}</td>
          <td>${!u.attivo ? badge(t('bo.inactive'), 'neutral') : (u.email_verificata ? badge(t('bo.active'), 'success') : badge(t('bo.invitePending'), 'info'))}</td>
        </tr>`)}</tbody>
      </table></div>
      ${canInvite ? html`<section class="card card--compact">
        <h2 class="card__title">${t('bo.invite')}</h2>
        <form id="inviteForm" class="stack">
          <label for="invNome">${t('auth.firstName')}</label><input id="invNome" name="nome" required autocomplete="off">
          <label for="invCognome">${t('auth.lastName')}</label><input id="invCognome" name="cognome" required autocomplete="off">
          <label for="invEmail">Email</label><input id="invEmail" name="email" type="email" required autocomplete="off">
          <button type="submit" class="btn btn--primary btn--sm">${t('bo.sendInvite')}</button>
        </form>
        <div id="inviteResult"></div>
      </section>` : ''}
    </div>
  `);
  $('#inviteForm', root)?.addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const body = { nome: $('#invNome', root).value.trim(), cognome: $('#invCognome', root).value.trim(), email: $('#invEmail', root).value.trim() };
    await busy(ev.submitter, async () => {
      try {
        const res = await api('/api/bo/operatori/inviti', { method: 'POST', json: body });
        toast('ok', t('bo.inviteSent', { email: body.email }));
        render($('#inviteResult', root), html`<div class="callout callout--success callout--compact"><div class="callout__body"><p>${t('bo.inviteLink')}</p><input type="text" readonly class="mono" value="${res?.invite_url || ''}" aria-label="${t('bo.inviteLink')}"></div></div>`);
        ev.target.reset();
        usersCache.at = 0;
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  });
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

const auditState = { page: 1 };

export async function viewAudit(root) {
  root = fresh(root);
  render(root, loading());
  let res;
  let users = [];
  try {
    [res, users] = await Promise.all([api(`/api/bo/audit-events?page=${auditState.page}&page_size=50`), loadUsers().catch(() => [])]);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/bo/audit'));
    return;
  }
  const items = res?.items || [];
  const pages = Math.max(1, Math.ceil((res?.total || 0) / 50));
  render(root, html`
    ${pageHeader(t('bo.audit'), t('bo.auditSub'), html`<button type="button" class="btn btn--secondary" data-export-audit>${icon('download', 16)} ${t('bo.exportCsv')}</button>`)}
    ${items.length ? html`<div class="table-wrap card card--flush"><table class="table table--dense">
      <thead><tr><th scope="col">${t('bo.when')}</th><th scope="col">${t('bo.action')}</th><th scope="col">${t('bo.resource')}</th><th scope="col">${t('bo.actor')}</th><th scope="col">IP</th></tr></thead>
      <tbody>${items.map((e) => html`<tr>
        <td>${fmtDateTime(e.creato_il)}</td>
        <td class="mono">${e.action}</td>
        <td>${e.resource}${e.resource_id ? html` <span class="muted mono small">${e.resource_id.slice(0, 8)}</span>` : ''}</td>
        <td>${userName(users, e.actor_id) || e.actor_role || '-'}</td>
        <td class="mono small">${e.ip || '-'}</td>
      </tr>`)}</tbody></table></div>
    <nav class="pager" aria-label="${t('common.pagination')}">
      <button type="button" class="btn btn--secondary btn--sm" data-apage="${auditState.page - 1}" ${auditState.page <= 1 ? 'disabled' : ''}>${icon('back', 14)} ${t('common.prev')}</button>
      <span class="muted">${t('common.pageOf', { n: auditState.page, total: pages })}</span>
      <button type="button" class="btn btn--secondary btn--sm" data-apage="${auditState.page + 1}" ${auditState.page >= pages ? 'disabled' : ''}>${t('common.next')} ${icon('next', 14)}</button>
    </nav>` : emptyState(t('bo.noAudit'), '')}
  `);
  on(root, 'click', '[data-apage]', (_ev, btn) => {
    auditState.page = Number(btn.dataset.apage) || 1;
    viewAudit(root);
  });
  on(root, 'click', '[data-export-audit]', async (_ev, btn) => {
    await busy(btn, async () => {
      try {
        downloadBlob(await apiBlob('/api/bo/audit-events/report.csv'), 'audit.csv');
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  });
}
