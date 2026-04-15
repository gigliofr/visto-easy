(() => {
  const LANG_KEY = 'visto_easy_lang_v1';

  const STRINGS = {
    it: {
      title: 'Registrazione completata con successo.',
      subtitle: 'Stiamo verificando il tuo indirizzo email per attivare il tuo profilo e permetterti di usare tutti i servizi.',
      checking: 'Premi "Conferma email" per completare la verifica dell\'account.',
      confirming: 'Verifica in corso...',
      noToken: 'Link non valido: token mancante. Apri il link ricevuto via email.',
      failed: 'Verifica non riuscita',
      success: 'Email verificata. Il tuo account e ora attivo.',
      verifyNow: 'Conferma email',
      login: 'Vai al login',
      home: 'Torna alla home',
    },
    en: {
      title: 'Registration completed successfully.',
      subtitle: 'We are verifying your email address to activate your profile and unlock all services.',
      checking: 'Press "Confirm email" to complete account verification.',
      confirming: 'Verification in progress...',
      noToken: 'Invalid link: missing token. Open the link received by email.',
      failed: 'Verification failed',
      success: 'Email verified. Your account is now active.',
      verifyNow: 'Confirm email',
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
  const btnVerifyNow = document.getElementById('btnVerifyNow');
  const btnLogin = document.getElementById('btnLogin');
  const btnHome = document.getElementById('btnHome');

  const t = (key) => STRINGS[lang][key] || STRINGS.it[key] || key;

  const pickToken = (value) => {
    const raw = String(value || '').trim();
    if (!raw) return '';
    const cleaned = raw.replace(/^token/i, '').replace(/^[:=\s]+/, '').trim();
    if (!cleaned) return '';
    const m = cleaned.match(/([A-Za-z0-9._~\-]{12,})/);
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

    const pathMatch = String(window.location.pathname || '').match(/\/verify-email\/([^/?#]+)/i);
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

  const applyLang = () => {
    document.documentElement.lang = lang;
    titleText.textContent = t('title');
    subtitleText.textContent = t('subtitle');
    btnVerifyNow.textContent = t('verifyNow');
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
    btnVerifyNow.disabled = true;
    return;
  }

  btnVerifyNow.addEventListener('click', async () => {
    if (btnVerifyNow.disabled) return;
    btnVerifyNow.disabled = true;
    resultBox.className = 'status';
    resultBox.dataset.state = 'checking';
    resultBox.textContent = t('confirming');

    try {
      const response = await fetch('/api/auth/verify-email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
      });
      const data = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error((data && (data.error || data.message)) || t('failed'));
      }

      resultBox.className = 'status ok';
      resultBox.dataset.state = 'success';
      resultBox.textContent = t('success');
    } catch (err) {
      resultBox.className = 'status err';
      resultBox.dataset.state = 'error';
      resultBox.textContent = err.message || t('failed');
      btnVerifyNow.disabled = false;
    }
  });
})();
