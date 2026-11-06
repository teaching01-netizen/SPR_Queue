// Builds STORYBOARD.md and gallery.html from the captions collected by the
// Playwright chapter scripts.
import fs from 'node:fs';
import path from 'node:path';
import { OUT, CAPS } from './lib.mjs';

const caps = JSON.parse(fs.readFileSync(CAPS, 'utf8'));
const chapters = caps.chapters;
const byChapter = {};
for (const shot of [...caps.shots].sort((a, b) => a.file.localeCompare(b.file))) {
  (byChapter[shot.chapter] ??= []).push(shot);
}

const intro = `# Warwick Private Consult — full consult-day walkthrough

Screenshots captured with **Playwright** (installed Chrome, headless) against an
isolated local Go server + PostgreSQL scratch database on 6 November 2026.
No production data, Railway setting, email delivery or live Sheet was touched.

The capture environment uses a demo copy of the event schedule so the whole day
fits the review window: the 6 November High Level rounds run 14:00–15:30 and
15:30–17:00 and the day is On-site, while the 7 November Beginner rounds keep
their real times. Committed event configs and application code are unchanged.

Story data: \`น้องมีน\` (hero booking), \`น้องภูมิ\`/\`น้องเฟิร์น\` (same Math queue),
\`น้องเจ\` (Math + Verbal), \`น้องออม\` (no-show), \`น้องกัน\` (cancel + restore),
\`น้องโบว์\` (Admin permanent delete), \`น้องสวย\`/\`น้องต้น\` (swap).

Every chapter names the actor (parent / staff / admin / public screen), the
action and the expected result, so the folder can be used as an acceptance
walkthrough for venue training.
`;

let md = intro;
for (const id of Object.keys(chapters).sort()) {
  const meta = chapters[id];
  const shots = byChapter[id] ?? [];
  if (!shots.length) continue;
  md += `\n## ${meta.title}\n\n${meta.intro}\n\n`;
  for (const shot of shots) {
    md += `### ${shot.title}\n\n![${shot.title}](${shot.file})\n\n${shot.detail}\n\n`;
  }
}

md += `\n## Reproduce\n\n\`\`\`bash
cd /tmp/warwick-flow
npm i playwright-core
./reset.sh        # fresh demo database + server on 127.0.0.1:18081
node c1-parent.mjs
node c2-staff.mjs
node c3-cases.mjs
node c4-misc.mjs
node storyboard.mjs
\`\`\`\n\nThe chapter scripts live in \`scripts/e2e-flow/\`; see its README for the\ndemo environment setup.\n`;
fs.writeFileSync(path.join(OUT, 'STORYBOARD.md'), md);

const htmlChapters = Object.keys(chapters)
  .sort()
  .map((id) => {
    const meta = chapters[id];
    const shots = byChapter[id] ?? [];
    if (!shots.length) return '';
    const figures = shots
      .map(
        (shot) => `<figure id="${shot.file.replace('.png', '')}">
  <a href="${shot.file}" target="_blank"><img loading="lazy" src="${shot.file}" alt="${escapeHtml(shot.title)}"></a>
  <figcaption><strong>${escapeHtml(shot.title)}</strong><p>${escapeHtml(shot.detail)}</p></figcaption>
</figure>`,
      )
      .join('\n');
    return `<section>
  <h2>${escapeHtml(meta.title)}</h2>
  <p class="intro">${escapeHtml(meta.intro)}</p>
  <div class="grid">${figures}</div>
</section>`;
  })
  .join('\n');

const html = `<!doctype html>
<html lang="th">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Warwick Private Consult — consult-day walkthrough</title>
<style>
  :root { --ink:#20334a; --muted:#7b8795; --burgundy:#861f35; --line:#e7ebef; --paper:#f6f7f9; }
  * { box-sizing: border-box; }
  body { margin:0; font-family:'IBM Plex Sans Thai','Manrope',Tahoma,sans-serif; color:var(--ink); background:var(--paper); }
  header { background:#14263c; color:#f1e5cb; padding:40px; }
  header h1 { margin:0 0 10px; font-size:26px; }
  header p { color:#aebfce; max-width:900px; line-height:1.8; margin:0; white-space:pre-line; }
  main { max-width:1280px; margin:0 auto; padding:30px 24px 80px; }
  section { margin:44px 0; }
  section h2 { font-size:20px; border-bottom:2px solid var(--line); padding-bottom:10px; }
  .intro { color:var(--muted); line-height:1.9; margin:12px 0 22px; }
  .grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(360px,1fr)); gap:22px; }
  figure { margin:0; background:#fff; border:1px solid var(--line); border-radius:12px; overflow:hidden; box-shadow:0 3px 10px #1a2f4a10; }
  figure img { display:block; width:100%; height:auto; border-bottom:1px solid var(--line); }
  figcaption { padding:14px 16px 18px; }
  figcaption strong { display:block; margin-bottom:6px; }
  figcaption p { margin:0; color:var(--muted); font-size:13px; line-height:1.8; }
</style>
<header>
  <h1>Warwick Private Consult — full consult-day walkthrough</h1>
  <p>Playwright capture of the complete parent → staff → admin flow, including every status and edge case.
Demo schedule on an isolated local server; no production data touched.</p>
</header>
<main>${htmlChapters}</main>
</html>`;
fs.writeFileSync(path.join(OUT, 'gallery.html'), html);

function escapeHtml(value = '') {
  return String(value).replace(/[&<>"]/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[char]));
}
console.log('storyboard written:', fs.readdirSync(OUT).length, 'files');
