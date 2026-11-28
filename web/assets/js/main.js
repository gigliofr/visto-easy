// Avvio dell'applicazione: accesso, guscio, router e impostazioni.
import { api, errorMessage, restoreSession } from './http.js';
import { $, $$, html, icon, on, render, fresh } from './dom.js';
import { initI18n, setLang, getLang, t, applyTranslations } from './i18n.js';
import { toast, busy } from './toast.js';
import {
  state, setSession, clearSession, hasActiveSession, hasBackofficeRole, saveApiBase,
} from './session.js';
import { initials, fullName } from './domain.js';
import { pageHeader, definitionList, badge } from './components.js';
import { viewDashboard, viewWizard, viewDettaglio } from './rich.js';
import {
  viewQueue, viewAll, viewBODettaglio, viewClienti, viewOperatori, viewAudit,
} from './bo.js';

// ---------------------------------------------------------------------------
// Router
// ---------------------------------------------------------------------------

const RICH_ROUTES = [
  [/^\/pratiche\/?$/, () => ({ view: viewDashboard, nav: 'pratiche' })],
  [/^\/pratiche\/nuova$/, () => ({ view: viewWizard, nav: 'nuova' })],
  [/^\/pratiche\/([\w-]+)\/modifica(?:\/(\d))?$/, (m) => ({ view: viewWizard, params: { id: m[1], step: m[2] }, nav: 'pratiche' })],
  [/^\/pratiche\/([\w-]+)$/, (m) => ({ view: viewDettaglio, params: { id: m[1] }, nav: 'pratiche' })],
  [/^\/impostazioni$/, () => ({ view: viewSettings, nav: 'impostazioni' })],
];

const BO_ROUTES = [
  [/^\/bo\/coda$/, () => ({ view: viewQueue, nav: 'coda' })],
  [/^\/bo\/pratiche$/, () => ({ view: viewAll, nav: 'tutte' })],
  [/^\/bo\/pratiche\/([\w-]+)$/, (m) => ({ view: viewBODettaglio, params: { id: m[1] }, nav: 'coda' })],
  [/^\/bo\/clienti$/, () => ({ view: viewClienti, nav: 'clienti' })],
  [/^\/bo\/operatori$/, () => ({ view: viewOperatori, nav: 'operatori' })],
  [/^\/bo\/audit$/, () => ({ view: viewAudit, nav: 'audit' })],
  [/^\/impostazioni$/, () => ({ view: viewSettings, nav: 'impostazioni' })],
];

function currentPath() {
  return (window.location.hash || '').replace(/^#/, '') || '';
}

function defaultRoute() {
  return hasBackofficeRole() ? '/bo/coda' : '/pratiche';
}

function resolve(path) {
  const routes = hasBackofficeRole() ? BO_ROUTES : RICH_ROUTES;
  for (const [re, make] of routes) {
    const m = re.exec(path);
    if (m) return make(m);
  }
  return null;
}

async function route() {
  if (!hasActiveSession()) {
    showLanding();
    return;
  }
  showShell();
  const path = currentPath();
  const match = resolve(path);
  if (!match) {
    window.location.replace(`#${defaultRoute()}`);
    return;
  }
  renderChrome(match.nav);
  const app = fresh($('#app'));
  app.id = 'app';
  window.scrollTo(0, 0);
  await match.view(app, match.params || {});
  const main = $('#app');
  const h1 = main?.querySelector('h1');
  if (h1) document.title = `${h1.textContent.trim()} · Visto Easy`;
}

// ---------------------------------------------------------------------------
// Guscio: barra superiore, barra laterale (backoffice), tab bar (mobile)
// ---------------------------------------------------------------------------

function brand() {
  return html`<a class="brand" href="#${defaultRoute()}"><span class="brand__mark" aria-hidden="true">${icon('check', 18)}</span>Visto Easy</a>`;
}

function userMenu() {
  const u = state.user || {};
  return html`<details class="user-menu">
    <summary class="user-menu__btn"><span class="avatar" aria-hidden="true">${initials(u)}</span><span class="user-menu__name">${fullName(u)}</span></summary>
    <div class="user-menu__panel">
      <p class="user-menu__email">${u.email || ''}</p>
      <a href="#/impostazioni">${icon('settings', 16)} ${t('nav.settings')}</a>
      <div class="user-menu__lang" role="group" aria-label="${t('settings.language')}">
        <button type="button" data-lang-choice="it" aria-pressed="${getLang() === 'it'}">Italiano</button>
        <button type="button" data-lang-choice="en" aria-pressed="${getLang() === 'en'}">English</button>
      </div>
      <button type="button" data-logout>${icon('logout', 16)} ${t('nav.logout')}</button>
    </div>
  </details>`;
}

function renderChrome(active) {
  const bo = hasBackofficeRole();
  document.body.classList.toggle('is-bo', bo);
  if (bo) {
    render($('#topbar'), html`<div class="topbar__inner">${brand()}<span class="topbar__tag">${t('nav.backoffice')}</span><div class="topbar__end">${userMenu()}</div></div>`);
    const items = [
      ['coda', '#/bo/coda', 'inbox', t('nav.queue')],
      ['tutte', '#/bo/pratiche', 'list', t('nav.allCases')],
      ['clienti', '#/bo/clienti', 'users', t('nav.clients')],
      ['operatori', '#/bo/operatori', 'user', t('nav.operators')],
      ['audit', '#/bo/audit', 'shield', t('nav.audit')],
      ['impostazioni', '#/impostazioni', 'settings', t('nav.settings')],
    ];
    const sb = $('#sidebar');
    sb.hidden = false;
    render(sb, html`${items.map(([key, href, ic, label]) => html`<a href="${href}" class="sidebar__link ${active === key ? 'is-active' : ''}" ${active === key ? 'aria-current="page"' : ''}>${icon(ic, 18)}<span>${label}</span></a>`)}
      <div class="sidebar__user"><strong>${fullName(state.user)}</strong><span>${t(`role.${state.user?.ruolo}`)}</span></div>`);
    $('#tabbar').hidden = true;
  } else {
    const items = [
      ['pratiche', '#/pratiche', t('nav.myCases')],
      ['nuova', '#/pratiche/nuova', t('nav.newCase')],
      ['impostazioni', '#/impostazioni', t('nav.settings')],
    ];
    render($('#topbar'), html`<div class="topbar__inner">${brand()}
      <nav class="topnav" aria-label="${t('nav.main')}">${items.map(([key, href, label]) => html`<a href="${href}" class="topnav__link ${active === key ? 'is-active' : ''}" ${active === key ? 'aria-current="page"' : ''}>${label}</a>`)}</nav>
      <div class="topbar__end">${userMenu()}</div></div>`);
    $('#sidebar').hidden = true;
    const tb = $('#tabbar');
    tb.hidden = false;
    render(tb, html`
      <a href="#/pratiche" class="tabbar__link ${active === 'pratiche' ? 'is-active' : ''}" ${active === 'pratiche' ? 'aria-current="page"' : ''}>${icon('home', 22)}<span>${t('nav.tabCases')}</span></a>
      <a href="#/pratiche/nuova" class="tabbar__link tabbar__link--primary ${active === 'nuova' ? 'is-active' : ''}">${icon('plus', 22)}<span>${t('nav.tabNew')}</span></a>
      <a href="#/impostazioni" class="tabbar__link ${active === 'impostazioni' ? 'is-active' : ''}" ${active === 'impostazioni' ? 'aria-current="page"' : ''}>${icon('user', 22)}<span>${t('nav.tabProfile')}</span></a>`);
  }
}

function showLanding() {
  document.body.dataset.auth = '0';
  $('#landing').hidden = false;
  $('#shell').hidden = true;
  document.title = 'Visto Easy';
}

function showShell() {
  document.body.dataset.auth = '1';
  $('#landing').hidden = true;
  $('#shell').hidden = false;
}

async function logout() {
  const refresh = state.refreshToken;
  try {
    // Il server revoca la sessione e cancella il cookie del refresh token.
    await api('/api/auth/logout', { method: 'POST', json: refresh ? { refresh_token: refresh } : {} });
  } catch {
    // la sessione locale viene chiusa comunque
  }
  clearSession();
  history.replaceState(null, '', window.location.pathname);
  showAuthView('login');
  route();
}

// ---------------------------------------------------------------------------
// Accesso
// ---------------------------------------------------------------------------

function showAuthView(view) {
  $$('#auth [data-auth-view]').forEach((el) => { el.hidden = el.dataset.authView !== view; });
  $$('#auth [role="tab"]').forEach((tab) => {
    const sel = tab.dataset.authTab === view;
    tab.setAttribute('aria-selected', sel ? 'true' : 'false');
    tab.classList.toggle('is-active', sel);
  });
  $('#auth .auth-tabs').hidden = !(view === 'login' || view === 'register');
}

function setFormError(id, text) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = text || '';
  el.hidden = !text;
}

const PW_RULES = {
  length: (v) => v.length >= 10,
  upper: (v) => /[A-Z]/.test(v),
  lower: (v) => /[a-z]/.test(v),
  number: (v) => /\d/.test(v),
  special: (v) => /[^A-Za-z0-9]/.test(v),
};

function updatePwRules() {
  const v = $('#regPassword').value;
  $$('#regPasswordRules [data-rule]').forEach((li) => li.classList.toggle('is-ok', PW_RULES[li.dataset.rule](v)));
}

function wireAuth() {
  on(document.body, 'click', '[data-auth-tab]', (ev, el) => {
    ev.preventDefault();
    showAuthView(el.dataset.authTab);
  });
  on(document.body, 'click', '[data-toggle-password]', (_ev, btn) => {
    const input = document.getElementById(btn.dataset.togglePassword);
    const show = input.type === 'password';
    input.type = show ? 'text' : 'password';
    btn.textContent = show ? t('auth.hide') : t('auth.show');
  });
  $('#regPassword').addEventListener('input', updatePwRules);

  $('#formLogin').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const form = ev.currentTarget;
    const email = form.email.value.trim();
    const password = form.password.value;
    const otp = form.otp.value.trim();
    if (!email || !password) {
      setFormError('loginError', t('auth.fillAll'));
      return;
    }
    setFormError('loginError', '');
    await busy(ev.submitter, async () => {
      try {
        const data = await api('/api/auth/login', { method: 'POST', json: { email, password, otp } });
        setSession({ accessToken: data.access_token, refreshToken: data.refresh_token, user: data.user });
        form.reset();
        $('#loginOtpField').hidden = true;
        const target = window.location.pathname.startsWith('/backoffice') || hasBackofficeRole() ? '/bo/coda' : '/pratiche';
        window.location.hash = `#${target}`;
        route();
      } catch (err) {
        const msg = errorMessage(err);
        if (/2fa/i.test(msg)) {
          $('#loginOtpField').hidden = false;
          $('#loginOtp').focus();
          setFormError('loginError', otp ? t('auth.otpInvalid') : t('auth.otpNeeded'));
          return;
        }
        if (err?.status === 429) {
          const secs = err.data?.retry_after_seconds;
          setFormError('loginError', secs ? t('auth.tooManyIn', { min: Math.ceil(secs / 60) }) : t('auth.tooMany'));
          return;
        }
        setFormError('loginError', err?.status === 401 ? t('auth.badCredentials') : msg);
      }
    });
  });

  $('#formRegister').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const f = ev.currentTarget;
    const body = { nome: f.nome.value.trim(), cognome: f.cognome.value.trim(), email: f.email.value.trim(), password: f.password.value };
    if (!body.nome || !body.cognome || !body.email || !body.password) return setFormError('registerError', t('auth.fillAll'));
    if (!Object.values(PW_RULES).every((rule) => rule(body.password))) return setFormError('registerError', t('auth.pwWeak'));
    if (body.password !== f.password_confirm.value) return setFormError('registerError', t('auth.pwMismatch'));
    if (!$('#regPrivacy').checked) return setFormError('registerError', t('auth.privacyNeeded'));
    setFormError('registerError', '');
    await busy(ev.submitter, async () => {
      try {
        await api('/api/auth/register', { method: 'POST', json: body });
        $('#pendingText').textContent = t('auth.pendingText', { email: body.email });
        f.reset();
        updatePwRules();
        showAuthView('pending');
      } catch (err) {
        setFormError('registerError', errorMessage(err));
      }
    });
    return undefined;
  });

  $('#formForgot').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const email = ev.currentTarget.email.value.trim();
    if (!email) return;
    await busy(ev.submitter, async () => {
      try {
        await api('/api/auth/forgot-password', { method: 'POST', json: { email } });
      } catch {
        // risposta sempre neutra: non rivela se l'email esiste
      }
      const out = $('#forgotResult');
      out.textContent = t('auth.forgotSent');
      out.hidden = false;
    });
  });
}

// ---------------------------------------------------------------------------
// Impostazioni
// ---------------------------------------------------------------------------

async function viewSettings(root) {
  root = fresh(root);
  const u = state.user || {};
  const bo = hasBackofficeRole();
  render(root, html`
    ${pageHeader(t('settings.title'))}
    <div class="settings-grid">
      <section class="card">
        <h2 class="card__title">${t('settings.account')}</h2>
        ${definitionList([
    [t('bo.name'), fullName(u)],
    ['Email', u.email],
    [t('bo.role'), t(`role.${u.ruolo}`)],
    [t('bo.emailVerified'), u.email_verificata ? t('common.yes') : t('common.no')],
  ])}
        <div class="btn-row"><button type="button" class="btn btn--secondary" data-reset-pw>${t('settings.resetPw')}</button></div>
      </section>
      <section class="card">
        <h2 class="card__title">${t('settings.language')}</h2>
        <div class="segmented" role="group" aria-label="${t('settings.language')}">
          <button type="button" data-lang-choice="it" aria-pressed="${getLang() === 'it'}">Italiano</button>
          <button type="button" data-lang-choice="en" aria-pressed="${getLang() === 'en'}">English</button>
        </div>
      </section>
      ${bo ? html`<section class="card" id="twofaCard">
        <h2 class="card__title">${t('settings.twofa')} ${u.totp_enabled ? badge(t('common.on'), 'success') : badge(t('common.off'), 'warning')}</h2>
        <p class="muted">${t('settings.twofaText')}</p>
        <div id="twofaBody">
          ${u.totp_enabled
    ? html`<form id="twofaDisable" class="stack"><label for="tfaOff">${t('settings.twofaCode')}</label><input id="tfaOff" inputmode="numeric" autocomplete="one-time-code" maxlength="8" required><button type="submit" class="btn btn--secondary">${t('settings.twofaDisable')}</button></form>`
    : html`<button type="button" class="btn btn--primary" data-twofa-setup>${t('settings.twofaSetup')}</button>`}
        </div>
      </section>` : ''}
      <section class="card">
        <h2 class="card__title">${t('settings.session')}</h2>
        <p class="muted">${t('settings.sessionText')}</p>
        <div class="btn-row"><button type="button" class="btn btn--secondary" data-logout>${icon('logout', 16)} ${t('nav.logout')}</button></div>
        <details class="more">
          <summary>${t('settings.advanced')}</summary>
          <form id="apiBaseForm" class="stack">
            <label for="apiBase">${t('settings.apiBase')}</label>
            <input id="apiBase" type="url" placeholder="https://api.example.com" value="${state.apiBase}">
            <p class="field__hint">${t('settings.apiBaseHint')}</p>
            <button type="submit" class="btn btn--secondary btn--sm">${t('common.save')}</button>
          </form>
        </details>
      </section>
    </div>`);

  on(root, 'click', '[data-reset-pw]', async (_ev, btn) => {
    await busy(btn, async () => {
      try {
        await api('/api/auth/forgot-password', { method: 'POST', json: { email: u.email } });
      } catch {
        // risposta neutra
      }
      toast('ok', t('settings.resetSent'));
    });
  });
  $('#apiBaseForm', root)?.addEventListener('submit', (ev) => {
    ev.preventDefault();
    saveApiBase($('#apiBase', root).value);
    toast('ok', t('common.saved'));
  });
  on(root, 'click', '[data-twofa-setup]', async (_ev, btn) => {
    await busy(btn, async () => {
      try {
        const res = await api('/api/auth/2fa/setup', { method: 'POST', json: {} });
        render($('#twofaBody', root), html`
          <ol class="steps-list">
            <li>${t('settings.twofaStep1')}</li>
            <li>${t('settings.twofaStep2')}<br><code class="secret">${res.secret}</code></li>
            <li>${t('settings.twofaStep3')}</li>
          </ol>
          <details class="more"><summary>${t('settings.twofaUri')}</summary><input type="text" readonly class="mono" value="${res.provisioning_uri}" aria-label="${t('settings.twofaUri')}"></details>
          <form id="twofaEnable" class="stack"><label for="tfaOn">${t('settings.twofaCode')}</label><input id="tfaOn" inputmode="numeric" autocomplete="one-time-code" maxlength="8" required><button type="submit" class="btn btn--primary">${t('settings.twofaEnable')}</button></form>`);
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  });
  root.addEventListener('submit', async (ev) => {
    const form = ev.target;
    if (form.id !== 'twofaEnable' && form.id !== 'twofaDisable') return;
    ev.preventDefault();
    const enable = form.id === 'twofaEnable';
    const code = form.querySelector('input').value.trim();
    await busy(ev.submitter, async () => {
      try {
        await api(`/api/auth/2fa/${enable ? 'enable' : 'disable'}`, { method: 'POST', json: { code } });
        setSession({ accessToken: state.accessToken, refreshToken: state.refreshToken, user: { ...state.user, totp_enabled: enable } });
        toast('ok', enable ? t('settings.twofaOn') : t('settings.twofaOffDone'));
        viewSettings(root);
      } catch (err) {
        toast('err', errorMessage(err));
      }
    });
  });
}

// ---------------------------------------------------------------------------
// Avvio
// ---------------------------------------------------------------------------

export function initApp(bootMessage) {
  initI18n();
  wireAuth();
  showAuthView('login');

  on(document.body, 'click', '[data-lang-choice]', (_ev, btn) => {
    setLang(btn.dataset.langChoice);
    $$('[data-lang-choice]').forEach((b) => b.setAttribute('aria-pressed', b.dataset.langChoice === getLang() ? 'true' : 'false'));
  });
  window.addEventListener('app:lang-change', () => {
    applyTranslations(document);
    if (hasActiveSession()) route();
  });
  on(document.body, 'click', '[data-logout]', (ev) => {
    ev.preventDefault();
    logout();
  });
  // Chiude il menu utente cliccando fuori o navigando.
  document.addEventListener('click', (ev) => {
    $$('.user-menu[open]').forEach((m) => { if (!m.contains(ev.target)) m.open = false; });
  });
  window.addEventListener('hashchange', () => {
    $$('.user-menu[open]').forEach((m) => { m.open = false; });
    route();
  });
  window.addEventListener('app:session-expired', () => {
    toast('info', t('auth.expired'));
    route();
  });

  restoreSession().finally(() => {
    if (window.location.pathname.startsWith('/backoffice') && !window.location.hash && hasBackofficeRole()) {
      window.location.replace('#/bo/coda');
    }
    route();
    document.documentElement.dataset.boot = bootMessage ? 'ok' : '';
  });
}
