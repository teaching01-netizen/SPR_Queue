/** SPR_Queue: signed server-to-server mail relay. Keep the secret in Script Properties. */
function doGet() {
  return relayJSON_({ok: true, service: 'SPR_Queue Mail Relay', version: 1});
}

function doPost(e) {
  try {
    return relayJSON_(receiveMail_(e));
  } catch (error) {
    // Do not expose recipient addresses, private links, request bodies or secrets.
    return relayJSON_({ok: false, code: 'INTERNAL_ERROR'});
  }
}

function receiveMail_(e) {
  var properties = PropertiesService.getScriptProperties();
  var secret = properties.getProperty('APPS_SCRIPT_SECRET') || '';
  if (!/^[a-fA-F0-9]{64}$/.test(secret)) return {ok: false, code: 'NOT_CONFIGURED'};
  var raw = e && e.postData && e.postData.contents;
  if (typeof raw !== 'string' || raw.length > 180000) return {ok: false, code: 'INVALID_REQUEST'};
  var request;
  try { request = JSON.parse(raw); } catch (error) { return {ok: false, code: 'INVALID_REQUEST'}; }
  if (!request || !Number.isSafeInteger(request.timestamp) || Math.abs(Date.now() / 1000 - request.timestamp) > 300 ||
      typeof request.payload !== 'string' || request.payload.length > 160000 ||
      typeof request.signature !== 'string' || !/^[a-f0-9]{64}$/.test(request.signature)) {
    return {ok: false, code: 'INVALID_SIGNATURE'};
  }
  var expected = Utilities.computeHmacSha256Signature(String(request.timestamp) + '\n' + request.payload, secret, Utilities.Charset.UTF_8)
    .map(function(b) { return ('0' + ((b + 256) % 256).toString(16)).slice(-2); }).join('');
  if (!sameSignature_(expected, request.signature)) return {ok: false, code: 'INVALID_SIGNATURE'};
  var message;
  try { message = JSON.parse(Utilities.newBlob(Utilities.base64Decode(request.payload)).getDataAsString('UTF-8')); }
  catch (error) { return {ok: false, code: 'INVALID_MESSAGE'}; }
  if (!validMessage_(message)) return {ok: false, code: 'INVALID_MESSAGE'};

  var lock = LockService.getScriptLock();
  if (!lock.tryLock(5000)) return {ok: false, code: 'BUSY'};
  try {
    var now = Date.now();
    pruneReceipts_(properties, now);
    var key = 'SPR_SENT_' + message.requestId;
    if (properties.getProperty(key)) return {ok: true, requestId: message.requestId, status: 'duplicate'};
    if (MailApp.getRemainingDailyQuota() < 1) return {ok: false, code: 'QUOTA_EXCEEDED'};
    try {
      MailApp.sendEmail({to: message.to, subject: message.subject, body: message.text, htmlBody: message.html, name: 'Warwick Institute'});
    } catch (error) {
      return {ok: false, code: 'SEND_FAILED'};
    }
    properties.setProperty(key, String(now));
    return {ok: true, requestId: message.requestId, status: 'sent'};
  } finally {
    lock.releaseLock();
  }
}

function validMessage_(message) {
  return message && typeof message.requestId === 'string' && /^[a-f0-9]{64}$/.test(message.requestId) &&
    typeof message.to === 'string' && message.to.length <= 254 && /^[^@\s,<>;]+@[^@\s,<>;]+\.[^@\s,<>;]+$/.test(message.to) &&
    ['Warwick Private Consult - Booking Confirmation', 'Warwick - Email Delivery Test'].indexOf(message.subject) !== -1 &&
    typeof message.html === 'string' && message.html.length > 0 && message.html.length <= 100000 &&
    typeof message.text === 'string' && message.text.length > 0 && message.text.length <= 20000;
}

function sameSignature_(a, b) {
  var difference = a.length ^ b.length;
  for (var i = 0; i < a.length; i++) difference |= a.charCodeAt(i) ^ (b.charCodeAt(i) || 0);
  return difference === 0;
}

function pruneReceipts_(properties, now) {
  var values = properties.getProperties();
  var retained = [];
  Object.keys(values).forEach(function(key) {
    if (key.indexOf('SPR_SENT_') !== 0) return;
    var at = Number(values[key]);
    if (!Number.isFinite(at) || now - at > 7 * 24 * 60 * 60 * 1000) properties.deleteProperty(key);
    else retained.push({key: key, at: at});
  });
  retained.sort(function(a, b) { return a.at - b.at; });
  for (var i = 0; i < retained.length - 1999; i++) properties.deleteProperty(retained[i].key);
}

function relayJSON_(value) {
  return ContentService.createTextOutput(JSON.stringify(value)).setMimeType(ContentService.MimeType.JSON);
}

/** Run once in the Apps Script editor to authorize sending; does not send an email. */
function checkMailSetup() {
  var secret = PropertiesService.getScriptProperties().getProperty('APPS_SCRIPT_SECRET') || '';
  if (!/^[a-fA-F0-9]{64}$/.test(secret)) throw new Error('Set APPS_SCRIPT_SECRET in Script Properties first.');
  var remaining = MailApp.getRemainingDailyQuota();
  console.log('Mail authorization ready. Remaining recipient quota: ' + remaining);
  return {ok: true, remainingQuota: remaining};
}
