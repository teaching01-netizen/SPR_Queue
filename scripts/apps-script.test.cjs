const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),crypto=require('node:crypto');
const source=fs.readFileSync(path.join(__dirname,'../ops/apps-script/Code.gs'),'utf8');
const secret='0123456789abcdef'.repeat(4);
function fixture(){
 const properties=new Map([['APPS_SCRIPT_SECRET',secret]]),sent=[];let quota=100,busy=false,fail=false,released=0;
 const props={getProperty:k=>properties.get(k)||null,getProperties:()=>Object.fromEntries(properties),setProperty:(k,v)=>properties.set(k,v),deleteProperty:k=>properties.delete(k)};
 const context=vm.createContext({console:{log:()=>{}},PropertiesService:{getScriptProperties:()=>props},LockService:{getScriptLock:()=>({tryLock:()=>!busy,releaseLock:()=>released++})},MailApp:{getRemainingDailyQuota:()=>quota,sendEmail:m=>{if(fail)throw Error('private failure');sent.push(m);}},Utilities:{Charset:{UTF_8:'UTF-8'},computeHmacSha256Signature:(text,key)=>[...crypto.createHmac('sha256',key).update(text).digest()].map(b=>b>127?b-256:b),base64Decode:s=>[...Buffer.from(s,'base64')],newBlob:bytes=>({getDataAsString:()=>Buffer.from(bytes).toString('utf8')})},ContentService:{MimeType:{JSON:'application/json'},createTextOutput:text=>({setMimeType:()=>text})}});
 vm.runInContext(source,context);
 const message={requestId:'a'.repeat(64),to:'parent@example.com',subject:'Warwick Private Consult - Booking Confirmation',html:'<b>ช่วงเวลา Consult: 14:00 - 15:30</b><a href="https://example.com/#/track/private">Check Queue Status</a>',text:'ช่วงเวลา Consult: 14:00 - 15:30'};
 const sign=(body=message,timestamp=Math.floor(Date.now()/1000))=>{const payload=Buffer.from(JSON.stringify(body)).toString('base64');return {timestamp,payload,signature:crypto.createHmac('sha256',secret).update(timestamp+'\n'+payload).digest('hex')};};
 const post=envelope=>JSON.parse(context.doPost({postData:{contents:JSON.stringify(envelope)}}));
 return {context,properties,sent,message,sign,post,setQuota:n=>quota=n,setBusy:n=>busy=n,setFail:n=>fail=n,releases:()=>released};
}
test('Apps Script verifies signed UTF-8 template and deduplicates accepted mail',()=>{
 const f=fixture();const a=f.post(f.sign());assert.equal(a.ok,true);assert.equal(a.status,'sent');assert.equal(f.sent.length,1);assert.equal(f.sent[0].htmlBody,f.message.html);assert.equal(f.sent[0].body,f.message.text);assert.equal(f.sent[0].name,'Warwick Institute');assert.equal(f.post(f.sign()).status,'duplicate');assert.equal(f.sent.length,1);assert.equal(f.releases(),2);
 const changed={...f.message,requestId:'b'.repeat(64),text:'Updated time'};assert.equal(f.post(f.sign(changed)).status,'sent');assert.equal(f.sent.length,2);
});
test('Apps Script rejects unsigned, stale, tampered and multi-recipient requests',()=>{
 const f=fixture();assert.equal(f.post({}).ok,false);const changed=f.sign();changed.payload+='x';assert.equal(f.post(changed).code,'INVALID_SIGNATURE');assert.equal(f.post(f.sign(f.message,Math.floor(Date.now()/1000)-301)).ok,false);assert.equal(f.post(f.sign(f.message,Math.floor(Date.now()/1000)+301)).ok,false);
 for(const to of ['first@example.com,second@example.com','Victim <victim@example.com>','parent@example.com\r\nBcc: other@example.com'])assert.equal(f.post(f.sign({...f.message,to})).code,'INVALID_MESSAGE');
 assert.equal(f.post(f.sign({...f.message,subject:'arbitrary bulk mail'})).ok,false);assert.equal(f.sent.length,0);
});
test('Apps Script quota/busy/send errors remain retryable and do not leak secrets',()=>{
 const f=fixture();f.setQuota(0);assert.equal(f.post(f.sign()).code,'QUOTA_EXCEEDED');f.setQuota(100);f.setBusy(true);assert.equal(f.post(f.sign()).code,'BUSY');f.setBusy(false);f.setFail(true);assert.equal(f.post(f.sign()).code,'SEND_FAILED');assert.equal(f.properties.has('SPR_SENT_'+f.message.requestId),false);f.setFail(false);assert.equal(f.post(f.sign()).status,'sent');assert.equal(f.sent.length,1);
 f.properties.delete('APPS_SCRIPT_SECRET');assert.equal(f.post(f.sign()).code,'NOT_CONFIGURED');assert.equal(f.context.doGet().includes(secret),false);
});
test('Apps Script receipts are bounded while unrelated configuration is preserved',()=>{
 const f=fixture();f.properties.set('KEEP_ME','setting');f.properties.set('SPR_SENT_'+'c'.repeat(64),String(Date.now()-8*86400000));for(let i=0;i<2000;i++)f.properties.set('SPR_SENT_'+String(i).padStart(64,'0'),String(Date.now()-10000-i));assert.equal(f.post(f.sign()).ok,true);assert.equal([...f.properties.keys()].filter(k=>k.startsWith('SPR_SENT_')).length,2000);assert.equal(f.properties.get('KEEP_ME'),'setting');assert.equal(f.properties.get('APPS_SCRIPT_SECRET'),secret);
});
