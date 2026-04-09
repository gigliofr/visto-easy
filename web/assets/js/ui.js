import { hasActiveSession, hasBackofficeRole, hasRichiedenteRole, role, state } from './session.js';

export const els = {
  appOutput: document.getElementById('appOutput'),
  sessionRole: document.getElementById('sessionRole'),
  sessionUser: document.getElementById('sessionUser'),
  boSessionRole: document.getElementById('boSessionRole'),
  boSessionUser: document.getElementById('boSessionUser'),
  btnSettings: document.getElementById('btnSettings'),
  boBtnSettings: document.getElementById('boBtnSettings'),
  btnLogout: document.getElementById('btnLogout'),
  boBtnLogout: document.getElementById('boBtnLogout'),
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
  panelProfilo: document.getElementById('profilo'),
  panelRichiedente: document.getElementById('richiedente'),
  panelBackoffice: document.getElementById('backoffice'),
};

export function renderSessionInfo() {
  const currentRole = role() || 'ospite';
  const email = state.user?.email || 'non autenticato';
  els.sessionRole.textContent = `Ruolo: ${currentRole}`;
  els.sessionUser.textContent = `Utente: ${email}`;
  if (els.boSessionRole) els.boSessionRole.textContent = `Ruolo: ${currentRole}`;
  if (els.boSessionUser) els.boSessionUser.textContent = `Utente: ${email}`;
  els.apiBaseInput.value = state.apiBase;
  const loggedIn = Boolean(hasActiveSession() && state.user?.id && state.user?.email);
  document.body.dataset.auth = loggedIn ? '1' : '0';
  if (els.btnSettings) {
    els.btnSettings.hidden = !loggedIn;
    els.btnSettings.setAttribute('aria-hidden', loggedIn ? 'false' : 'true');
    els.btnSettings.disabled = !loggedIn;
  }
  if (els.boBtnSettings) {
    els.boBtnSettings.hidden = !loggedIn;
    els.boBtnSettings.setAttribute('aria-hidden', loggedIn ? 'false' : 'true');
    els.boBtnSettings.disabled = !loggedIn;
  }
  if (els.btnRefreshSession) els.btnRefreshSession.hidden = !loggedIn;
  if (els.btnLogout) els.btnLogout.hidden = !loggedIn;
  if (els.boBtnLogout) els.boBtnLogout.hidden = !loggedIn;
}

export function notify(kind, text) {
  const toast = document.createElement('article');
  toast.className = `toast ${kind}`;
  toast.textContent = text;
  els.toastRegion.prepend(toast);
  window.setTimeout(() => {
    toast.remove();
  }, 3000);
}

export function out(title, payload) {
  const ts = new Date().toISOString();
  const block = `[${ts}] ${title}\n${JSON.stringify(payload, null, 2)}\n\n`;
  els.appOutput.textContent = block + els.appOutput.textContent;
}

export function extractErrMessage(err) {
  if (!err) return 'errore sconosciuto';
  const data = err.data;
  if (typeof data === 'string' && data.trim()) return data;
  if (data && typeof data.error === 'string') return data.error;
  return `errore ${err.status || ''}`.trim();
}

export function setBusy(el, busy) {
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

export async function withBusy(buttonEl, op) {
  try {
    setBusy(buttonEl, true);
    return await op();
  } finally {
    setBusy(buttonEl, false);
  }
}

export function renderList(container, items, mapper) {
  if (!items || items.length === 0) {
    container.innerHTML = '<div class="list-item"><p>Nessun elemento</p></div>';
    return;
  }
  container.innerHTML = items.map(mapper).join('');
}

export function setSectionFromHash() {
  const hash = (window.location.hash || '#auth').toLowerCase();
  const valid = ['#auth', '#profilo', '#richiedente', '#backoffice'];
  const current = valid.includes(hash) ? hash : '#auth';
  document.body.dataset.section = current.replace('#', '');
  [els.panelAuth, els.panelProfilo, els.panelRichiedente, els.panelBackoffice].forEach((panel) => panel.classList.add('hidden'));
  if (current === '#auth') els.panelAuth.classList.remove('hidden');
  if (current === '#profilo') els.panelProfilo.classList.remove('hidden');
  if (current === '#richiedente') els.panelRichiedente.classList.remove('hidden');
  if (current === '#backoffice') els.panelBackoffice.classList.remove('hidden');
}

export function applyRoleGuards() {
  const currentHash = String(window.location.hash || '#auth').toLowerCase();
  if (!hasActiveSession()) {
    els.panelAuth.classList.remove('hidden');
    els.panelProfilo.classList.add('hidden');
    els.panelRichiedente.classList.add('hidden');
    els.panelBackoffice.classList.add('hidden');
    return;
  }
  if (currentHash === '#profilo') {
    els.panelAuth.classList.add('hidden');
    els.panelProfilo.classList.remove('hidden');
    els.panelRichiedente.classList.add('hidden');
    els.panelBackoffice.classList.add('hidden');
    return;
  }
  if (hasRichiedenteRole()) {
    if (window.location.hash === '#backoffice') window.location.hash = '#richiedente';
    els.panelAuth.classList.add('hidden');
    els.panelProfilo.classList.add('hidden');
    els.panelRichiedente.classList.remove('hidden');
    els.panelBackoffice.classList.add('hidden');
    return;
  }
  if (hasBackofficeRole()) {
    if (window.location.hash === '#richiedente') window.location.hash = '#backoffice';
    els.panelAuth.classList.add('hidden');
    els.panelProfilo.classList.add('hidden');
    els.panelRichiedente.classList.add('hidden');
    els.panelBackoffice.classList.remove('hidden');
    return;
  }
  els.panelAuth.classList.remove('hidden');
  els.panelProfilo.classList.add('hidden');
  els.panelRichiedente.classList.add('hidden');
  els.panelBackoffice.classList.add('hidden');
}
