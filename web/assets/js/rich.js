// Area del richiedente: elenco pratiche, percorso guidato, dettaglio.
import { api, errorMessage } from './http.js';
import { $, $$, html, icon, on, render, debounce, fresh } from './dom.js';
import { t } from './i18n.js';
import { toast, busy } from './toast.js';
import { state } from './session.js';
import {
  statoGroup, progressPct, tappaIndex, TAPPE, prossimoPasso, countryName, flag, tipoVistoLabel,
  fmtDate, fmtShort, fmtDateTime, parseDate, documentiConsigliati, docTipoLabel, TIPI_DOCUMENTO,
  docStatoLabel, docStatoTone, fmtBytes, fmtMoney, isoToDisplay, cap,
} from './domain.js';
import {
  FIELDS, FIELD_BY_CODE, STEPS, isVisible, validate, completion, valuesFromPratica, payloadFromValues,
  durationDays, fieldLabel, optionLabel,
} from './schema.js';
import {
  badge, statoBadge, progress, emptyState, loading, errorBox, formField, readControl, pageHeader, definitionList,
} from './components.js';
import { uploadDocumento, openDocumento, checkFile } from './upload.js';

const listState = { filter: 'in_corso' };

function isMine(evt) {
  return evt.attore_id && evt.attore_id === state.user?.id;
}

function sortByUpdated(list) {
  return [...list].sort((a, b) => (parseDate(b.aggiornato_il) || 0) - (parseDate(a.aggiornato_il) || 0));
}

function lastOperatorMessage(p) {
  const msgs = (p.eventi || []).filter((e) => e.tipo_evento === 'NOTA_AGGIUNTA' && !isMine(e));
  return msgs.length ? msgs[msgs.length - 1] : null;
}

function praticaTitle(p) {
  return html`${flag(p.paese_dest)} ${countryName(p.paese_dest)}`;
}

function praticaCode(p) {
  return p.codice || t('rich.draftCode');
}

// ---------------------------------------------------------------------------
// Elenco pratiche
// ---------------------------------------------------------------------------

function actionBanner(pratiche) {
  const integr = pratiche.find((p) => p.stato === 'INTEGRAZIONE_RICHIESTA');
  if (integr) {
    const msg = lastOperatorMessage(integr);
    return html`<section class="callout callout--attention" aria-label="${t('rich.actionNeeded')}">
      <span class="callout__icon callout__icon--solid">${icon('alert', 22)}</span>
      <div class="callout__body">
        <p class="callout__title">${t('rich.banner.integr', { code: praticaCode(integr) })}</p>
        <p>${msg ? msg.messaggio : cap(t('next.INTEGRAZIONE_RICHIESTA'))}</p>
      </div>
      <a class="btn btn--attention-ghost" href="#/pratiche/${integr.id}">${t('rich.banner.integrCta')}</a>
    </section>`;
  }
  const pay = pratiche.find((p) => p.stato === 'ATTENDE_PAGAMENTO');
  if (pay) {
    return html`<section class="callout callout--warning" aria-label="${t('rich.actionNeeded')}">
      <span class="callout__icon">${icon('clock', 22)}</span>
      <div class="callout__body">
        <p class="callout__title">${t('rich.banner.pay', { code: praticaCode(pay) })}</p>
        <p>${cap(t('next.ATTENDE_PAGAMENTO'))}</p>
      </div>
      <a class="btn btn--secondary" href="#/pratiche/${pay.id}">${t('rich.open')}</a>
    </section>`;
  }
  const draft = pratiche.find((p) => p.stato === 'BOZZA');
  if (draft) {
    const c = completion(valuesFromPratica(draft));
    return html`<section class="callout callout--info" aria-label="${t('rich.actionNeeded')}">
      <span class="callout__icon">${icon('file', 22)}</span>
      <div class="callout__body">
        <p class="callout__title">${t('rich.banner.draft', { country: countryName(draft.paese_dest) })}</p>
        <p>${t('rich.banner.draftText', { pct: c.pct })}</p>
      </div>
      <a class="btn btn--secondary" href="#/pratiche/${draft.id}/modifica">${t('rich.continue')}</a>
    </section>`;
  }
  return '';
}

function praticaCard(p) {
  const group = statoGroup(p.stato);
  const pct = progressPct(p.stato);
  const idx = tappaIndex(p.stato);
  let stepText;
  if (p.stato === 'BOZZA') {
    const c = completion(valuesFromPratica(p));
    stepText = t('rich.card.draftProgress', { pct: c.pct });
  } else if (group === 'concluse') {
    stepText = t('rich.card.closed', { date: fmtDate(p.completato_il || p.aggiornato_il) });
  } else {
    stepText = t('rich.card.step', { n: idx + 1, total: TAPPE.length, name: t(`tappa.${TAPPE[Math.max(idx, 0)]}`) });
  }
  const tone = p.stato === 'INTEGRAZIONE_RICHIESTA' ? 'attention' : (group === 'concluse' ? 'muted' : 'brand');
  const href = p.stato === 'BOZZA' ? `#/pratiche/${p.id}/modifica` : `#/pratiche/${p.id}`;
  const next = prossimoPasso(p);
  return html`<article class="card pratica-card">
    <div class="pratica-card__head">
      <div>
        <p class="pratica-card__meta">${praticaCode(p)} · ${tipoVistoLabel(p.tipo_visto)}</p>
        <h2 class="pratica-card__title">${praticaTitle(p)}</h2>
      </div>
      ${statoBadge(p.stato)}
    </div>
    <div>
      <div class="pratica-card__progress-label"><span>${stepText}</span>${p.stato === 'BOZZA' ? '' : html`<span>${pct}%</span>`}</div>
      ${progress(p.stato === 'BOZZA' ? completion(valuesFromPratica(p)).pct : pct, tone, stepText)}
    </div>
    ${next ? html`<p class="pratica-card__next"><strong>${t('rich.nextStep')}</strong> ${next}</p>` : ''}
    <a class="btn btn--secondary btn--block pratica-card__cta" href="${href}">${p.stato === 'BOZZA' ? t('rich.continue') : t('rich.open')}</a>
  </article>`;
}

export async function viewDashboard(root) {
  root = fresh(root);
  render(root, loading());
  let pratiche;
  try {
    pratiche = sortByUpdated(await api('/api/pratiche/') || []);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/pratiche'));
    return;
  }
  const counts = { in_corso: 0, bozze: 0, concluse: 0 };
  pratiche.forEach((p) => { counts[statoGroup(p.stato)] += 1; });
  if (!counts[listState.filter] && pratiche.length) {
    listState.filter = ['in_corso', 'bozze', 'concluse'].find((k) => counts[k]) || 'in_corso';
  }
  const visible = pratiche.filter((p) => statoGroup(p.stato) === listState.filter);
  const name = state.user?.nome || '';
  const hour = new Date().getHours();
  const greet = hour < 13 ? t('rich.greet.morning') : (hour < 18 ? t('rich.greet.afternoon') : t('rich.greet.evening'));

  render(root, html`
    ${pageHeader(t('rich.title'), name ? `${greet}, ${name}` : greet, html`<a class="btn btn--primary btn--lg" href="#/pratiche/nuova">${icon('plus')} ${t('rich.new')}</a>`)}
    ${actionBanner(pratiche)}
    ${pratiche.length ? html`
      <div class="pills" role="tablist" aria-label="${t('rich.filterAria')}">
        ${['in_corso', 'bozze', 'concluse'].map((k) => html`<button type="button" class="pill ${listState.filter === k ? 'is-active' : ''}" role="tab" aria-selected="${listState.filter === k ? 'true' : 'false'}" data-filter="${k}">${t(`rich.filter.${k}`)} · ${counts[k]}</button>`)}
      </div>
      ${visible.length
    ? html`<div class="card-grid">${visible.map(praticaCard)}</div>`
    : emptyState(t('rich.emptyFilter'), t('rich.emptyFilterText'))}
    ` : emptyState(t('rich.empty'), t('rich.emptyText'), html`<a class="btn btn--primary" href="#/pratiche/nuova">${icon('plus')} ${t('rich.new')}</a>`)}
  `);

  on(root, 'click', '[data-filter]', (_ev, btn) => {
    listState.filter = btn.dataset.filter;
    viewDashboard(root);
  });
}

// ---------------------------------------------------------------------------
// Percorso guidato
// ---------------------------------------------------------------------------

export async function viewWizard(root, params) {
  root = fresh(root);
  const wiz = {
    id: params.id || '',
    pratica: null,
    values: {},
    step: 1,
    errors: {},
    savedAt: null,
    saving: false,
    dirty: false,
  };

  if (wiz.id) {
    render(root, loading());
    try {
      wiz.pratica = await api(`/api/pratiche/${encodeURIComponent(wiz.id)}`);
    } catch (err) {
      render(root, errorBox(errorMessage(err), '#/pratiche'));
      return;
    }
    if (wiz.pratica.stato !== 'BOZZA') {
      window.location.hash = `#/pratiche/${wiz.id}`;
      return;
    }
    wiz.values = valuesFromPratica(wiz.pratica);
    wiz.savedAt = parseDate(wiz.pratica.aggiornato_il);
    // Riparte dal primo passo con campi obbligatori mancanti.
    const firstMissing = STEPS.find((s) => s.n <= 5 && Object.keys(validate(wiz.values, s.n)).length);
    wiz.step = params.step ? Number(params.step) : (firstMissing ? firstMissing.n : 6);
  } else {
    wiz.values = {
      '1.01': state.user?.cognome || '',
      '1.02': state.user?.nome || '',
      '1.13': state.user?.email || '',
      '3.08': 'IT',
      '3.09': 'IT',
    };
  }

  const save = async () => {
    if (!wiz.id || !wiz.dirty) return;
    wiz.saving = true;
    updateSaveStatus();
    try {
      wiz.pratica = await api(`/api/pratiche/${encodeURIComponent(wiz.id)}`, { method: 'PATCH', json: payloadFromValues(wiz.values) });
      wiz.dirty = false;
      wiz.savedAt = new Date();
    } catch (err) {
      toast('err', t('wiz.saveError', { error: errorMessage(err) }));
    } finally {
      wiz.saving = false;
      updateSaveStatus();
    }
  };
  const autosave = debounce(save, 1500);

  function updateSaveStatus() {
    const el = $('#wizSaveStatus', root);
    if (!el) return;
    if (!wiz.id) el.textContent = t('wiz.notSaved');
    else if (wiz.saving) el.textContent = t('wiz.saving');
    else if (wiz.dirty) el.textContent = t('wiz.unsaved');
    else if (wiz.savedAt) el.textContent = t('wiz.savedAt', { time: wiz.savedAt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) });
    else el.textContent = '';
  }

  function sideInfo() {
    const c = completion(wiz.values);
    const docs = documentiConsigliati(wiz.values['3.02']);
    return html`<aside class="wizard__aside">
      <section class="card card--compact">
        <h2 class="card__title">${t('wiz.completion')}</h2>
        <p class="muted">${t('wiz.completionText', { done: c.done, total: c.total })}</p>
        ${progress(c.pct, 'success', t('wiz.completion'))}
      </section>
      <section class="card card--compact">
        <h2 class="card__title">${t('wiz.docsTitle')}</h2>
        <ul class="checklist">${docs.map((d) => html`<li>${icon('file', 16)} ${docTipoLabel(d)}</li>`)}</ul>
        <p class="muted small">${t('wiz.docsText')}</p>
      </section>
      <section class="card card--compact">
        <h2 class="card__title">${t('wiz.helpTitle')}</h2>
        <p class="muted small">${t('wiz.helpText')}</p>
      </section>
    </aside>`;
  }

  function stepNav() {
    return html`<nav class="stepper" aria-label="${t('wiz.stepsAria')}">
      ${STEPS.map((s) => {
    const errs = s.n <= 5 ? Object.keys(validate(wiz.values, s.n)).length : 0;
    const done = s.n === 6 ? (wiz.pratica?.documenti || []).length > 0 && s.n !== wiz.step : (s.n < wiz.step || (wiz.id && s.n <= 5)) && !errs;
    const current = s.n === wiz.step;
    const reachable = Boolean(wiz.id) || s.n === 1;
    return html`<button type="button" class="stepper__item ${current ? 'is-current' : ''} ${done ? 'is-done' : ''}" data-goto="${s.n}" ${current ? 'aria-current="step"' : ''} ${reachable ? '' : 'disabled'}>
          <span class="stepper__dot">${done ? icon('check', 14) : s.n}</span>
          <span class="stepper__label">${t(`step.${s.key}`)}</span>
        </button>`;
  })}
    </nav>`;
  }

  function fieldsFor(step) {
    const fields = FIELDS.filter((f) => f.step === step);
    const main = fields.filter((f) => !f.optional);
    const optional = fields.filter((f) => f.optional);
    const hasOptErr = optional.some((f) => wiz.errors[f.code]);
    return html`
      <div class="form-grid">${main.map((f) => formField(f, wiz.values, wiz.errors[f.code]))}</div>
      ${optional.length ? html`<details class="more" ${hasOptErr ? 'open' : ''}>
        <summary>${t(`wiz.optional.${step}`) || t('wiz.optional')}</summary>
        <div class="form-grid">${optional.map((f) => formField(f, wiz.values, wiz.errors[f.code]))}</div>
      </details>` : ''}`;
  }

  function documentsStep() {
    const docs = wiz.pratica?.documenti || [];
    const suggested = documentiConsigliati(wiz.values['3.02']);
    const uploadedTypes = new Set(docs.map((d) => d.tipo));
    return html`
      <ul class="doc-list">
        ${suggested.map((tipo) => {
    const have = docs.filter((d) => d.tipo === tipo);
    return html`<li class="doc-row">
            <span class="doc-row__icon ${have.length ? 'is-ok' : ''}">${icon(have.length ? 'check' : 'file', 18)}</span>
            <div class="doc-row__body">
              <p class="doc-row__title">${docTipoLabel(tipo)}</p>
              <p class="doc-row__meta">${have.length ? have.map((d) => d.nome_file).join(', ') : t('wiz.docMissing')}</p>
            </div>
            <label class="btn btn--secondary btn--sm file-btn">${icon('upload', 16)} ${have.length ? t('wiz.docReplace') : t('wiz.docUpload')}
              <input type="file" class="sr-only" accept=".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/*" data-upload="${tipo}">
            </label>
          </li>`;
  })}
        ${docs.filter((d) => !suggested.includes(d.tipo)).map((d) => html`<li class="doc-row">
          <span class="doc-row__icon is-ok">${icon('check', 18)}</span>
          <div class="doc-row__body"><p class="doc-row__title">${docTipoLabel(d.tipo)}</p><p class="doc-row__meta">${d.nome_file} · ${fmtBytes(d.dimensione)}</p></div>
        </li>`)}
      </ul>
      <div class="upload-other">
        <label for="wizOtherType">${t('wiz.docOther')}</label>
        <div class="inline-controls">
          <select id="wizOtherType">${TIPI_DOCUMENTO.filter((x) => !uploadedTypes.has(x) || x === 'ALTRO').map((x) => html`<option value="${x}">${docTipoLabel(x)}</option>`)}</select>
          <label class="btn btn--secondary file-btn">${icon('upload', 16)} ${t('wiz.docChoose')}
            <input type="file" class="sr-only" accept=".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/*" data-upload-other="1">
          </label>
        </div>
        <p class="field__hint">${t('upload.rules')}</p>
      </div>`;
  }

  function summaryStep() {
    const allErrors = validate(wiz.values);
    const missing = Object.keys(allErrors);
    const docs = wiz.pratica?.documenti || [];
    const v = wiz.values;
    return html`
      ${missing.length ? html`<div class="callout callout--attention" role="alert">
        <span class="callout__icon">${icon('alert', 20)}</span>
        <div class="callout__body">
          <p class="callout__title">${t('wiz.missingTitle', { n: missing.length })}</p>
          <ul class="missing-list">${missing.slice(0, 8).map((code) => html`<li><button type="button" class="link-btn" data-goto="${FIELD_BY_CODE[code].step}">${fieldLabel(code)}</button>: ${allErrors[code]}</li>`)}</ul>
        </div>
      </div>` : html`<div class="callout callout--success"><span class="callout__icon">${icon('check', 20)}</span><div class="callout__body"><p class="callout__title">${t('wiz.readyTitle')}</p><p>${t('wiz.readyText')}</p></div></div>`}
      <div class="summary-grid">
        ${[1, 2, 3, 4, 5].map((n) => html`<section class="summary-block">
          <div class="summary-block__head"><h3>${t(`step.${STEPS[n - 1].key}`)}</h3><button type="button" class="link-btn" data-goto="${n}">${t('common.edit')}</button></div>
          ${definitionList(FIELDS.filter((f) => f.step === n && isVisible(f, v) && String(v[f.code] || '').trim()).slice(0, n === 5 ? 4 : 8).map((f) => [fieldLabel(f.code), displayValue(f, v[f.code])]))}
        </section>`)}
        <section class="summary-block">
          <div class="summary-block__head"><h3>${t('step.documenti')}</h3><button type="button" class="link-btn" data-goto="6">${t('common.edit')}</button></div>
          ${docs.length ? html`<ul class="plain-list">${docs.map((d) => html`<li>${docTipoLabel(d.tipo)}: ${d.nome_file}</li>`)}</ul>` : html`<p class="muted">${t('wiz.noDocs')}</p>`}
        </section>
      </div>
      <label class="check">
        <input type="checkbox" id="wizConfirm">
        <span>${t('wiz.confirm')}</span>
      </label>`;
  }

  function stepBody() {
    if (wiz.step <= 5) return fieldsFor(wiz.step);
    if (wiz.step === 6) return documentsStep();
    return summaryStep();
  }

  function draw() {
    const stepDef = STEPS[wiz.step - 1];
    const title = wiz.values['3.01']
      ? t('wiz.headerWith', { country: countryName(wiz.values['3.01']), tipo: tipoVistoLabel(wiz.values['3.02']) })
      : t('wiz.header');
    const isLast = wiz.step === STEPS.length;
    render(root, html`
      <div class="wizard">
        <div class="wizard__bar">
          <a class="back-link" href="#/pratiche">${icon('back', 16)} ${t('rich.title')}</a>
          <p class="wizard__bar-title">${title}</p>
          <p class="wizard__save" id="wizSaveStatus" aria-live="polite"></p>
        </div>
        <div class="wizard__layout">
          ${stepNav()}
          <main class="wizard__main card" aria-labelledby="wizTitle">
            <div class="wizard__head">
              <p class="eyebrow">${t('wiz.stepOf', { n: wiz.step, total: STEPS.length })}</p>
              <h1 id="wizTitle">${t(`wiz.title.${stepDef.key}`)}</h1>
              <p class="muted">${t(`wiz.intro.${stepDef.key}`)}</p>
            </div>
            <form id="wizForm" novalidate>
              ${stepBody()}
              <div class="wizard__actions">
                ${wiz.step > 1 ? html`<button type="button" class="btn btn--secondary" data-prev>${icon('back', 16)} ${t('common.back')}</button>` : html`<span></span>`}
                ${isLast
    ? html`<button type="submit" class="btn btn--primary btn--lg" id="wizSubmit">${icon('send', 16)} ${t('wiz.submit')}</button>`
    : html`<button type="submit" class="btn btn--primary btn--lg">${t('wiz.continueTo', { next: t(`step.${STEPS[wiz.step].key}`) })} ${icon('next', 16)}</button>`}
              </div>
            </form>
          </main>
          ${sideInfo()}
        </div>
      </div>`);
    updateSaveStatus();
    const firstErr = $('[aria-invalid="true"]', root);
    if (firstErr && wiz.focusError) {
      firstErr.focus();
      wiz.focusError = false;
    }
  }

  function displayValue(field, value) {
    if (field.type === 'country') return `${flag(value)} ${countryName(value)}`;
    if (field.type === 'select') return optionLabel(field.code, value);
    if (field.type === 'date') return isoToDisplay(value);
    return value;
  }

  function readForm() {
    $$('[data-code]', root).forEach((el) => {
      wiz.values[el.dataset.code] = readControl(el);
    });
    const days = durationDays(wiz.values);
    if (days) wiz.values['3.07'] = days;
  }

  function refreshConditional() {
    $$('[data-field]', root).forEach((wrap) => {
      const f = FIELD_BY_CODE[wrap.dataset.field];
      if (f) wrap.classList.toggle('is-hidden', !isVisible(f, wiz.values));
    });
    const dur = $('[data-code="3.07"]', root);
    if (dur) dur.value = wiz.values['3.07'] || '';
  }

  async function ensureCreated() {
    if (wiz.id) return true;
    const payload = payloadFromValues(wiz.values);
    try {
      wiz.pratica = await api('/api/pratiche/', { method: 'POST', json: payload });
      wiz.id = wiz.pratica.id;
      wiz.dirty = false;
      wiz.savedAt = new Date();
      history.replaceState(null, '', `#/pratiche/${wiz.id}/modifica`);
      return true;
    } catch (err) {
      toast('err', errorMessage(err));
      return false;
    }
  }

  async function goTo(step, { checkCurrent }) {
    readForm();
    if (checkCurrent && wiz.step <= 5) {
      wiz.errors = validate(wiz.values, wiz.step);
      if (Object.keys(wiz.errors).length) {
        wiz.focusError = true;
        draw();
        toast('err', t('wiz.fixErrors'));
        return;
      }
    }
    if (!await ensureCreated()) return;
    await autosave.flush();
    if (step >= 6) {
      try {
        wiz.pratica = await api(`/api/pratiche/${encodeURIComponent(wiz.id)}`);
      } catch {
        // si usa l'ultima versione disponibile
      }
    }
    wiz.errors = step <= 5 ? pickVisibleErrors(step) : {};
    wiz.step = step;
    draw();
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  // Tornando su un passo già visitato mostra subito gli errori presenti.
  function pickVisibleErrors(step) {
    const errs = validate(wiz.values, step);
    const out = {};
    Object.entries(errs).forEach(([code, msg]) => {
      if (String(wiz.values[code] || '').trim()) out[code] = msg;
    });
    return out;
  }

  async function handleUpload(input, tipo) {
    const file = input.files?.[0];
    if (!file) return;
    const problem = checkFile(file);
    if (problem) {
      toast('err', problem);
      input.value = '';
      return;
    }
    const label = input.closest('label');
    label?.classList.add('is-busy');
    try {
      const { stored } = await uploadDocumento(wiz.id, tipo, file);
      toast(stored ? 'ok' : 'info', stored ? t('upload.ok', { name: file.name }) : t('upload.metaOnly'));
      wiz.pratica = await api(`/api/pratiche/${encodeURIComponent(wiz.id)}`);
      draw();
    } catch (err) {
      toast('err', errorMessage(err));
    } finally {
      label?.classList.remove('is-busy');
      input.value = '';
    }
  }

  async function submitPratica(button) {
    readForm();
    const errs = validate(wiz.values);
    if (Object.keys(errs).length) {
      toast('err', t('wiz.fixErrors'));
      draw();
      return;
    }
    if (!$('#wizConfirm', root)?.checked) {
      toast('err', t('wiz.confirmNeeded'));
      $('#wizConfirm', root)?.focus();
      return;
    }
    await busy(button, async () => {
      try {
        await autosave.flush();
        await api(`/api/pratiche/${encodeURIComponent(wiz.id)}/submit`, { method: 'POST' });
        toast('ok', t('wiz.submitted'));
        window.location.hash = `#/pratiche/${wiz.id}`;
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  }

  draw();

  root.addEventListener('input', (ev) => {
    const el = ev.target.closest('[data-code]');
    if (!el) return;
    if (FIELD_BY_CODE[el.dataset.code]?.transform === 'upper' && el.value !== el.value.toUpperCase()) {
      const pos = el.selectionStart;
      el.value = el.value.toUpperCase();
      if (typeof pos === 'number') el.setSelectionRange(pos, pos);
    }
    wiz.values[el.dataset.code] = readControl(el);
    const days = durationDays(wiz.values);
    if (days) wiz.values['3.07'] = days;
    wiz.dirty = true;
    if (wiz.errors[el.dataset.code]) {
      delete wiz.errors[el.dataset.code];
      el.setAttribute('aria-invalid', 'false');
      el.closest('.field')?.querySelector('.field__error')?.remove();
    }
    updateSaveStatus();
    if (wiz.id) autosave();
  });
  root.addEventListener('change', (ev) => {
    const el = ev.target;
    if (el.matches('[data-code]')) {
      if (el.dataset.type === 'date') {
        const iso = readControl(el);
        if (/^\d{4}-\d{2}-\d{2}$/.test(iso)) el.value = isoToDisplay(iso);
      }
      refreshConditional();
    }
    if (el.matches('[data-upload]')) handleUpload(el, el.dataset.upload);
    if (el.matches('[data-upload-other]')) handleUpload(el, $('#wizOtherType', root)?.value || 'ALTRO');
  });
  on(root, 'click', '[data-goto]', (ev, btn) => {
    ev.preventDefault();
    goTo(Number(btn.dataset.goto), { checkCurrent: false });
  });
  on(root, 'click', '[data-prev]', (ev) => {
    ev.preventDefault();
    goTo(wiz.step - 1, { checkCurrent: false });
  });
  root.addEventListener('submit', (ev) => {
    if (!ev.target.matches('#wizForm')) return;
    ev.preventDefault();
    if (wiz.step === STEPS.length) submitPratica($('#wizSubmit', root));
    else goTo(wiz.step + 1, { checkCurrent: true });
  });

  // Salva le modifiche in sospeso quando si lascia la pagina.
  const flushOnLeave = () => { if (wiz.dirty) autosave.flush(); };
  window.addEventListener('hashchange', flushOnLeave, { once: true });
}

// ---------------------------------------------------------------------------
// Dettaglio pratica
// ---------------------------------------------------------------------------

function timelineLabel(e) {
  switch (e.tipo_evento) {
    case 'PRATICA_CREATA': return t('evt.created');
    case 'STATO_CAMBIATO': return t('evt.state', { state: t(`stato.${e.a_stato}`) || e.a_stato });
    case 'NOTA_AGGIUNTA': return isMine(e) ? t('evt.msgSent') : t('evt.msgReceived');
    case 'ASSEGNAZIONE_OPERATORE': return t('evt.assigned');
    default: return String(e.tipo_evento || '').replaceAll('_', ' ').toLowerCase();
  }
}

function stepperTappe(p) {
  const idx = tappaIndex(p.stato);
  const waiting = p.stato === 'INTEGRAZIONE_RICHIESTA' || p.stato === 'ATTENDE_PAGAMENTO';
  const dates = {};
  (p.eventi || []).forEach((e) => {
    if (e.tipo_evento !== 'STATO_CAMBIATO') return;
    const i = tappaIndex(e.a_stato);
    if (i >= 0 && !dates[i]) dates[i] = e.creato_il;
  });
  if (!dates[0] && p.inviato_il) dates[0] = p.inviato_il;
  const closedBad = p.stato === 'RIFIUTATA' || p.stato === 'ANNULLATA';
  return html`<ol class="track" aria-label="${t('det.trackAria')}">
    ${TAPPE.map((key, i) => {
    const done = !closedBad && (i < idx || (i === idx && (p.stato === 'VISTO_EMESSO' || p.stato === 'COMPLETATA')));
    const current = !closedBad && i === idx && !done;
    let sub = dates[i] ? fmtShort(dates[i]) : '';
    if (current && waiting) sub = t('det.waitingYou');
    return html`<li class="track__step ${done ? 'is-done' : ''} ${current ? 'is-current' : ''} ${current && waiting ? 'is-attention' : ''}" ${current ? 'aria-current="step"' : ''}>
        <span class="track__dot">${done ? icon('check', 14) : ''}</span>
        <span class="track__label">${t(`tappa.${key}`)}</span>
        <span class="track__sub">${sub}</span>
      </li>`;
  })}
  </ol>`;
}

export async function viewDettaglio(root, params) {
  root = fresh(root);
  render(root, loading());
  let p;
  try {
    p = await api(`/api/pratiche/${encodeURIComponent(params.id)}`);
  } catch (err) {
    render(root, errorBox(errorMessage(err), '#/pratiche'));
    return;
  }
  if (p.stato === 'BOZZA') {
    window.location.hash = `#/pratiche/${p.id}/modifica`;
    return;
  }
  const values = valuesFromPratica(p);
  const eventi = p.eventi || [];
  const messaggi = eventi.filter((e) => e.tipo_evento === 'NOTA_AGGIUNTA');
  const docs = p.documenti || [];
  const suggested = documentiConsigliati(p.tipo_visto);
  const canUpload = !['RIFIUTATA', 'ANNULLATA', 'COMPLETATA', 'VISTO_EMESSO'].includes(p.stato);
  const lastOp = lastOperatorMessage(p);
  const missingSuggested = suggested.filter((s) => !docs.some((d) => d.tipo === s));

  render(root, html`
    <a class="back-link" href="#/pratiche">${icon('back', 16)} ${t('rich.title')}</a>
    <div class="detail-head">
      <div>
        <p class="eyebrow">${praticaCode(p)} · ${t('det.visaFor', { tipo: tipoVistoLabel(p.tipo_visto).toLowerCase() })}</p>
        <h1>${praticaTitle(p)}</h1>
      </div>
      ${statoBadge(p.stato)}
    </div>
    ${p.stato === 'RIFIUTATA' || p.stato === 'ANNULLATA' ? html`<div class="callout callout--danger"><span class="callout__icon">${icon('alert', 20)}</span><div class="callout__body"><p class="callout__title">${t(`stato.${p.stato}`)}</p><p>${cap(t(`next.${p.stato}`))}</p></div></div>` : stepperTappe(p)}
    ${p.stato === 'INTEGRAZIONE_RICHIESTA' ? html`<div class="callout callout--attention"><span class="callout__icon callout__icon--solid">${icon('alert', 20)}</span><div class="callout__body"><p class="callout__title">${t('det.integrTitle')}</p><p>${lastOp ? lastOp.messaggio : cap(t('next.INTEGRAZIONE_RICHIESTA'))}</p></div></div>` : ''}
    ${p.stato === 'ATTENDE_PAGAMENTO' ? html`<div class="callout callout--warning"><span class="callout__icon">${icon('clock', 20)}</span><div class="callout__body"><p class="callout__title">${t('det.payTitle')}${p.importo_dovuto ? ` · ${fmtMoney(p.importo_dovuto, p.valuta)}` : ''}</p><p>${t('det.payText')}</p></div></div>` : ''}
    <div class="detail-layout">
      <div class="detail-main">
        <section class="card" aria-labelledby="docsTitle">
          <div class="card__head">
            <h2 class="card__title" id="docsTitle">${t('det.docs')}</h2>
            <span class="muted small">${t('det.docsCount', { n: docs.length })}</span>
          </div>
          <ul class="doc-list">
            ${docs.map((d) => html`<li class="doc-row">
              <span class="doc-row__icon">${icon('file', 18)}</span>
              <div class="doc-row__body">
                <p class="doc-row__title">${docTipoLabel(d.tipo)}</p>
                <p class="doc-row__meta">${d.nome_file} · ${fmtBytes(d.dimensione)} · ${fmtShort(d.caricato_il)}${d.note_validazione ? html` · ${d.note_validazione}` : ''}</p>
              </div>
              ${badge(docStatoLabel(d.stato_validazione), docStatoTone(d.stato_validazione))}
              <button type="button" class="icon-btn" data-open-doc="${d.id}" aria-label="${t('det.openDoc', { name: d.nome_file })}">${icon('download', 18)}</button>
            </li>`)}
            ${missingSuggested.map((tipo) => html`<li class="doc-row doc-row--missing">
              <span class="doc-row__icon">${icon('file', 18)}</span>
              <div class="doc-row__body"><p class="doc-row__title">${docTipoLabel(tipo)}</p><p class="doc-row__meta">${t('det.suggested')}</p></div>
              ${canUpload ? html`<label class="btn btn--secondary btn--sm file-btn">${icon('upload', 16)} ${t('wiz.docUpload')}<input type="file" class="sr-only" accept=".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/*" data-upload="${tipo}"></label>` : ''}
            </li>`)}
          </ul>
          ${canUpload ? html`<div class="upload-other">
            <label for="detOtherType">${t('det.uploadOther')}</label>
            <div class="inline-controls">
              <select id="detOtherType">${TIPI_DOCUMENTO.map((x) => html`<option value="${x}">${docTipoLabel(x)}</option>`)}</select>
              <label class="btn btn--secondary file-btn">${icon('upload', 16)} ${t('wiz.docChoose')}<input type="file" class="sr-only" accept=".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/*" data-upload-other="1"></label>
            </div>
            <p class="field__hint">${t('upload.rules')}</p>
          </div>` : ''}
        </section>

        <section class="card" aria-labelledby="msgTitle">
          <h2 class="card__title" id="msgTitle">${t('det.messages')}</h2>
          <div class="thread">
            ${messaggi.length ? messaggi.map((m) => html`<div class="bubble ${isMine(m) ? 'bubble--mine' : ''}">
              <p class="bubble__meta">${isMine(m) ? t('det.you') : t('det.operator')} · ${fmtDateTime(m.creato_il)}</p>
              <p class="bubble__text">${m.messaggio}</p>
            </div>`) : html`<p class="muted">${t('det.noMessages')}</p>`}
          </div>
          <form class="composer" id="msgForm">
            <label class="sr-only" for="msgText">${t('det.writeMessage')}</label>
            <textarea id="msgText" name="messaggio" rows="2" maxlength="2000" placeholder="${t('det.writeMessage')}" required></textarea>
            <button type="submit" class="btn btn--primary">${icon('send', 16)} ${t('det.send')}</button>
          </form>
        </section>
      </div>

      <aside class="detail-aside">
        <section class="card card--compact">
          <h2 class="card__title">${t('det.summary')}</h2>
          ${definitionList([
    [t('det.entry'), values['3.05'] ? fmtDate(values['3.05']) : '-'],
    [t('det.exit'), values['3.06'] ? fmtDate(values['3.06']) : '-'],
    [t('det.duration'), values['3.07'] ? t('det.days', { n: values['3.07'] }) : '-'],
    [t('det.entries'), values['3.04'] ? optionLabel('3.04', values['3.04']) : '-'],
    [t('det.fee'), fmtMoney(p.importo_dovuto, p.valuta)],
    [t('det.sentOn'), fmtDate(p.inviato_il)],
  ])}
        </section>
        <section class="card card--compact">
          <h2 class="card__title">${t('det.history')}</h2>
          <ol class="timeline">
            ${[...eventi].reverse().map((e) => html`<li class="timeline__item">
              <p class="timeline__title">${timelineLabel(e)}</p>
              <p class="timeline__meta">${fmtDateTime(e.creato_il)}${isMine(e) ? ` · ${t('det.you')}` : ''}</p>
            </li>`)}
          </ol>
        </section>
      </aside>
    </div>
  `);

  const reload = () => viewDettaglio(root, params);

  on(root, 'click', '[data-open-doc]', async (_ev, btn) => {
    try {
      await openDocumento(p.id, btn.dataset.openDoc);
    } catch (err) {
      toast('err', errorMessage(err));
    }
  });
  root.addEventListener('change', async (ev) => {
    const input = ev.target;
    if (!input.matches('[data-upload], [data-upload-other]')) return;
    const file = input.files?.[0];
    if (!file) return;
    const tipo = input.dataset.upload || $('#detOtherType', root)?.value || 'ALTRO';
    const label = input.closest('label');
    label?.classList.add('is-busy');
    try {
      const { stored } = await uploadDocumento(p.id, tipo, file);
      toast(stored ? 'ok' : 'info', stored ? t('upload.ok', { name: file.name }) : t('upload.metaOnly'));
      reload();
    } catch (err) {
      toast('err', errorMessage(err));
    } finally {
      label?.classList.remove('is-busy');
      input.value = '';
    }
  });
  $('#msgForm', root)?.addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const text = $('#msgText', root).value.trim();
    if (!text) return;
    await busy(ev.submitter, async () => {
      try {
        await api(`/api/pratiche/${encodeURIComponent(p.id)}/messaggi`, { method: 'POST', json: { messaggio: text } });
        toast('ok', t('det.msgSent'));
        reload();
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  });
}
