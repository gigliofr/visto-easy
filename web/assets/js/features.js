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
  populateDocPraticaSelect(data, preferredPraticaID);
  renderList(els.miePratiche, data, (p) => `
    <article class="list-item">
      <h3>${p.codice || p.id} - ${p.stato}</h3>
      <p>${p.tipo_visto || '-'} | ${p.paese_dest || '-'} | ${new Date(p.creato_il).toLocaleString()}</p>
      <div class="inline-actions" data-pratica-id="${p.id}">
        <button class="btn btn-ghost" type="button" data-action="submit">Submit</button>
        <button class="btn btn-ghost" type="button" data-action="delete">Delete</button>
        <button class="btn btn-ghost" type="button" data-action="docs">Documenti</button>
      </div>
    </article>
  `);

  els.miePratiche.querySelectorAll('[data-pratica-id]').forEach((bar) => {
    const praticaID = bar.dataset.praticaId;
    bar.querySelector('[data-action="submit"]').addEventListener('click', async (ev) => {
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

    bar.querySelector('[data-action="delete"]').addEventListener('click', async (ev) => {
      try {
        await withBusy(ev.currentTarget, async () => {
          await api(`/api/pratiche/${praticaID}`, { method: 'DELETE' });
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

  out('Pratiche caricate', { count: data.length });
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
  const params = new URLSearchParams();
  if (query.q) params.set('q', query.q);
  if (query.stato) params.set('stato', query.stato);
  const data = await api(`/api/bo/pratiche?${params.toString()}`);
  const items = data.items || [];
  renderList(els.boPratiche, items, (row) => {
    const p = row.pratica;
    const u = row.richiedente || {};
    return `
      <article class="list-item">
        <h3>${p.codice || p.id} - ${p.stato}</h3>
        <p>${u.email || '-'} | ${p.tipo_visto || '-'} ${p.paese_dest || '-'}</p>
      </article>
    `;
  });
  els.kpiPratiche.textContent = String(data.total || 0);
  out('BO pratiche caricate', { total: data.total || 0, by_status: computeStatusBreakdown(items) });
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
        if (data.user?.ruolo === 'BACKOFFICE') window.location.hash = '#backoffice';
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
        await loadBOPratiche({ q: payload.q, stato: payload.stato });
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

export function initApp(bootMessage) {
  if (window.location.pathname.toLowerCase().startsWith('/backoffice')) {
    window.location.hash = '#backoffice';
  }

  renderSessionInfo();
  wireTabs();
  wireForms();
  wireSessionButtons();
  wireDocUploadMetadata();
  loadCountriesIntoSelect().catch(() => {});
  setSectionFromHash();
  applyRoleGuards();
  window.addEventListener('hashchange', () => {
    setSectionFromHash();
    applyRoleGuards();
  });

  if (hasBackofficeRole()) {
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
