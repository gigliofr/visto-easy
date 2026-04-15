(() => {
  const LANG_KEY = 'visto_easy_lang_v1';

  const STRINGS = {
    it: {
      title: 'Imposta una nuova password.',
      subtitle: 'Usa il link ricevuto via email per completare il reset o l\'attivazione dell\'account.',
      prompt: 'Inserisci la nuova password per continuare.',
      noToken: 'Link non valido: token mancante. Apri il link ricevuto via email.',
      tooShort: 'La password deve avere almeno 8 caratteri.',
      failed: 'Reset non riuscito',
      success: 'Password aggiornata con successo. Puoi accedere con le nuove credenziali.',
      passwordLabel: 'Nuova password',
      passwordToggleShow: 'Mostra',
      passwordToggleHide: 'Nascondi',
      submit: 'Conferma nuova password',
      home: 'Torna alla home',
      login: 'Vai al login',
    },
    en: {
      title: 'Set a new password.',
      subtitle: 'Use the link received by email to complete password reset or account activation.',
      prompt: 'Enter your new password to continue.',
      noToken: 'Invalid link: missing token. Open the link received by email.',
      tooShort: 'Password must be at least 8 characters long.',
      failed: 'Reset failed',
      success: 'Password updated successfully. You can now sign in with your new credentials.',
      passwordLabel: 'New password',
      passwordToggleShow: 'Show',
      passwordToggleHide: 'Hide',
      submit: 'Confirm new password',
      home: 'Back to home',
      login: 'Go to login',
    },
  };

  const normalizeLang = (lang) => (String(lang || '').toLowerCase() === 'en' ? 'en' : 'it');
  const getLang = () => normalizeLang(window.localStorage.getItem(LANG_KEY) || document.documentElement.lang || navigator.language?.slice(0, 2));
  let lang = getLang();

  const titleText = document.getElementById('titleText');
  const subtitleText = document.getElementById('subtitleText');
  const passwordLabel = document.getElementById('passwordLabel');
  const btnSubmit = document.getElementById('btnSubmit');
  const passwordInput = document.getElementById('passwordInput');
  const passwordToggle = document.getElementById('passwordToggle');
  const btnHome = document.getElementById('btnHome');
  const btnLogin = document.getElementById('btnLogin');
  const resultBox = document.getElementById('resultBox');
  const form = document.getElementById('resetForm');

  const t = (key) => STRINGS[lang][key] || STRINGS.it[key] || key;

  const extractTokenFromURL = () => {
    const params = new URLSearchParams(window.location.search);
    const direct = String(params.get('token') || '').trim();
    if (direct) return direct;

    for (const [key, value] of params.entries()) {
      const v = String(value || '').trim();
      if (key === 'token' && v) return v;

      const keyToken = String(key || '').trim();
      const m = keyToken.match(/^token=?([A-Fa-f0-9]{24,})$/i);
      if (m && m[1]) return m[1];
    }

    const rawQuery = String(window.location.search || '').replace(/^\?/, '').trim();
    const queryMatch = rawQuery.match(/(?:^|[?&])token=?([A-Fa-f0-9]{24,})(?:&|$)/i);
    if (queryMatch && queryMatch[1]) return queryMatch[1];

    const rawHash = String(window.location.hash || '').replace(/^#/, '').trim();
    const hashMatch = rawHash.match(/(?:^|[?&])token=?([A-Fa-f0-9]{24,})(?:&|$)/i);
    if (hashMatch && hashMatch[1]) return hashMatch[1];

    return '';
  };

  const syncPasswordToggle = () => {
    const isVisible = passwordInput.type === 'text';
    const label = isVisible ? t('passwordToggleHide') : t('passwordToggleShow');
    passwordToggle.textContent = label;
    passwordToggle.setAttribute('aria-label', label);
    passwordToggle.setAttribute('aria-pressed', isVisible ? 'true' : 'false');
  };

  const applyLang = () => {
    document.documentElement.lang = lang;
    titleText.textContent = t('title');
    subtitleText.textContent = t('subtitle');
    passwordLabel.textContent = t('passwordLabel');
    btnSubmit.textContent = t('submit');
    btnHome.textContent = t('home');
    btnLogin.textContent = t('login');

    if (!resultBox.dataset.state || resultBox.dataset.state === 'prompt') {
      resultBox.textContent = t('prompt');
    }

    syncPasswordToggle();

    document.querySelectorAll('[data-lang]').forEach((btn) => {
      const active = btn.getAttribute('data-lang') === lang;
      btn.setAttribute('aria-pressed', active ? 'true' : 'false');
    });
  };

  document.querySelectorAll('[data-lang]').forEach((btn) => {
    btn.addEventListener('click', () => {
      lang = normalizeLang(btn.getAttribute('data-lang'));
      window.localStorage.setItem(LANG_KEY, lang);
      applyLang();
    });
  });

  passwordToggle.addEventListener('click', () => {
    passwordInput.type = passwordInput.type === 'password' ? 'text' : 'password';
    syncPasswordToggle();
  });

  applyLang();

  const token = extractTokenFromURL();
  if (!token) {
    resultBox.className = 'status err';
    resultBox.dataset.state = 'error';
    resultBox.textContent = t('noToken');
    btnSubmit.disabled = true;
    return;
  }

  form.addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const newPassword = passwordInput.value;
    if (newPassword.length < 8) {
      resultBox.className = 'status err';
      resultBox.dataset.state = 'error';
      resultBox.textContent = t('tooShort');
      return;
    }

    try {
      const response = await fetch('/api/auth/reset-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, new_password: newPassword }),
      });
      const data = await response.json().catch(() => null);
      if (!response.ok) throw new Error((data && (data.error || data.message)) || t('failed'));

      resultBox.className = 'status ok';
      resultBox.dataset.state = 'success';
      resultBox.textContent = t('success');
      form.reset();
      passwordInput.type = 'password';
      syncPasswordToggle();
    } catch (err) {
      resultBox.className = 'status err';
      resultBox.dataset.state = 'error';
      resultBox.textContent = err.message || t('failed');
    }
  });
})();
