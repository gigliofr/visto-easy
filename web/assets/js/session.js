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

export function hasBackofficeRole() {
  return role() === 'ADMIN' || role() === 'OPERATORE' || role() === 'SUPERVISORE';
}

export function hasRichiedenteRole() {
  return role() === 'RICHIEDENTE';
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
