import { clearSession, isAccessTokenExpired, setSession, state } from './session.js';

function fullPath(path) {
  if (!state.apiBase) return path;
  return `${state.apiBase}${path}`;
}

let refreshing = null;

// refreshTokens rinnova l'access token con il cookie HttpOnly del refresh token,
// che il browser invia da solo. Un token rimasto in localStorage dalle versioni
// precedenti viene mandato un'ultima volta: il server lo sposta nel cookie.
async function refreshTokens() {
  if (!refreshing) {
    refreshing = (async () => {
      try {
        const res = await fetch(fullPath('/api/auth/refresh'), {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(state.refreshToken ? { refresh_token: state.refreshToken } : {}),
        });
        if (!res.ok) return false;
        const data = await res.json();
        if (!data || !data.access_token) return false;
        setSession({ accessToken: data.access_token, user: data.user || state.user });
        return true;
      } catch {
        return false;
      } finally {
        setTimeout(() => { refreshing = null; }, 0);
      }
    })();
  }
  return refreshing;
}

async function rawRequest(path, options) {
  const headers = { ...(options.headers || {}) };
  if (options.body !== undefined && !(options.body instanceof Blob) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }
  if (state.accessToken) headers.Authorization = `Bearer ${state.accessToken}`;
  return fetch(fullPath(path), { ...options, headers });
}

async function parse(response) {
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

// api esegue una chiamata JSON. Su 401 prova una volta a rinnovare i token.
// In caso di errore lancia { status, data, message }.
export async function api(path, options = {}) {
  const opts = { ...options };
  if (opts.json !== undefined) {
    opts.body = JSON.stringify(opts.json);
    delete opts.json;
  }
  let response = await rawRequest(path, opts);
  if (response.status === 401 && !path.startsWith('/api/auth/') && await refreshTokens()) {
    response = await rawRequest(path, opts);
  }
  const data = await parse(response);
  if (!response.ok) {
    if (response.status === 401 && !path.startsWith('/api/auth/')) {
      clearSession();
      window.dispatchEvent(new CustomEvent('app:session-expired'));
    }
    const message = (data && typeof data === 'object' && (data.error || data.message)) || (typeof data === 'string' ? data : '') || `HTTP ${response.status}`;
    throw { status: response.status, data, message };
  }
  return data;
}

export async function apiBlob(path) {
  let response = await rawRequest(path, {});
  if (response.status === 401 && await refreshTokens()) response = await rawRequest(path, {});
  if (!response.ok) throw { status: response.status, message: `HTTP ${response.status}` };
  return response.blob();
}

// restoreSession riprende la sessione all'avvio quando l'access token (15 minuti)
// è scaduto ma il cookie del refresh token è ancora valido.
export async function restoreSession() {
  if (state.user && isAccessTokenExpired()) {
    if (!(await refreshTokens())) clearSession();
  }
}

export function errorMessage(err) {
  if (!err) return '';
  if (typeof err === 'string') return err;
  return err.message || (err.data && err.data.error) || String(err);
}
