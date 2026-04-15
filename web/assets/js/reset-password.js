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

  const pickToken = (value) => {
    const raw = String(value || '').trim();
    if (!raw) return '';
    const cleaned = raw.replace(/^token/i, '').replace(/^[:=\s]+/, '').trim();
    if (!cleaned) return '';
    const m = cleaned.match(/^([A-Za-z0-9._~\-]{12,})/);
    if (m && m[1]) return m[1];
    return '';
  };

  const safeDecode = (value) => {
    try {
      return decodeURIComponent(String(value || ''));
    } catch (_err) {
      return String(value || '');
    }
  };

  const extractTokenFromURL = () => {
    const params = new URLSearchParams(window.location.search);
    const direct = pickToken(params.get('token'));
    if (direct) return direct;

    for (const [key, value] of params.entries()) {
      const v = pickToken(value);
      if (String(key || '').toLowerCase() === 'token' && v) return v;

      const keyRaw = String(key || '').trim();
      if (/^token/i.test(keyRaw)) {
        const keyToken = pickToken(keyRaw);
        if (keyToken) return keyToken;
      }

      const merged = pickToken(`${keyRaw}${value || ''}`);
      if (merged) return merged;
    }

    const pathMatch = String(window.location.pathname || '').match(/\/reset-password\/([^/?#]+)/i);
    if (pathMatch && pathMatch[1]) return pathMatch[1];

    const rawQuery = String(window.location.search || '').replace(/^\?/, '').trim();
    const rawHash = String(window.location.hash || '').replace(/^#/, '').trim();
    const fullHref = safeDecode(String(window.location.href || ''));

    const tokenLikeQuery = safeDecode(rawQuery).match(/(?:^|[?&#])token(?:=|%3D)?([^&#\s]+)/i);
    if (tokenLikeQuery && tokenLikeQuery[1]) return pickToken(tokenLikeQuery[1]);

    const tokenLikeHash = safeDecode(rawHash).match(/(?:^|[?&#])token(?:=|%3D)?([^&#\s]+)/i);
    if (tokenLikeHash && tokenLikeHash[1]) return pickToken(tokenLikeHash[1]);

    const tokenFromHref = fullHref.match(/token(?:=|%3D)?([^&#\s]+)/i);
    if (tokenFromHref && tokenFromHref[1]) return pickToken(tokenFromHref[1]);

    const genericQuery = pickToken(safeDecode(rawQuery));
    if (genericQuery) return genericQuery;

    const genericHash = pickToken(safeDecode(rawHash));
    if (genericHash) return genericHash;

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
