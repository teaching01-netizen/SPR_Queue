const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),os=require('node:os'),path=require('node:path');
const {prepare}=require('./prepare-railway.cjs');
test('creates seven distinct private credentials and a private application key without overwriting',()=>{
 const temp=fs.mkdtempSync(path.join(os.tmpdir(),'warwick-production-'));const out=path.join(temp,'private');prepare(temp,out);
 const read=n=>fs.readFileSync(path.join(out,n),'utf8');const env=Object.fromEntries(read('railway-web.env').trim().split('\n').map(l=>{const i=l.indexOf('=');return [l.slice(0,i),l.slice(i+1)]}));
 const accounts=JSON.parse(env.STAFF_ACCOUNTS_JSON);assert.equal(accounts.length,7);assert.equal(new Set(accounts.map(a=>a.password)).size,7);
 assert.deepEqual(accounts.map(a=>a.username),['RD1','RD2','RD3','SM1','SM2','SM3','SM4']);assert.equal(accounts.filter(a=>a.role==='admin').length,1);
 for(const a of accounts){assert.ok(a.password.length>=24);assert.notEqual(a.username,a.password)}
 assert.equal(env.EMAIL_PROVIDER,'off');assert.equal(env.DATABASE_URL,'${{Postgres.DATABASE_URL}}');
 assert.equal(Buffer.from(env.APP_SECRET,'base64').length,32);assert.deepEqual(fs.readdirSync(out).sort(),['README_PRIVATE.txt','railway-web.env','staff-credentials.csv']);
 for(const n of fs.readdirSync(out))assert.equal(fs.statSync(path.join(out,n)).mode&0o777,0o600);
 assert.equal(fs.statSync(out).mode&0o777,0o700);assert.throws(()=>prepare(temp,out));assert.equal(read('railway-web.env').includes(env.APP_SECRET),true);
 fs.rmSync(temp,{recursive:true});
});
