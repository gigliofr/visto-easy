const SCHENGEN_COUNTRIES = new Set([
  'AT', 'BE', 'CH', 'CZ', 'DE', 'DK', 'EE', 'ES', 'FI', 'FR', 'GR', 'HR', 'HU', 'IS',
  'IT', 'LI', 'LT', 'LU', 'LV', 'MT', 'NL', 'NO', 'PL', 'PT', 'SE', 'SI', 'SK',
]);

const FIELD_SPECS = [
  { code: '1.01', section: 'Dati anagrafici richiedente', label: 'Cognome (come su passaporto)', type: 'text', required: true },
  { code: '1.02', section: 'Dati anagrafici richiedente', label: 'Nome/i (come su passaporto)', type: 'text', required: true },
  { code: '1.03', section: 'Dati anagrafici richiedente', label: 'Data di nascita', type: 'date', required: true },
  { code: '1.04', section: 'Dati anagrafici richiedente', label: 'Luogo di nascita (citta e Paese)', type: 'text', required: true },
  { code: '1.05', section: 'Dati anagrafici richiedente', label: 'Nazionalita attuale', type: 'text', required: true },
  { code: '1.06', section: 'Dati anagrafici richiedente', label: 'Altra nazionalita', type: 'text', required: false },
  { code: '1.07', section: 'Dati anagrafici richiedente', label: 'Sesso', type: 'select', required: true, options: ['M', 'F', 'X'] },
  { code: '1.08', section: 'Dati anagrafici richiedente', label: 'Stato civile', type: 'select', required: true, options: ['NUBILE_CELIBE', 'CONIUGATO', 'DIVORZIATO', 'VEDOVO'] },
  {
    code: '1.09',
    section: 'Dati anagrafici richiedente',
    label: 'Nome del coniuge',
    type: 'text',
    required: false,
    requiredIf: (v) => ['CONIUGATO', 'DIVORZIATO'].includes(String(v['1.08'] || '').toUpperCase()),
  },
  { code: '1.10', section: 'Dati anagrafici richiedente', label: 'Professione attuale', type: 'text', required: true },
  { code: '1.11', section: 'Dati anagrafici richiedente', label: 'Datore di lavoro / Istituto', type: 'text', required: true },
  { code: '1.12', section: 'Dati anagrafici richiedente', label: 'Indirizzo di residenza', type: 'textarea', required: true },
  { code: '1.13', section: 'Dati anagrafici richiedente', label: 'Email di contatto', type: 'email', required: true },

  { code: '2.01', section: 'Documento di viaggio (passaporto)', label: 'Tipo documento', type: 'select', required: true, options: ['ORDINARIO', 'SERVIZIO', 'DIPLOMATICO', 'EMERGENZA'] },
  { code: '2.02', section: 'Documento di viaggio (passaporto)', label: 'Numero documento', type: 'text', required: true },
  { code: '2.03', section: 'Documento di viaggio (passaporto)', label: 'Paese di rilascio', type: 'text', required: true },
  { code: '2.04', section: 'Documento di viaggio (passaporto)', label: 'Autorita di rilascio', type: 'text', required: true },
  { code: '2.05', section: 'Documento di viaggio (passaporto)', label: 'Data di rilascio', type: 'date', required: true },
  { code: '2.06', section: 'Documento di viaggio (passaporto)', label: 'Data di scadenza', type: 'date', required: true },
  { code: '2.07', section: 'Documento di viaggio (passaporto)', label: 'MRZ (Machine Readable Zone)', type: 'textarea', required: false },
  { code: '2.08', section: 'Documento di viaggio (passaporto)', label: 'Passaporto precedente', type: 'text', required: false },
  { code: '2.09', section: 'Documento di viaggio (passaporto)', label: 'Numero pagine disponibili', type: 'number', required: false },

  { code: '3.01', section: 'Tipo visto e informazioni soggiorno', label: 'Paese di destinazione', type: 'text', required: true },
  { code: '3.02', section: 'Tipo visto e informazioni soggiorno', label: 'Tipo di visto', type: 'select', required: true, options: ['TURISMO', 'LAVORO', 'STUDIO', 'TRANSITO', 'RICONGIUNGIMENTO', 'DIPLOMATICO', 'ALTRO'] },
  { code: '3.03', section: 'Tipo visto e informazioni soggiorno', label: 'Categoria visto', type: 'select', required: true, options: ['A', 'B', 'C', 'D'] },
  { code: '3.04', section: 'Tipo visto e informazioni soggiorno', label: 'Numero di ingressi', type: 'select', required: true, options: ['SINGOLO', 'DOPPIO', 'MULTIPLO'] },
  { code: '3.05', section: 'Tipo visto e informazioni soggiorno', label: 'Data prevista di ingresso', type: 'date', required: true },
  { code: '3.06', section: 'Tipo visto e informazioni soggiorno', label: 'Data prevista di uscita', type: 'date', required: true },
  { code: '3.07', section: 'Tipo visto e informazioni soggiorno', label: 'Durata soggiorno (giorni)', type: 'number', required: true },
  { code: '3.08', section: 'Tipo visto e informazioni soggiorno', label: 'Paese di residenza attuale', type: 'text', required: true },
  { code: '3.09', section: 'Tipo visto e informazioni soggiorno', label: 'Paese di presentazione domanda', type: 'text', required: true },
  { code: '3.10', section: 'Tipo visto e informazioni soggiorno', label: 'Motivazione del viaggio', type: 'textarea', required: true },
  {
    code: '3.11',
    section: 'Tipo visto e informazioni soggiorno',
    label: 'Itinerario dettagliato',
    type: 'textarea',
    required: false,
    requiredIf: (v) => String(v['3.03'] || '').toUpperCase() === 'C' && SCHENGEN_COUNTRIES.has(String(v['3.01'] || '').toUpperCase()),
  },

  { code: '4.01', section: 'Alloggio e contatti destinazione', label: 'Tipo di alloggio', type: 'select', required: true, options: ['HOTEL', 'ABITAZIONE_PRIVATA', 'OSTELLO', 'RESIDENZA_UNIVERSITARIA', 'ALTRO'] },
  { code: '4.02', section: 'Alloggio e contatti destinazione', label: 'Indirizzo primo alloggio', type: 'textarea', required: true },
  { code: '4.03', section: 'Alloggio e contatti destinazione', label: 'Numero prenotazione / riferimento', type: 'text', required: false },
  {
    code: '4.04',
    section: 'Alloggio e contatti destinazione',
    label: 'Nome garante/invitante',
    type: 'text',
    required: false,
    requiredIf: (v) => String(v['4.01'] || '').toUpperCase() === 'ABITAZIONE_PRIVATA',
  },
  {
    code: '4.05',
    section: 'Alloggio e contatti destinazione',
    label: 'Indirizzo garante',
    type: 'textarea',
    required: false,
    requiredIf: (v) => String(v['4.01'] || '').toUpperCase() === 'ABITAZIONE_PRIVATA',
  },
  {
    code: '4.06',
    section: 'Alloggio e contatti destinazione',
    label: 'Telefono garante',
    type: 'text',
    required: false,
    requiredIf: (v) => String(v['4.01'] || '').toUpperCase() === 'ABITAZIONE_PRIVATA',
  },
  {
    code: '4.07',
    section: 'Alloggio e contatti destinazione',
    label: 'Rapporto con il garante',
    type: 'select',
    required: false,
    requiredIf: (v) => String(v['4.01'] || '').toUpperCase() === 'ABITAZIONE_PRIVATA',
    options: ['FAMILIARE', 'AMICO', 'SOCIO_COMMERCIALE', 'ALTRO'],
  },
  { code: '4.08', section: 'Alloggio e contatti destinazione', label: 'Contatto locale di emergenza', type: 'text', required: false },

  { code: '5.01', section: 'Mezzi finanziari e sussistenza', label: 'Fonte finanziamento viaggio', type: 'select', required: true, options: ['FONDI_PROPRI', 'DATORE_LAVORO', 'GARANTE', 'BORSA_STUDIO', 'ALTRO'] },
  { code: '5.02', section: 'Mezzi finanziari e sussistenza', label: 'Disponibilita finanziaria dichiarata', type: 'text', required: true },
  {
    code: '5.03',
    section: 'Mezzi finanziari e sussistenza',
    label: 'Reddito mensile/annuale',
    type: 'text',
    required: false,
    requiredIf: (v) => String(v['5.01'] || '').toUpperCase() === 'FONDI_PROPRI',
  },
  {
    code: '5.04',
    section: 'Mezzi finanziari e sussistenza',
    label: 'Dati bancari (saldo medio)',
    type: 'text',
    required: false,
    requiredIf: (v) => String(v['5.01'] || '').toUpperCase() === 'FONDI_PROPRI',
  },
  { code: '5.05', section: 'Mezzi finanziari e sussistenza', label: 'Chi sostiene le spese', type: 'select', required: false, options: ['RICHIEDENTE', 'GARANTE', 'AZIENDA', 'ALTRO'] },
  { code: '5.06', section: 'Mezzi finanziari e sussistenza', label: 'Carta di credito internazionale', type: 'text', required: false },
  { code: '5.07', section: 'Mezzi finanziari e sussistenza', label: 'Sponsor commerciale', type: 'text', required: false },

  { code: '6.01', section: 'Storico visti e viaggi precedenti', label: 'Visti precedenti stesso Paese', type: 'textarea', required: true },
  { code: '6.02', section: 'Storico visti e viaggi precedenti', label: 'Visto mai rifiutato?', type: 'textarea', required: true },
  { code: '6.03', section: 'Storico visti e viaggi precedenti', label: 'Mai espulso/deportato?', type: 'textarea', required: true },
  { code: '6.04', section: 'Storico visti e viaggi precedenti', label: 'Paesi visitati ultimi 5 anni', type: 'textarea', required: true },
  { code: '6.05', section: 'Storico visti e viaggi precedenti', label: 'Soggiorni precedenti nel Paese', type: 'textarea', required: false },
  { code: '6.06', section: 'Storico visti e viaggi precedenti', label: 'Violazioni periodo soggiorno', type: 'textarea', required: false },
  { code: '6.07', section: 'Storico visti e viaggi precedenti', label: 'Richiesta asilo precedente', type: 'textarea', required: false },
  { code: '6.08', section: 'Storico visti e viaggi precedenti', label: 'Residenza in Paese terzo', type: 'text', required: false },
  { code: '6.09', section: 'Storico visti e viaggi precedenti', label: 'Familiari nel Paese di destinazione', type: 'text', required: false },

  { code: '7.01', section: 'Sicurezza e precedenti penali', label: 'Precedenti penali', type: 'textarea', required: true },
  { code: '7.02', section: 'Sicurezza e precedenti penali', label: 'Procedimenti penali in corso', type: 'textarea', required: true },
  { code: '7.03', section: 'Sicurezza e precedenti penali', label: 'Appartenenza a organizzazioni', type: 'textarea', required: true },
  { code: '7.04', section: 'Sicurezza e precedenti penali', label: 'Controllo biometrico (data/check)', type: 'text', required: true },
  { code: '7.05', section: 'Sicurezza e precedenti penali', label: 'Servizio militare', type: 'text', required: false },
  { code: '7.06', section: 'Sicurezza e precedenti penali', label: 'Verifica liste sanzioni internazionali', type: 'text', required: false },
  { code: '7.07', section: 'Sicurezza e precedenti penali', label: 'Note sicurezza operatore', type: 'textarea', required: false },
];

function renderField(field) {
  const id = `attr_${field.code.replace('.', '_')}`;
  const requiredMark = field.required ? '<span class="rich-req">* </span>' : '';
  const condMark = field.requiredIf ? '<span class="rich-cond">(cond.)</span>' : '';

  if (field.type === 'select') {
    const options = [
      '<option value="">Seleziona</option>',
      ...(field.options || []).map((item) => `<option value="${item}">${item.replaceAll('_', ' ')}</option>`),
    ].join('');
    return `
      <label class="rich-attr-field" for="${id}">
        ${requiredMark}${field.code} - ${field.label} ${condMark}
        <select id="${id}" name="${id}" data-attr-code="${field.code}">${options}</select>
      </label>
    `;
  }

  if (field.type === 'textarea') {
    return `
      <label class="rich-attr-field" for="${id}">
        ${requiredMark}${field.code} - ${field.label} ${condMark}
        <textarea id="${id}" name="${id}" data-attr-code="${field.code}" rows="3"></textarea>
      </label>
    `;
  }

  const inputType = field.type === 'email' || field.type === 'date' || field.type === 'number' ? field.type : 'text';
  return `
    <label class="rich-attr-field" for="${id}">
      ${requiredMark}${field.code} - ${field.label} ${condMark}
      <input id="${id}" name="${id}" data-attr-code="${field.code}" type="${inputType}">
    </label>
  `;
}

function sectionOrder(sectionName) {
  const map = {
    'Dati anagrafici richiedente': 1,
    'Documento di viaggio (passaporto)': 2,
    'Tipo visto e informazioni soggiorno': 3,
    'Alloggio e contatti destinazione': 4,
    'Mezzi finanziari e sussistenza': 5,
    'Storico visti e viaggi precedenti': 6,
    'Sicurezza e precedenti penali': 7,
  };
  return map[sectionName] || 99;
}

export function initRichAttributesComposer() {
  const container = document.getElementById('richAttributesComposer');
  if (!container) return;

  const grouped = FIELD_SPECS.reduce((acc, field) => {
    const key = field.section;
    if (!acc[key]) acc[key] = [];
    acc[key].push(field);
    return acc;
  }, {});

  const sections = Object.keys(grouped).sort((a, b) => sectionOrder(a) - sectionOrder(b));
  container.innerHTML = `
    <div class="rich-attributes-head">
      <h4>Compilazione attributi pratica visto</h4>
      <p class="helper-text">Campi obbligatori segnati con *; i condizionali si attivano in base alle risposte.</p>
    </div>
    ${sections.map((section, idx) => `
      <details class="rich-attr-section" ${idx < 2 ? 'open' : ''}>
        <summary>${section}</summary>
        <div class="rich-attr-grid">
          ${grouped[section].map((field) => renderField(field)).join('')}
        </div>
      </details>
    `).join('')}
  `;

  const formTipoVisto = document.querySelector('#formCreatePratica select[name="tipo_visto"]');
  const formPaeseDest = document.querySelector('#formCreatePratica select[name="paese_dest"]');
  const attrTipoVisto = document.getElementById('attr_3_02');
  const attrPaeseDest = document.getElementById('attr_3_01');

  if (formTipoVisto && attrTipoVisto) {
    if (!attrTipoVisto.value && formTipoVisto.value) attrTipoVisto.value = formTipoVisto.value;
    formTipoVisto.addEventListener('change', () => {
      if (!attrTipoVisto.value || attrTipoVisto.value !== formTipoVisto.value) {
        attrTipoVisto.value = formTipoVisto.value;
      }
    });
    attrTipoVisto.addEventListener('change', () => {
      if (formTipoVisto.value !== attrTipoVisto.value) {
        formTipoVisto.value = attrTipoVisto.value;
      }
    });
  }

  if (formPaeseDest && attrPaeseDest) {
    if (!attrPaeseDest.value && formPaeseDest.value) attrPaeseDest.value = formPaeseDest.value;
    formPaeseDest.addEventListener('change', () => {
      if (!attrPaeseDest.value || attrPaeseDest.value !== formPaeseDest.value) {
        attrPaeseDest.value = formPaeseDest.value;
      }
    });
    attrPaeseDest.addEventListener('change', () => {
      if (formPaeseDest.value !== attrPaeseDest.value) {
        formPaeseDest.value = attrPaeseDest.value;
      }
    });
  }
}

function readValuesFromDOM() {
  const values = {};
  document.querySelectorAll('[data-attr-code]').forEach((el) => {
    values[el.dataset.attrCode] = String(el.value || '').trim();
  });
  return values;
}

export function collectRichAttributes() {
  const values = readValuesFromDOM();
  const passaporto = {};
  const sections = {};
  Object.entries(values).forEach(([code, value]) => {
    if (code.startsWith('2.')) {
      passaporto[code] = value;
    } else {
      sections[code] = value;
    }
  });
  return {
    values,
    sections,
    passaporto,
  };
}

export function validateRichAttributes() {
  const values = readValuesFromDOM();
  const errors = [];

  FIELD_SPECS.forEach((field) => {
    const value = String(values[field.code] || '').trim();
    const isRequired = Boolean(field.required) || (typeof field.requiredIf === 'function' && field.requiredIf(values));
    if (isRequired && !value) {
      errors.push(`${field.code} ${field.label}`);
    }
  });

  const schengenDest = String(values['3.01'] || '').toUpperCase();
  const isSchengen = SCHENGEN_COUNTRIES.has(schengenDest);
  if (isSchengen) {
    ['3.11', '5.02', '6.04'].forEach((code) => {
      if (!String(values[code] || '').trim()) {
        const spec = FIELD_SPECS.find((f) => f.code === code);
        errors.push(`${code} ${spec?.label || ''}`.trim());
      }
    });
  }

  return {
    ok: errors.length === 0,
    errors,
    values,
  };
}
