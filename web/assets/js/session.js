export const state = {
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

export function role() {
  return state.user?.ruolo || '';
}

function decodeJwtPayload(token) {
  try {
    const parts = String(token || '').split('.');
    if (parts.length < 2) return null;
    const normalized = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const pad = normalized.length % 4;
    const padded = normalized + (pad ? '='.repeat(4 - pad) : '');
    return JSON.parse(atob(padded));
  } catch {
    return null;
  }
}

export function isAccessTokenExpired() {
  if (!state.accessToken) return true;
  const payload = decodeJwtPayload(state.accessToken);
  if (!payload || typeof payload.exp !== 'number') return true;
  const nowSec = Math.floor(Date.now() / 1000);
  return payload.exp <= nowSec;
}

export function hasActiveSession() {
  return Boolean(state.accessToken && state.user?.email && !isAccessTokenExpired());
}

export function hasBackofficeRole() {
  return hasActiveSession() && (role() === 'ADMIN' || role() === 'OPERATORE' || role() === 'SUPERVISORE');
}

export function hasRichiedenteRole() {
  return hasActiveSession() && role() === 'RICHIEDENTE';
}

export function saveApiBase(value) {
  state.apiBase = (value || '').trim().replace(/\/$/, '');
  if (state.apiBase) {
    localStorage.setItem('ve_api_base', state.apiBase);
  } else {
    localStorage.removeItem('ve_api_base');
  }
}

export function setSession(next) {
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
}

export function clearSession() {
  setSession({ accessToken: '', refreshToken: '', user: null });
}
