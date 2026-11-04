const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),crypto=require('node:crypto');
const source=fs.readFileSync(path.join(__dirname,'../ops/apps-script-sheets/Code.gs'),'utf8');
const secret='0123456789abcdef'.repeat(4),id=n=>String(n).repeat(43);
const headers=Array.from({length:39},(_,n)=>'คอลัมน์ '+n);headers[0]='รหัสการจอง';headers[38]='Verbal จบ';
const row=(n,name='ชื่อ '+n)=>{const r=Array(39).fill('');r[0]=id(n);r[2]='000012';r[3]=name;r[19]='=HYPERLINK("https://example.com")';return r;};
function fixture({columns=26}={}){
 const props=new Map([['APPS_SCRIPT_SECRET',secret]]),grid=[],formulas=new Map();let maxRows=1000,maxCols=columns,busy=false,flushFail=false,writes=0,released=0;
 const get=(r,c)=>grid[r]?.[c]??'';
 function range(row,col,nr,nc){assert.ok(row>=1&&col>=1&&row+nr-1<=maxRows&&col+nc-1<=maxCols);const o={
  getDisplayValues:()=>Array.from({length:nr},(_,r)=>Array.from({length:nc},(_,c)=>String(get(row+r-1,col+c-1)))),
  getFormulas:()=>Array.from({length:nr},(_,r)=>Array.from({length:nc},(_,c)=>formulas.get((row+r-1)+':'+(col+c-1))||'')),
  setValues:values=>{writes++;values.forEach((v,r)=>v.forEach((s,c)=>{grid[row+r-1]??=[];grid[row+r-1][col+c-1]=s;}));return o;},
  setRichTextValues:values=>{writes++;values.forEach((v,r)=>v.forEach((s,c)=>{grid[row+r-1]??=[];grid[row+r-1][col+c-1]=s.text;}));return o;},
  setBackground:()=>o,setFontColor:()=>o,setFontWeight:()=>o,setWrap:()=>o,setVerticalAlignment:()=>o};return o;
 }
 const sheet={getSheetId:()=>1971459091,getName:()=> 'ปลายทางที่ผู้ใช้เลือก',getMaxRows:()=>maxRows,getMaxColumns:()=>maxCols,getLastRow:()=>{for(let r=grid.length-1;r>=0;r--)if(grid[r]?.some(v=>v!==''))return r+1;return 0;},getRange:range,insertColumnsAfter:(_n,count)=>{maxCols+=count;},insertRowsAfter:(_n,count)=>{maxRows+=count;},setFrozenRows:()=>{},setColumnWidths:()=>{},setColumnWidth:()=>{}};
 const context=vm.createContext({console:{log:()=>{},warn:()=>{}},PropertiesService:{getScriptProperties:()=>({getProperty:k=>props.get(k)||null,setProperty:(k,v)=>props.set(k,v)})},LockService:{getScriptLock:()=>({tryLock:()=>!busy,releaseLock:()=>released++})},SpreadsheetApp:{openById:book=>{assert.equal(book,'1DUxn8Br2TDwQze-RmxcFBJFMuA1gkQi5qGs8amDCsF8');return {getSheets:()=>[sheet]};},newRichTextValue:()=>{let text='';const b={setText:s=>{text=s;return b;},build:()=>({text})};return b;},flush:()=>{if(flushFail)throw Error('Google timed out after committing cells');}},Utilities:{Charset:{UTF_8:'UTF-8'},computeHmacSha256Signature:(text,key)=>[...crypto.createHmac('sha256',key).update(text).digest()].map(b=>b>127?b-256:b),base64Decode:s=>[...Buffer.from(s,'base64')],newBlob:b=>({getDataAsString:()=>Buffer.from(b).toString('utf8')})},ContentService:{MimeType:{JSON:'application/json'},createTextOutput:text=>({setMimeType:()=>text})}});
 vm.runInContext(source,context);
 const message=(revision,rows)=>({action:'sync-registrations',eventId:'warwick-spr-2026',revision,headers,rows,requestId:String(revision%10).repeat(64)});
 const sign=(m,timestamp=Math.floor(Date.now()/1000))=>{const payload=Buffer.from(JSON.stringify(m)).toString('base64');return {timestamp,payload,signature:crypto.createHmac('sha256',secret).update(timestamp+'\n'+payload).digest('hex')};};
 const post=envelope=>JSON.parse(context.doPost({postData:{contents:JSON.stringify(envelope)}}));
 return {props,grid,formulas,context,message,sign,post,setBusy:n=>busy=n,setFlushFail:n=>flushFail=n,writes:()=>writes,releases:()=>released,columns:()=>maxCols,get};
}
test('Sheets appends only new registrations, edits original rows and preserves literal strings',()=>{
 const f=fixture();f.context.checkSheetSetup();assert.equal(f.writes(),0);
 assert.equal(f.post(f.sign(f.message(1,[row(1),row(2)]))).status,'synced');assert.equal(f.columns(),39);assert.equal(f.get(1,2),'000012');assert.equal(f.get(1,19),'=HYPERLINK("https://example.com")');
 assert.equal(f.post(f.sign(f.message(1,[row(1),row(2)]))).ok,true);assert.equal(f.grid.filter(r=>r[0]===id(1)).length,1);
 assert.equal(f.post(f.sign(f.message(2,[row(2,'ชื่อแก้'),row(1),row(3)]))).ok,true);
 assert.equal(f.get(1,0),id(1));assert.equal(f.get(2,0),id(2));assert.equal(f.get(2,3),'ชื่อแก้');assert.equal(f.get(3,0),id(3));
});
test('Sheets clears removed registrations without shifting user columns or filling deleted slots',()=>{
 const f=fixture({columns:40});f.post(f.sign(f.message(1,[row(1),row(2)])));f.grid[1][39]='หมายเหตุ AN';
 f.post(f.sign(f.message(2,[row(2)])));assert.equal(f.get(1,0),'');assert.equal(f.get(1,39),'หมายเหตุ AN');assert.equal(f.get(2,0),id(2));
 f.post(f.sign(f.message(3,[])));assert.equal(f.get(2,0),'');
 f.post(f.sign(f.message(4,[row(3)])));assert.equal(f.get(3,0),id(3));assert.equal(f.get(1,0),'');assert.equal(f.props.get('SPR_SYNC_ROWS'),'3');
});
test('Sheets ignores stale snapshots, refuses a different event, and repairs interrupted writes',()=>{
 const f=fixture();f.post(f.sign(f.message(4,[row(1,'ล่าสุด')])));
 const writes=f.writes();assert.equal(f.post(f.sign(f.message(3,[row(1,'เก่า')]))).status,'stale');assert.equal(f.writes(),writes);assert.equal(f.get(1,3),'ล่าสุด');
 assert.equal(f.post(f.sign({...f.message(5,[]),eventId:'another-event'})).code,'EVENT_MISMATCH');assert.equal(f.get(1,0),id(1));
 const g=fixture();g.setFlushFail(true);assert.equal(g.post(g.sign(g.message(1,[row(1)]))).ok,false);g.setFlushFail(false);assert.equal(g.post(g.sign(g.message(1,[row(1)]))).ok,true);assert.equal(g.grid.filter(r=>r[0]===id(1)).length,1);
 // A same-revision reconciliation repairs manual cell changes after reconnect.
 g.grid[1][3]='แก้ผิดในชีต';g.post(g.sign(g.message(1,[row(1)])));assert.equal(g.get(1,3),'ชื่อ 1');
});
test('Sheets verifies signatures and payloads before any target cell write',()=>{
 const f=fixture(),m=f.message(1,[row(1)]);
 assert.equal(f.post({}).ok,false);assert.equal(f.post(f.sign(m,Math.floor(Date.now()/1000)-301)).ok,false);
 const e=f.sign(m);e.payload+='x';assert.equal(f.post(e).code,'INVALID_SIGNATURE');
 for(const bad of [{...m,rows:[row(1),row(1)]},{...m,rows:[['short']]},{...m,eventId:'local-test-event'},{...m,headers:['bad']},{...m,revision:-1}])assert.equal(f.post(f.sign(bad)).code,'INVALID_SNAPSHOT');
 assert.equal(f.writes(),0);f.setBusy(true);assert.equal(f.post(f.sign(m)).code,'BUSY');f.setBusy(false);f.props.delete('APPS_SCRIPT_SECRET');assert.equal(f.post(f.sign(m)).code,'NOT_CONFIGURED');assert.equal(f.context.doGet().includes(secret),false);
});
test('Sheets blocks populated unowned tabs, altered headers/formulas and duplicate IDs',()=>{
 const f=fixture();f.grid[0]=['ข้อมูลเดิมของผู้ใช้'];assert.throws(()=>f.context.checkSheetSetup(),/TARGET_NOT_EMPTY/);assert.equal(f.post(f.sign(f.message(1,[row(1)]))).code,'TARGET_NOT_EMPTY');assert.equal(f.writes(),0);assert.equal(f.get(0,0),'ข้อมูลเดิมของผู้ใช้');
 const g=fixture();g.post(g.sign(g.message(1,[row(1)])));g.grid[0][2]='เปลี่ยนหัวเอง';assert.equal(g.post(g.sign(g.message(2,[row(1)]))).code,'HEADER_MISMATCH');g.grid[0][2]=headers[2];
 g.formulas.set('1:3','=1+1');assert.equal(g.post(g.sign(g.message(2,[row(1)]))).code,'TARGET_CONTAINS_FORMULAS');g.formulas.clear();g.grid[2]=[...g.grid[1]];
 assert.equal(g.post(g.sign(g.message(2,[row(1)]))).code,'TARGET_DATA_CONFLICT');assert.equal(g.releases(),4);
});
