/** SPR_Queue Sheets receiver. Deploy separately from the email script. */
var SPR_SPREADSHEET_ID = '1DUxn8Br2TDwQze-RmxcFBJFMuA1gkQi5qGs8amDCsF8';
var SPR_SHEET_ID = 1971459091;
var SPR_COLUMNS = 39;

function doGet() {
  return sheetJSON_({ok:true,service:'SPR_Queue Sheets Sync',version:1});
}

function doPost(e) {
  try {
    var result=receiveSnapshot_(e);
    if(!result.ok)console.warn('SPR_SHEETS_SYNC '+result.code);
    return sheetJSON_(result);
  }
  catch (error) { console.warn('SPR_SHEETS_SYNC SYNC_FAILED'); return sheetJSON_({ok:false,code:'SYNC_FAILED'}); }
}

function receiveSnapshot_(e) {
  var props=PropertiesService.getScriptProperties();
  var secret=props.getProperty('APPS_SCRIPT_SECRET')||'';
  if(!/^[a-fA-F0-9]{64}$/.test(secret))return {ok:false,code:'NOT_CONFIGURED'};
  var raw=e&&e.postData&&e.postData.contents,request;
  if(typeof raw!=='string'||raw.length>2800000)return {ok:false,code:'INVALID_REQUEST'};
  try {request=JSON.parse(raw);}catch(error){return {ok:false,code:'INVALID_REQUEST'};}
  if(!request||!Number.isSafeInteger(request.timestamp)||Math.abs(Date.now()/1000-request.timestamp)>300||typeof request.payload!=='string'||request.payload.length>2700000||typeof request.signature!=='string'||!/^[a-f0-9]{64}$/.test(request.signature))return {ok:false,code:'INVALID_SIGNATURE'};
  var expected=Utilities.computeHmacSha256Signature(String(request.timestamp)+'\n'+request.payload,secret,Utilities.Charset.UTF_8).map(function(b){return ('0'+((b+256)%256).toString(16)).slice(-2);}).join('');
  var different=0;for(var n=0;n<expected.length;n++)different|=expected.charCodeAt(n)^request.signature.charCodeAt(n);
  if(different)return {ok:false,code:'INVALID_SIGNATURE'};
  var message;
  try {message=JSON.parse(Utilities.newBlob(Utilities.base64Decode(request.payload)).getDataAsString('UTF-8'));}
  catch(error){return {ok:false,code:'INVALID_SNAPSHOT'};}
  if(!validSnapshot_(message))return {ok:false,code:'INVALID_SNAPSHOT'};
  var lock=LockService.getScriptLock();if(!lock.tryLock(5000))return {ok:false,code:'BUSY'};
  try {return applySnapshot_(props,message);}finally{lock.releaseLock();}
}

function validSnapshot_(m) {
  if(!m||m.action!=='sync-registrations'||typeof m.eventId!=='string'||!/^[A-Za-z0-9_-]{1,100}$/.test(m.eventId)||m.eventId.indexOf('local-test-')===0||!Number.isSafeInteger(m.revision)||m.revision<0||typeof m.requestId!=='string'||!/^[a-f0-9]{64}$/.test(m.requestId)||!Array.isArray(m.headers)||m.headers.length!==SPR_COLUMNS||!Array.isArray(m.rows)||m.rows.length>1000)return false;
  if(m.headers[0]!=='รหัสการจอง'||m.headers[SPR_COLUMNS-1]!=='Verbal จบ'||!m.headers.every(function(v){return typeof v==='string'&&v.length>0&&v.length<250;}))return false;
  var ids={};
  return m.rows.every(function(row){
    if(!Array.isArray(row)||row.length!==SPR_COLUMNS||!row.every(function(v){return typeof v==='string'&&v.length<=10000;})||!/^[A-Za-z0-9_-]{43}$/.test(row[0])||ids[row[0]])return false;
    ids[row[0]]=true;return true;
  });
}

function targetSheet_() {
  var book=SpreadsheetApp.openById(SPR_SPREADSHEET_ID);
  var sheet=book.getSheets().filter(function(s){return s.getSheetId()===SPR_SHEET_ID;})[0];
  if(!sheet)throw Error('TARGET_TAB_NOT_FOUND');
  return sheet;
}

function targetData_(sheet,props) {
  var marker=SPR_SPREADSHEET_ID+'/'+SPR_SHEET_ID;
  var owned=props.getProperty('SPR_SYNC_TARGET')===marker;
  if(props.getProperty('SPR_SYNC_TARGET')&&!owned)throw Error('TARGET_MISMATCH');
  var remembered=Number(props.getProperty('SPR_SYNC_ROWS')||0);
  if(!Number.isSafeInteger(remembered)||remembered<0)throw Error('INVALID_SYNC_STATE');
  var count=Math.max(sheet.getLastRow()-1,owned?remembered:0,0);
  if(count>10000)throw Error('TARGET_TOO_LARGE');
  // Read only existing columns; initial blank tabs commonly have only A:Z.
  var width=Math.min(SPR_COLUMNS,sheet.getMaxColumns());
  var readCount=Math.min(count+1,sheet.getMaxRows());
  var values=sheet.getRange(1,1,Math.max(1,readCount),width).getDisplayValues();
  var formulas=sheet.getRange(1,1,Math.max(1,readCount),width).getFormulas();
  var hasData=values.some(function(row){return row.some(function(v){return v!=='';});});
  var hasFormulas=formulas.some(function(row){return row.some(function(v){return v!=='';});});
  if(!owned&&(hasData||hasFormulas))throw Error('TARGET_NOT_EMPTY');
  if(hasFormulas)throw Error('TARGET_CONTAINS_FORMULAS');
  return {owned:owned,count:count,values:values,marker:marker};
}

function applySnapshot_(props,m) {
  var bound=props.getProperty('SPR_SYNC_EVENT');
  if(bound&&bound!==m.eventId)return {ok:false,code:'EVENT_MISMATCH'};
  var sheet=targetSheet_(),data;
  try {data=targetData_(sheet,props);}catch(error){
    var code=String(error.message||'');
    if(['TARGET_MISMATCH','INVALID_SYNC_STATE','TARGET_TOO_LARGE','TARGET_NOT_EMPTY','TARGET_CONTAINS_FORMULAS'].indexOf(code)<0)code='TARGET_NOT_READY';
    return {ok:false,code:code};
  }
  var revision=Number(props.getProperty('SPR_SYNC_REVISION')||-1);
  if(!Number.isSafeInteger(revision)||revision< -1)return {ok:false,code:'INVALID_SYNC_STATE'};
  if(m.revision<revision)return {ok:true,status:'stale',requestId:m.requestId,revision:revision};
  var oldHeaders=data.values[0]||[];
  var hasHeaders=oldHeaders.some(function(v){return v!=='';});
  if(hasHeaders&&JSON.stringify(oldHeaders)!==JSON.stringify(m.headers))return {ok:false,code:'HEADER_MISMATCH'};
  var incoming={},seen={},output=[],added=0,updated=0,removed=0;
  m.rows.forEach(function(row){incoming[row[0]]=row;});
  for(var r=0;r<data.count;r++) {
    var old=data.values[r+1]||[],id=old[0]||'';
    if(!id) {
      if(old.some(function(v){return v!=='';}))return {ok:false,code:'TARGET_DATA_CONFLICT'};
      output.push(blankRow_());continue;
    }
    if(!/^[A-Za-z0-9_-]{43}$/.test(id)||seen[id])return {ok:false,code:'TARGET_DATA_CONFLICT'};
    seen[id]=true;
    if(incoming[id]){output.push(incoming[id]);updated++;}
    else {output.push(blankRow_());removed++;}
  }
  m.rows.forEach(function(row){if(!seen[row[0]]){output.push(row);added++;}});
  if(output.length>10000)return {ok:false,code:'TARGET_TOO_LARGE'};
  // Claim the initially empty range before writing. A timeout can then retry
  // safely even if Google committed the cells but the response was lost.
  props.setProperty('SPR_SYNC_TARGET',data.marker);
  props.setProperty('SPR_SYNC_EVENT',m.eventId);
  if(sheet.getMaxColumns()<SPR_COLUMNS)sheet.insertColumnsAfter(sheet.getMaxColumns(),SPR_COLUMNS-sheet.getMaxColumns());
  if(sheet.getMaxRows()<output.length+1)sheet.insertRowsAfter(sheet.getMaxRows(),output.length+1-sheet.getMaxRows());
  if(!hasHeaders) {
    sheet.getRange(1,1,1,SPR_COLUMNS).setValues([m.headers]).setBackground('#193047').setFontColor('#ffffff').setFontWeight('bold').setWrap(true);
    sheet.setFrozenRows(1);
    sheet.setColumnWidths(1,SPR_COLUMNS,160);
    sheet.setColumnWidth(6,240);sheet.setColumnWidth(19,280);sheet.setColumnWidths(20,3,300);
  }
  if(output.length) {
    // Rich-text writes are literal strings, preserving 00001 and '=...' text.
    // Never evaluate questions as formulas or shift user columns outside A:AM.
    var rich=output.map(function(row){return row.map(function(v){return SpreadsheetApp.newRichTextValue().setText(v).build();});});
    sheet.getRange(2,1,output.length,SPR_COLUMNS).setRichTextValues(rich).setWrap(true).setVerticalAlignment('top');
  }
  SpreadsheetApp.flush();
  props.setProperty('SPR_SYNC_ROWS',String(output.length));
  props.setProperty('SPR_SYNC_REVISION',String(m.revision));
  return {ok:true,status:'synced',requestId:m.requestId,revision:m.revision,added:added,updated:updated,removed:removed};
}

function blankRow_(){return Array(SPR_COLUMNS).fill('');}
function sheetJSON_(value){return ContentService.createTextOutput(JSON.stringify(value)).setMimeType(ContentService.MimeType.JSON);}

// Run once to grant Sheets access and verify the exact supplied target tab.
// It reads metadata/cells only: no registrations or existing cells are changed.
function checkSheetSetup(){
  var props=PropertiesService.getScriptProperties();
  if(!/^[a-fA-F0-9]{64}$/.test(props.getProperty('APPS_SCRIPT_SECRET')||''))throw Error('Set APPS_SCRIPT_SECRET in Project Settings / Script Properties first');
  var sheet=targetSheet_();targetData_(sheet,props);
  console.log('Ready: target tab "'+sheet.getName()+'" (gid '+sheet.getSheetId()+'), columns A:AM. No cells changed.');
}
