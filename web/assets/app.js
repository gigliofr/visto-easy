const state = {
  accessToken: localStorage.getItem('ve_access_token') || '',
  refreshToken: localStorage.getItem('ve_refresh_token') || '',
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
  twofaOutput: document.getElementById('twofaOutput'),
  miePratiche: document.getElementById('miePratiche'),
  boPratiche: document.getElementById('boPratiche'),
  boUtenti: document.getElementById('boUtenti'),
  boAudit: document.getElementById('boAudit'),
};

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
}

function clearSession() {
  setSession({ accessToken: '', refreshToken: '', user: null });
}

function renderSessionInfo() {
  const role = state.user?.ruolo || 'ospite';
  const email = state.user?.email || 'non autenticato';
  els.sessionRole.textContent = `Ruolo: ${role}`;
  els.sessionUser.textContent = `Utente: ${email}`;
}

function out(title, payload) {
  const ts = new Date().toISOString();
  const block = `[${ts}] ${title}\n${JSON.stringify(payload, null, 2)}\n\n`;
  els.appOutput.textContent = block + els.appOutput.textContent;
}

async function api(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (state.accessToken) {
    headers.Authorization = `Bearer ${state.accessToken}`;
  }
  const response = await fetch(path, { ...options, headers });
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
    bar.querySelector('[data-action="submit"]').addEventListener('click', async () => {
      const res = await api(`/api/pratiche/${praticaID}/submit`, { method: 'POST' });
      out('Pratica submit', res);
      await loadMiePratiche();
    });
    bar.querySelector('[data-action="delete"]').addEventListener('click', async () => {
      await api(`/api/pratiche/${praticaID}`, { method: 'DELETE' });
      out('Pratica eliminata', { id: praticaID });
      await loadMiePratiche();
    });
    bar.querySelector('[data-action="docs"]').addEventListener('click', async () => {
      const docs = await api(`/api/pratiche/${praticaID}/documenti`);
      out(`Documenti pratica ${praticaID}`, docs);
    });
  });

  out('Pratiche caricate', { count: data.length });
}

async function loadBOPratiche(query = {}) {
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
  out('BO pratiche caricate', { total: data.total || 0 });
}

async function loadBOUsers() {
  const data = await api('/api/bo/utenti?page=1&page_size=20');
  const items = data.items || [];
  renderList(els.boUtenti, items, (u) => `
    <article class="list-item">
      <h3>${u.email}</h3>
      <p>${u.nome || ''} ${u.cognome || ''} | ${u.ruolo}</p>
    </article>
  `);
  out('BO utenti caricati', { total: data.total || 0 });
}

async function loadBOAudit() {
  const data = await api('/api/bo/audit-events?page=1&page_size=20');
  const items = data.items || [];
  renderList(els.boAudit, items, (e) => `
    <article class="list-item">
      <h3>${e.action}</h3>
      <p>${e.actor_role || '-'} | ${e.resource} ${e.resource_id || ''}</p>
    </article>
  `);
  out('Audit eventi caricati', { total: data.total || 0 });
}

function wireForms() {
  document.getElementById('formLogin').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api('/api/auth/login', { method: 'POST', body: JSON.stringify(payload) });
    setSession({ accessToken: data.access_token, refreshToken: data.refresh_token, user: data.user });
    out('Login completato', { user: data.user });
  });

  document.getElementById('formRegister').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api('/api/auth/register', { method: 'POST', body: JSON.stringify(payload) });
    out('Registrazione completata', data);
  });

  document.getElementById('formForgot').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api('/api/auth/forgot-password', { method: 'POST', body: JSON.stringify(payload) });
    out('Forgot password', data);
  });

  document.getElementById('formReset').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api('/api/auth/reset-password', { method: 'POST', body: JSON.stringify(payload) });
    out('Reset password', data);
  });

  document.getElementById('btn2FASetup').addEventListener('click', async () => {
    const data = await api('/api/auth/2fa/setup', { method: 'POST', body: '{}' });
    els.twofaOutput.textContent = JSON.stringify(data, null, 2);
    out('2FA setup', data);
  });

  document.getElementById('form2FAEnable').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api('/api/auth/2fa/enable', { method: 'POST', body: JSON.stringify(payload) });
    out('2FA enable', data);
  });

  document.getElementById('form2FADisable').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api('/api/auth/2fa/disable', { method: 'POST', body: JSON.stringify(payload) });
    out('2FA disable', data);
  });

  document.getElementById('formCreatePratica').addEventListener('submit', async (ev) => {
    ev.preventDefault();
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
    await loadMiePratiche();
  });

  document.getElementById('btnLoadMiePratiche').addEventListener('click', () => loadMiePratiche());

  document.getElementById('formDocUpload').addEventListener('submit', async (ev) => {
    ev.preventDefault();
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
  });

  document.getElementById('formBOFilters').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    await loadBOPratiche({ q: payload.q, stato: payload.stato });
  });

  document.getElementById('btnLoadUsers').addEventListener('click', () => loadBOUsers());
  document.getElementById('btnLoadAudit').addEventListener('click', () => loadBOAudit());

  document.getElementById('formBOChangeState').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api(`/api/bo/pratiche/${payload.id}/stato`, {
      method: 'PATCH',
      body: JSON.stringify({ stato: payload.stato, nota: payload.nota }),
    });
    out('BO cambio stato', data);
    await loadBOPratiche();
  });

  document.getElementById('formBOAddNote').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const payload = formJson(ev.currentTarget);
    const data = await api(`/api/bo/pratiche/${payload.id}/note`, {
      method: 'POST',
      body: JSON.stringify({
        messaggio: payload.messaggio,
        interna: payload.interna === 'true',
      }),
    });
    out('BO aggiungi nota', data);
  });
}

function wireSessionButtons() {
  els.btnLogout.addEventListener('click', async () => {
    if (!state.refreshToken) {
      clearSession();
      out('Logout locale', { status: 'no_refresh_token' });
      return;
    }
    const data = await api('/api/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: state.refreshToken }),
    });
    clearSession();
    out('Logout server', data);
  });

  els.btnRefreshSession.addEventListener('click', async () => {
    if (!state.refreshToken) {
      out('Refresh saltato', { reason: 'missing refresh token' });
      return;
    }
    const data = await api('/api/auth/refresh', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: state.refreshToken }),
    });
    setSession({ accessToken: data.access_token, refreshToken: data.refresh_token, user: state.user });
    out('Sessione aggiornata', { status: 'ok' });
  });
}

function boot() {
  renderSessionInfo();
  wireTabs();
  wireForms();
  wireSessionButtons();
  out('Frontend inizializzato', {
    hasToken: Boolean(state.accessToken),
    role: state.user?.ruolo || 'ospite',
  });
}

boot();
