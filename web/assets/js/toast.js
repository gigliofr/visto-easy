let region = null;

function ensureRegion() {
  if (region && document.body.contains(region)) return region;
  region = document.getElementById('toastRegion');
  if (!region) {
    region = document.createElement('div');
    region.id = 'toastRegion';
    region.className = 'toast-region';
    region.setAttribute('aria-live', 'polite');
    document.body.appendChild(region);
  }
  return region;
}

export function toast(kind, text) {
  if (!text) return;
  const el = document.createElement('div');
  el.className = `toast toast--${kind === 'err' ? 'error' : kind === 'ok' ? 'success' : 'info'}`;
  el.setAttribute('role', kind === 'err' ? 'alert' : 'status');
  el.textContent = text;
  ensureRegion().appendChild(el);
  setTimeout(() => el.classList.add('toast--leaving'), 4200);
  setTimeout(() => el.remove(), 4700);
}

// busy disabilita un pulsante durante un'operazione asincrona.
export async function busy(button, fn) {
  if (button) {
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
  }
  try {
    return await fn();
  } finally {
    if (button) {
      button.disabled = false;
      button.removeAttribute('aria-busy');
    }
  }
}
