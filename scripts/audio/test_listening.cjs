// Optional browser verification. Test-entered scores are never human evidence.
const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const assert = require('node:assert/strict');

(async () => {
  const pack = path.resolve(process.argv[2]);
  const qa = path.resolve(process.argv[3]);
  const base = path.resolve(__dirname, '../../.artifacts/audio-listening-20260906');
  assert(pack.startsWith(base + path.sep) && qa.startsWith(base + path.sep));
  fs.mkdirSync(qa, {recursive:false});
  const browser = await chromium.launch({headless:true});
  try {
    const page = await browser.newPage({viewport:{width:1100,height:850}});
    const errors=[];
    page.on('pageerror',e=>errors.push(String(e)));
    await page.goto(pathToFileURL(path.join(pack,'index.html')).href);
    await page.waitForFunction(() => [...document.querySelectorAll('audio')].length===20 &&
      [...document.querySelectorAll('audio')].every(a=>a.readyState>=1 && Number.isFinite(a.duration) && !a.error));
    assert.equal(await page.locator('select').count(),16);
    const section = page.locator('section').first();
    assert.equal(await section.locator('button').isDisabled(),true);
    assert.equal(await section.locator('pre').isVisible(),false);
    await page.screenshot({path:path.join(qa,'desktop.png'),fullPage:true});
    await page.setViewportSize({width:390,height:844});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
    await page.screenshot({path:path.join(qa,'mobile.png'),fullPage:true});
    const players=await page.locator('audio').evaluateAll(nodes=>nodes.map(a=>({src:a.getAttribute('src'),duration:a.duration,ready:a.readyState})));
    for(let i=0;i<7;i++)await section.locator('select').nth(i).selectOption('3');
    assert.equal(await section.locator('button').isDisabled(),true);
    await section.locator('select').nth(7).selectOption('3');
    assert.equal(await section.locator('button').isEnabled(),true);
    await section.locator('button').click();
    assert.equal(await section.locator('pre').isVisible(),true);
    assert.equal(await section.locator('select').first().isDisabled(),true);
    assert.equal(await page.locator('section').nth(1).locator('pre').isVisible(),false);
    assert.deepEqual(errors,[]);
    fs.writeFileSync(path.join(qa,'ui-verification.json'),JSON.stringify({passed:true,players,
      transcript_gated:true,mobile_overflow:false,human_scoring:false,audio_autoplay:false},null,2));
    console.log('PASS: 20 WAV metadata loads, 16 candidates, transcript gating and desktop/mobile layout');
  } finally {
    await browser.close();
  }
})().catch(e=>{console.error(e);process.exitCode=1;});
