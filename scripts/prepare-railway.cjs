#!/usr/bin/env node
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
function prepare(root,out=path.join(root,'.private','railway')){
 if(fs.existsSync(out))throw new Error('Private settings already exist. Reuse them; do not replace APP_SECRET.');
 const accounts=['RD1','RD2','RD3','SM1','SM2','SM3','SM4'].map(username=>({username,password:crypto.randomBytes(24).toString('base64url'),role:username==='RD1'?'admin':'operator'}));
 const appSecret=crypto.randomBytes(32).toString('base64');
 fs.mkdirSync(out,{recursive:true,mode:0o700});fs.chmodSync(out,0o700);
 const write=(name,data)=>fs.writeFileSync(path.join(out,name),data,{mode:0o600,flag:'wx'});
 write('railway-web.env',`APP_ENV=production\nAPP_SECRET=${appSecret}\nSTAFF_ACCOUNTS_JSON=${JSON.stringify(accounts)}\nDATABASE_URL=\${{Postgres.DATABASE_URL}}\nEVENT_CONFIG=config/event.production.json\nPUBLIC_URL=\nEMAIL_PROVIDER=off\n`);
 write('staff-credentials.csv','Username,Password,Role\n'+accounts.map(a=>`${a.username},${a.password},${a.role}`).join('\n')+'\n');
 write('README_PRIVATE.txt','Private production settings. Keep APP_SECRET and Staff credentials in a password manager. Do not put this folder in GitHub or a shared download. Fill the domain before using Railway. Existing database requires its original APP_SECRET. No ordinary work-email password is required.\n');
 return out;
}
if(require.main===module){try{const root=path.resolve(__dirname,'..');const out=prepare(root,process.argv[2]?path.resolve(process.argv[2]):undefined);console.log('Created private Railway settings and seven Staff credentials in '+out);console.log('No secret values printed. Keep the private settings securely.');}catch(e){console.error(e.message);process.exit(1);}}
module.exports={prepare};
