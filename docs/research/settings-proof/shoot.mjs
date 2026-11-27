// Screenshots for docs/research/settings-proof. Usage:
//   node shoot.mjs <sode url> <out dir> <name prefix> [section ...]
// Signs in as Ada (PIN 1234) on the PIN pad, opens Settings, and for each data-section slug scrolls it to the top
// and saves <prefix>-<slug>.png at 1920x1080. "status" saves the main page with the Routers list open.
import { createRequire } from 'node:module';

const { chromium } = createRequire(import.meta.url)('playwright');

const [url, out, prefix, ...sections] = process.argv.slice(2);
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1920, height: 1080 }, deviceScaleFactor: 1 });
await page.goto(url);
await page.waitForSelector('.sode-lock__key[data-key="1"]', { state: 'visible', timeout: 20000 });
await page.evaluate(() => {
  const i = document.getElementById('lock-name');
  if (i && !i.closest('[hidden]')) {
    i.value = 'Ada';
    i.dispatchEvent(new Event('input', { bubbles: true }));
    i.blur();
  }
});
for (const d of '1234') await page.click(`.sode-lock__key[data-key="${d}"]`);
await page.evaluate(() => document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click());
await page.waitForFunction(() => !document.documentElement.hasAttribute('data-sode-lock'), null, { timeout: 20000 });
await page.waitForTimeout(3000);
for (const s of sections) {
  if (s === 'status') {
    await page.evaluate(() => { location.hash = ''; });
    await page.waitForTimeout(500);
    await page.screenshot({ path: `${out}/${prefix}-main.png` });
    continue;
  }
  await page.evaluate(() => { if (location.hash !== '#settings') location.hash = '#settings'; });
  await page.waitForTimeout(800);
  const ok = await page.evaluate((slug) => {
    const el = document.querySelector(`[data-section="${slug}"]`) || document.getElementById(slug);
    if (!el) return false;
    el.scrollIntoView({ block: 'start' });
    return true;
  }, s);
  await page.waitForTimeout(500);
  if (ok) await page.screenshot({ path: `${out}/${prefix}-${s}.png` });
  else console.log('no section', s);
}
await browser.close();
