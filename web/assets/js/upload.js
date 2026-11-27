import { api } from './http.js';
import { t } from './i18n.js';

const ALLOWED = ['application/pdf', 'image/jpeg', 'image/jpg', 'image/png', 'image/heic'];
export const MAX_BYTES = 10 * 1024 * 1024;

export function checkFile(file) {
  if (!file) return t('upload.noFile');
  const mime = (file.type || '').toLowerCase();
  if (!ALLOWED.includes(mime)) return t('upload.badType');
  if (file.size <= 0 || file.size > MAX_BYTES) return t('upload.tooBig');
  return '';
}

// uploadDocumento carica il file sullo storage con un URL firmato e poi
// registra il documento sulla pratica. Se lo storage non è configurato
// (sviluppo locale) registra solo i metadati e lo segnala.
export async function uploadDocumento(praticaID, tipo, file) {
  const problem = checkFile(file);
  if (problem) throw { message: problem };
  const meta = { nome_file: file.name, mime_type: (file.type || '').toLowerCase(), dimensione: file.size };
  let s3Key = '';
  let stored = false;
  try {
    const res = await api(`/api/pratiche/${encodeURIComponent(praticaID)}/documenti/presign`, { method: 'POST', json: meta });
    const upload = res?.upload || {};
    if (upload.upload_url) {
      const put = await fetch(upload.upload_url, { method: 'PUT', body: file, headers: { 'Content-Type': meta.mime_type } });
      if (!put.ok) throw { message: t('upload.putFailed') };
      s3Key = upload.key || '';
      stored = true;
    }
  } catch (err) {
    if (err?.status !== 503) throw err;
  }
  const doc = await api(`/api/pratiche/${encodeURIComponent(praticaID)}/documenti`, {
    method: 'POST',
    json: { ...meta, tipo, s3_key: s3Key },
  });
  return { doc: doc?.documento || doc, stored };
}

export async function openDocumento(praticaID, docID, mode = 'download') {
  const res = await api(`/api/pratiche/${encodeURIComponent(praticaID)}/documenti/${encodeURIComponent(docID)}/${mode}?format=json`);
  if (res?.url) window.open(res.url, '_blank', 'noopener');
}
