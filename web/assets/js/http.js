import { state } from './session.js';
import { out } from './ui.js';

function fullPath(path) {
  if (!state.apiBase) return path;
  return `${state.apiBase}${path}`;
}

export async function api(path, options = {}) {
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

export function formJson(form) {
  const fd = new FormData(form);
  const obj = {};
  for (const [k, v] of fd.entries()) {
    obj[k] = String(v).trim();
  }
  return obj;
}
