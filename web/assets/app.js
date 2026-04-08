const state = {
  accessToken: localStorage.getItem('ve_access_token') || '',
  refreshToken: localStorage.getItem('ve_refresh_token') || '',
  apiBase: localStorage.getItem('ve_api_base') || '',
  user: (() => {
    const raw = localStorage.getItem('ve_user');
    if (!raw) return null;
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  })(),
};

const els = {
  appOutput: document.getElementById('appOutput'),
  sessionRole: document.getElementById('sessionRole'),
  sessionUser: document.getElementById('sessionUser'),
  btnLogout: document.getElementById('btnLogout'),
  btnRefreshSession: document.getElementById('btnRefreshSession'),
  btnSaveApiBase: document.getElementById('btnSaveApiBase'),
  apiBaseInput: document.getElementById('apiBaseInput'),
  twofaOutput: document.getElementById('twofaOutput'),
  miePratiche: document.getElementById('miePratiche'),
  boPratiche: document.getElementById('boPratiche'),
  boUtenti: document.getElementById('boUtenti'),
  boAudit: document.getElementById('boAudit'),
  kpiPratiche: document.getElementById('kpiPratiche'),
  kpiUtenti: document.getElementById('kpiUtenti'),
  kpiAudit: document.getElementById('kpiAudit'),
  toastRegion: document.getElementById('toastRegion'),
  panelAuth: document.getElementById('auth'),
  panelRichiedente: document.getElementById('richiedente'),
  panelBackoffice: document.getElementById('backoffice'),
};

function role() {
  return state.user?.ruolo || '';
}

function hasBackofficeRole() {
  return role() === 'ADMIN' || role() === 'OPERATORE' || role() === 'SUPERVISORE';
}

function hasRichiedenteRole() {
  return role() === 'RICHIEDENTE';
}

function saveApiBase(value) {
  state.apiBase = (value || '').trim().replace(/\/$/, '');
  if (state.apiBase) {
    localStorage.setItem('ve_api_base', state.apiBase);
  } else {
    localStorage.removeItem('ve_api_base');
  }
}

function fullPath(path) {
  if (!state.apiBase) return path;
  return `${state.apiBase}${path}`;
}

function setSession(next) {
  state.accessToken = next.accessToken || '';
  state.refreshToken = next.refreshToken || '';
  state.user = next.user || null;
  localStorage.setItem('ve_access_token', state.accessToken);
  localStorage.setItem('ve_refresh_token', state.refreshToken);
  if (state.user) {
    localStorage.setItem('ve_user', JSON.stringify(state.user));
  } else {
    localStorage.removeItem('ve_user');
  }
  renderSessionInfo();
  applyRoleGuards();
}

function clearSession() {
  setSession({ accessToken: '', refreshToken: '', user: null });
}

function renderSessionInfo() {
  const currentRole = role() || 'ospite';
  const email = state.user?.email || 'non autenticato';
  els.sessionRole.textContent = `Ruolo: ${currentRole}`;
  els.sessionUser.textContent = `Utente: ${email}`;
  els.apiBaseInput.value = state.apiBase;
}

function notify(kind, text) {
  const toast = document.createElement('article');
  toast.className = `toast ${kind}`;
  toast.textContent = text;
  els.toastRegion.prepend(toast);
  window.setTimeout(() => {
    toast.remove();
  }, 3000);
}

function out(title, payload) {
  const ts = new Date().toISOString();
  const block = `[${ts}] ${title}\n${JSON.stringify(payload, null, 2)}\n\n`;
  els.appOutput.textContent = block + els.appOutput.textContent;
}

function extractErrMessage(err) {
  if (!err) return 'errore sconosciuto';
  const data = err.data;
  if (typeof data === 'string' && data.trim()) return data;
  if (data && typeof data.error === 'string') return data.error;
  return `errore ${err.status || ''}`.trim();
}

function setBusy(el, busy) {
  if (!el) return;
  if (busy) {
    el.dataset.prevText = el.textContent;
    el.disabled = true;
    el.textContent = 'Attendere...';
    return;
  }
  el.disabled = false;
  if (el.dataset.prevText) {
    el.textContent = el.dataset.prevText;
    delete el.dataset.prevText;
  }
}

async function withBusy(buttonEl, op) {
  try {
    setBusy(buttonEl, true);
    return await op();
  } finally {
    setBusy(buttonEl, false);
  }
}

async function api(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (state.accessToken) {
    headers.Authorization = `Bearer ${state.accessToken}`;
  }
  const response = await fetch(fullPath(path), { ...options, headers });
  const text = await response.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }
  if (!response.ok) {
    const err = { status: response.status, data };
    out(`Errore ${path}`, err);
    throw err;
  }
  return data;
}

function formJson(form) {
  const fd = new FormData(form);
  const obj = {};
  for (const [k, v] of fd.entries()) {
    obj[k] = String(v).trim();
  }
  return obj;
}

function renderList(container, items, mapper) {
  if (!items || items.length === 0) {
    container.innerHTML = '<div class="list-item"><p>Nessun elemento</p></div>';
    return;
  }
  container.innerHTML = items.map(mapper).join('');
}

function setSectionFromHash() {
  const hash = (window.location.hash || '#auth').toLowerCase();
  const valid = ['#auth', '#richiedente', '#backoffice'];
  const current = valid.includes(hash) ? hash : '#auth';
  [els.panelAuth, els.panelRichiedente, els.panelBackoffice].forEach((panel) => panel.classList.add('hidden'));
  if (current === '#auth') els.panelAuth.classList.remove('hidden');
  if (current === '#richiedente') els.panelRichiedente.classList.remove('hidden');
  if (current === '#backoffice') els.panelBackoffice.classList.remove('hidden');
}

function applyRoleGuards() {
  if (!state.accessToken) {
    els.panelRichiedente.classList.add('hidden');
    els.panelBackoffice.classList.add('hidden');
    return;
  }
  if (hasRichiedenteRole()) {
    if (window.location.hash === '#backoffice') window.location.hash = '#richiedente';
    els.panelAuth.classList.remove('hidden');
    els.panelRichiedente.classList.remove('hidden');
    els.panelBackoffice.classList.add('hidden');
    return;
  }
  if (hasBackofficeRole()) {
    if (window.location.hash === '#richiedente') window.location.hash = '#backoffice';
    els.panelAuth.classList.remove('hidden');
    els.panelRichiedente.classList.add('hidden');
    els.panelBackoffice.classList.remove('hidden');
    return;
  }
  els.panelRichiedente.classList.add('hidden');
  els.panelBackoffice.classList.add('hidden');
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

async function loadMiePratiche() {
  if (!hasRichiedenteRole()) {
    notify('err', 'Azione riservata al ruolo RICHIEDENTE');
    return;
  }
  const data = await api('/api/pratiche/');
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
          out(`Documenti pratica ${praticaID}`, docs);
          notify('ok', `Documenti caricati: ${docs.length || 0}`);
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
        await loadMiePratiche();
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
    notify('ok', state.apiBase ? `API base salvata: ${state.apiBase}` : 'API base reimpostata su locale');
    out('API base aggiornata', { api_base: state.apiBase || '(relativa)' });
  });

  els.btnLogout.addEventListener('click', async (ev) => {
    try {
      await withBusy(ev.currentTarget, async () => {
        if (!state.refreshToken) {
          clearSession();
          out('Logout locale', { status: 'no_refresh_token' });
          notify('ok', 'Sessione locale chiusa');
          return;
        }
        const data = await api('/api/auth/logout', {
          method: 'POST',
          body: JSON.stringify({ refresh_token: state.refreshToken }),
        });
        clearSession();
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
        out('Sessione aggiornata', { status: 'ok' });
        notify('ok', 'Sessione aggiornata');
      });
    } catch (err) {
      notify('err', extractErrMessage(err));
    }
  });
}

function boot() {
  renderSessionInfo();
  wireTabs();
  wireForms();
  wireSessionButtons();
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

  out('Frontend inizializzato', {
    has_token: Boolean(state.accessToken),
    role: role() || 'ospite',
    api_base: state.apiBase || '(relativa)',
  });
}

boot();
