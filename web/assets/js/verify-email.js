(() => {
  const LANG_KEY = 'visto_easy_lang_v1';

  const STRINGS = {
    it: {
      title: 'Registrazione completata con successo.',
      subtitle: 'Stiamo verificando il tuo indirizzo email per attivare il tuo profilo e permetterti di usare tutti i servizi.',
      checking: 'Verifica in corso...',
      noToken: 'Link non valido: token mancante. Apri il link ricevuto via email.',
      failed: 'Verifica non riuscita',
      success: 'Email verificata. Il tuo account e ora attivo.',
      login: 'Vai al login',
      home: 'Torna alla home',
    },
    en: {
      title: 'Registration completed successfully.',
      subtitle: 'We are verifying your email address to activate your profile and unlock all services.',
      checking: 'Verification in progress...',
      noToken: 'Invalid link: missing token. Open the link received by email.',
      failed: 'Verification failed',
      success: 'Email verified. Your account is now active.',
      login: 'Go to login',
      home: 'Back to home',
    },
  };

  const normalizeLang = (lang) => (String(lang || '').toLowerCase() === 'en' ? 'en' : 'it');
  const getLang = () => normalizeLang(window.localStorage.getItem(LANG_KEY) || document.documentElement.lang || navigator.language?.slice(0, 2));
  let lang = getLang();

  const titleText = document.getElementById('titleText');
  const subtitleText = document.getElementById('subtitleText');
  const resultBox = document.getElementById('resultBox');
  const btnLogin = document.getElementById('btnLogin');
  const btnHome = document.getElementById('btnHome');

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

  const applyLang = () => {
    document.documentElement.lang = lang;
    titleText.textContent = t('title');
    subtitleText.textContent = t('subtitle');
    btnLogin.textContent = t('login');
    btnHome.textContent = t('home');

    if (!resultBox.dataset.state || resultBox.dataset.state === 'checking') {
      resultBox.textContent = t('checking');
    }

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

  applyLang();

  const token = extractTokenFromURL();
  if (!token) {
    resultBox.className = 'status err';
    resultBox.dataset.state = 'error';
    resultBox.textContent = t('noToken');
    return;
  }

  fetch('/api/auth/verify-email', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  })
    .then(async (response) => {
      const data = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error((data && (data.error || data.message)) || t('failed'));
      }

      resultBox.className = 'status ok';
      resultBox.dataset.state = 'success';
      resultBox.textContent = t('success');
    })
    .catch((err) => {
      resultBox.className = 'status err';
      resultBox.dataset.state = 'error';
      resultBox.textContent = err.message || t('failed');
    });
})();
