const LANG_KEY = 'visto_easy_lang_v1';
const SUPPORTED_LANGS = new Set(['it', 'en']);

const DICT = {
  it: {
    'hero.modeTitle': 'Piattaforma visti',
    'hero.title': 'Un flusso chiaro per ogni pratica.',
    'hero.subtitle': 'Richiedenti e backoffice nello stesso spazio, con stato pratica, documenti e operazioni sempre leggibili.',
    'hero.kicker': 'Un\'interfaccia piu sobria, piu leggibile e piu vicina al lavoro operativo reale.',
    'hero.settings': 'Impostazioni',
    'hero.logout': 'Esci',
    'lang.switchLabel': 'Seleziona lingua',
    'lang.it': 'Italiano',
    'lang.en': 'English',
    'auth.title': 'Autenticazione',
    'auth.subtitle': 'Accedi con email e password.',
    'auth.tab.login': 'Login',
    'auth.tab.register': 'Crea account',
    'auth.tab.recovery': 'Recupero',
    'auth.label.nome': 'Nome',
    'auth.label.cognome': 'Cognome',
    'auth.label.email': 'Email',
    'auth.label.password': 'Password',
    'auth.label.passwordConfirm': 'Conferma password',
    'auth.label.token': 'Token',
    'auth.label.newPassword': 'Nuova password',
    'auth.btn.login': 'Accedi',
    'auth.btn.createAccount': 'Conferma creazione account',
    'auth.btn.backToLogin': 'Torna al login',
    'auth.btn.forgotPassword': 'Recupero password',
    'auth.btn.sendReset': 'Invia reset',
    'auth.btn.confirmReset': 'Conferma reset password',
    'auth.passwordRules': 'Usa almeno 10 caratteri con maiuscole, minuscole, numeri e simboli.',
    'auth.passwordStrength': 'Forza password: {level}',
    'auth.passwordToggleShow': 'Mostra',
    'auth.passwordToggleHide': 'Nascondi',
    'auth.passwordSuggestion.length': 'Almeno 10 caratteri.',
    'auth.passwordSuggestion.upper': 'Una o piu maiuscole.',
    'auth.passwordSuggestion.lower': 'Una o piu minuscole.',
    'auth.passwordSuggestion.number': 'Uno o piu numeri.',
    'auth.passwordSuggestion.special': 'Uno o piu caratteri speciali.',
    'auth.password.level.weak': 'Scarso',
    'auth.password.level.medium': 'Medio',
    'auth.password.level.strong': 'Eccellente',
    'auth.passwordMismatch': 'Le password non coincidono.',
    'auth.err.passwordMismatch': 'Le password non coincidono',
    'auth.err.passwordWeak': 'Password troppo debole: usa almeno 10 caratteri con maiuscole, minuscole, numeri e simboli',
    'auth.ok.registerPending': 'Registrazione completata. Controlla la tua email per attivare l\'account.',
    'auth.ok.registerDone': 'Registrazione completata',
    'auth.ok.loginDone': 'Login completato',
    'auth.ok.forgotSent': 'Richiesta reset inviata',
    'auth.ok.passwordUpdated': 'Password aggiornata',
    'auth.pending.title': 'Controlla la tua casella email',
    'auth.pending.subtitle': 'Abbiamo inviato un link di attivazione a {email}. Apri la mail e conferma l\'account per completare la registrazione.',
    'auth.pending.step1': 'Apri la tua casella di posta (controlla anche spam/promozioni).',
    'auth.pending.step2': 'Clicca il pulsante di verifica presente nella mail.',
    'auth.pending.step3': 'Dopo la conferma, torna qui ed effettua il login.',
    'auth.pending.openGmail': 'Apri Gmail',
    'auth.pending.openOutlook': 'Apri Outlook',
    'auth.pending.backLogin': 'Ho verificato, vai al login',
    'auth.pending.changeEmail': 'Usa un\'altra email',
    'ops.err.richOnly': 'Azione riservata al ruolo RICHIEDENTE',
    'ops.err.backofficeOnly': 'Azione riservata al backoffice',
    'ops.err.backofficeUserMissing': 'Utente backoffice non disponibile',
    'ops.err.internalNoteEmpty': 'Inserisci un testo per la nota interna.',
    'ops.err.selectPracticeFirst': 'Seleziona prima una pratica',
    'ops.err.backofficeAccessDenied': 'Il tuo account non ha accesso al backoffice.',
    'ops.err.refreshTokenMissing': 'Refresh token mancante',
    'ops.ok.practiceSubmitted': 'Pratica inviata',
    'ops.ok.practiceDeleted': 'Pratica eliminata',
    'ops.ok.practiceDocTarget': 'Pratica selezionata per upload documenti: {id}',
    'ops.ok.practiceAssignedToMe': 'Pratica assegnata a te',
    'ops.ok.internalNoteAdded': 'Nota interna aggiunta',
    'ops.ok.practiceAssigned': 'Pratica assegnata',
    'ops.ok.twofaSetupDone': '2FA setup completato',
    'ops.ok.twofaEnabled': '2FA abilitato',
    'ops.ok.twofaDisabled': '2FA disabilitato',
    'ops.ok.practiceCreated': 'Pratica creata',
    'ops.ok.documentRegistered': 'Documento registrato',
    'ops.ok.filtersApplied': 'Filtri applicati',
    'ops.ok.usersUpdated': 'Utenti aggiornati',
    'ops.ok.operatorInviteSent': 'Invito operatore inviato',
    'ops.ok.auditUpdated': 'Audit aggiornato',
    'ops.ok.practiceStateUpdated': 'Stato pratica aggiornato',
    'ops.ok.noteAdded': 'Nota aggiunta',
    'ops.ok.operatorAssigned': 'Operatore assegnato',
    'ops.ok.documentRequestSent': 'Richiesta documento inviata',
    'ops.ok.paymentLinkCreated': 'Link pagamento creato',
    'ops.ok.apiBaseSaved': 'API base salvata: {apiBase}',
    'ops.ok.apiBaseReset': 'API base reimpostata su locale',
    'ops.ok.localSessionClosed': 'Sessione locale chiusa',
    'ops.ok.logoutDone': 'Logout completato',
    'ops.ok.sessionRefreshed': 'Sessione aggiornata',
    'footer.privacy': 'Privacy Policy',
    'footer.cookie': 'Cookie Policy',
    'session.role': 'Ruolo: {role}',
    'session.user': 'Utente: {email}',
    'session.guest': 'ospite',
    'session.notAuth': 'non autenticato',
    'generic.unknownError': 'errore sconosciuto',
    'generic.errorPrefix': 'errore {status}',
    'generic.loading': 'Attendere...',
    'generic.emptyList': 'Nessun elemento',
  },
  en: {
    'hero.modeTitle': 'Visa platform',
    'hero.title': 'A clear workflow for every case.',
    'hero.subtitle': 'Applicants and backoffice in one place, with status, documents, and operations always easy to read.',
    'hero.kicker': 'A cleaner interface, easier to read, closer to real operational work.',
    'hero.settings': 'Settings',
    'hero.logout': 'Log out',
    'lang.switchLabel': 'Select language',
    'lang.it': 'Italian',
    'lang.en': 'English',
    'auth.title': 'Authentication',
    'auth.subtitle': 'Sign in with email and password.',
    'auth.tab.login': 'Login',
    'auth.tab.register': 'Create account',
    'auth.tab.recovery': 'Recovery',
    'auth.label.nome': 'First name',
    'auth.label.cognome': 'Last name',
    'auth.label.email': 'Email',
    'auth.label.password': 'Password',
    'auth.label.passwordConfirm': 'Confirm password',
    'auth.label.token': 'Token',
    'auth.label.newPassword': 'New password',
    'auth.btn.login': 'Sign in',
    'auth.btn.createAccount': 'Create account',
    'auth.btn.backToLogin': 'Back to login',
    'auth.btn.forgotPassword': 'Forgot password',
    'auth.btn.sendReset': 'Send reset',
    'auth.btn.confirmReset': 'Confirm password reset',
    'auth.passwordRules': 'Use at least 10 characters with uppercase, lowercase, numbers, and symbols.',
    'auth.passwordStrength': 'Password strength: {level}',
    'auth.passwordToggleShow': 'Show',
    'auth.passwordToggleHide': 'Hide',
    'auth.passwordSuggestion.length': 'At least 10 characters.',
    'auth.passwordSuggestion.upper': 'One or more uppercase letters.',
    'auth.passwordSuggestion.lower': 'One or more lowercase letters.',
    'auth.passwordSuggestion.number': 'One or more numbers.',
    'auth.passwordSuggestion.special': 'One or more special characters.',
    'auth.password.level.weak': 'Weak',
    'auth.password.level.medium': 'Medium',
    'auth.password.level.strong': 'Excellent',
    'auth.passwordMismatch': 'Passwords do not match.',
    'auth.err.passwordMismatch': 'Passwords do not match',
    'auth.err.passwordWeak': 'Password too weak: use at least 10 characters with uppercase, lowercase, numbers, and symbols',
    'auth.ok.registerPending': 'Registration completed. Check your email to activate your account.',
    'auth.ok.registerDone': 'Registration completed',
    'auth.ok.loginDone': 'Login completed',
    'auth.ok.forgotSent': 'Password reset request sent',
    'auth.ok.passwordUpdated': 'Password updated',
    'auth.pending.title': 'Check your email inbox',
    'auth.pending.subtitle': 'We sent an activation link to {email}. Open the email and confirm your account to complete registration.',
    'auth.pending.step1': 'Open your mailbox (also check spam/promotions).',
    'auth.pending.step2': 'Click the verification button inside the email.',
    'auth.pending.step3': 'After confirmation, come back here and sign in.',
    'auth.pending.openGmail': 'Open Gmail',
    'auth.pending.openOutlook': 'Open Outlook',
    'auth.pending.backLogin': 'I verified, go to login',
    'auth.pending.changeEmail': 'Use another email',
    'ops.err.richOnly': 'Action allowed only for APPLICANT role',
    'ops.err.backofficeOnly': 'Action allowed only for backoffice role',
    'ops.err.backofficeUserMissing': 'Backoffice user is not available',
    'ops.err.internalNoteEmpty': 'Enter text for the internal note.',
    'ops.err.selectPracticeFirst': 'Select a case first',
    'ops.err.backofficeAccessDenied': 'Your account does not have backoffice access.',
    'ops.err.refreshTokenMissing': 'Missing refresh token',
    'ops.ok.practiceSubmitted': 'Case submitted',
    'ops.ok.practiceDeleted': 'Case deleted',
    'ops.ok.practiceDocTarget': 'Case selected for document upload: {id}',
    'ops.ok.practiceAssignedToMe': 'Case assigned to you',
    'ops.ok.internalNoteAdded': 'Internal note added',
    'ops.ok.practiceAssigned': 'Case assigned',
    'ops.ok.twofaSetupDone': '2FA setup completed',
    'ops.ok.twofaEnabled': '2FA enabled',
    'ops.ok.twofaDisabled': '2FA disabled',
    'ops.ok.practiceCreated': 'Case created',
    'ops.ok.documentRegistered': 'Document registered',
    'ops.ok.filtersApplied': 'Filters applied',
    'ops.ok.usersUpdated': 'Users updated',
    'ops.ok.operatorInviteSent': 'Operator invite sent',
    'ops.ok.auditUpdated': 'Audit updated',
    'ops.ok.practiceStateUpdated': 'Case status updated',
    'ops.ok.noteAdded': 'Note added',
    'ops.ok.operatorAssigned': 'Operator assigned',
    'ops.ok.documentRequestSent': 'Document request sent',
    'ops.ok.paymentLinkCreated': 'Payment link created',
    'ops.ok.apiBaseSaved': 'API base saved: {apiBase}',
    'ops.ok.apiBaseReset': 'API base reset to local',
    'ops.ok.localSessionClosed': 'Local session closed',
    'ops.ok.logoutDone': 'Logout completed',
    'ops.ok.sessionRefreshed': 'Session refreshed',
    'footer.privacy': 'Privacy Policy',
    'footer.cookie': 'Cookie Policy',
    'session.role': 'Role: {role}',
    'session.user': 'User: {email}',
    'session.guest': 'guest',
    'session.notAuth': 'not authenticated',
    'generic.unknownError': 'unknown error',
    'generic.errorPrefix': 'error {status}',
    'generic.loading': 'Please wait...',
    'generic.emptyList': 'No items',
  },
};

function normalizeLang(value) {
  const raw = String(value || '').toLowerCase();
  if (SUPPORTED_LANGS.has(raw)) return raw;
  return 'it';
}

export function getCurrentLang() {
  const stored = normalizeLang(window.localStorage.getItem(LANG_KEY));
  if (stored) return stored;
  const browser = normalizeLang(navigator.language?.slice(0, 2));
  return browser;
}

export function t(key, params = {}) {
  const lang = normalizeLang(window.document.documentElement.lang || getCurrentLang());
  const fallback = DICT.it[key] || key;
  const template = DICT[lang]?.[key] || fallback;
  return String(template).replace(/\{([a-zA-Z0-9_]+)\}/g, (_full, token) => {
    if (Object.prototype.hasOwnProperty.call(params, token)) return String(params[token]);
    return '';
  });
}

export function applyTranslations(root = document) {
  root.querySelectorAll('[data-i18n]').forEach((el) => {
    const key = el.getAttribute('data-i18n');
    if (!key) return;
    el.textContent = t(key);
  });

  root.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
    const key = el.getAttribute('data-i18n-placeholder');
    if (!key) return;
    el.setAttribute('placeholder', t(key));
  });

  root.querySelectorAll('[data-i18n-aria-label]').forEach((el) => {
    const key = el.getAttribute('data-i18n-aria-label');
    if (!key) return;
    el.setAttribute('aria-label', t(key));
  });
}

export function setCurrentLang(lang) {
  const next = normalizeLang(lang);
  window.localStorage.setItem(LANG_KEY, next);
  document.documentElement.lang = next;
  applyTranslations(document);
  window.dispatchEvent(new CustomEvent('app:lang-change', { detail: { lang: next } }));
}

function wireLanguageButtons() {
  const buttons = document.querySelectorAll('[data-lang-choice]');
  if (!buttons.length) return;

  const sync = () => {
    const lang = getCurrentLang();
    buttons.forEach((btn) => {
      const isActive = btn.getAttribute('data-lang-choice') === lang;
      btn.classList.toggle('active', isActive);
      btn.setAttribute('aria-pressed', isActive ? 'true' : 'false');
    });
  };

  buttons.forEach((btn) => {
    btn.addEventListener('click', () => {
      setCurrentLang(btn.getAttribute('data-lang-choice'));
      sync();
    });
  });

  sync();
}

export function initI18n() {
  const lang = getCurrentLang();
  document.documentElement.lang = lang;
  applyTranslations(document);
  wireLanguageButtons();
}
