// Optional Playwright QA. Fixture audio and test-entered feedback are not human evidence.
// Real pack: NODE_PATH=<optional packages> node scripts/audio/test_opus_ui.cjs <pack> <fresh-qa>
// UI only:  NODE_PATH=<optional packages> node scripts/audio/test_opus_ui.cjs --fixture <fresh-qa>
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL, fileURLToPath } = require('node:url');
const assert = require('node:assert/strict');
const { createHash } = require('node:crypto');
const { chromium } = require('playwright');

const base = path.resolve(__dirname, '../../.artifacts/opus-listening-20260906');
const templatePath = path.join(__dirname, 'opus_listening.html');
const digest = file => createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const safeJSON = value => JSON.stringify(value).replace(/</g, '\\u003c');
function inside(child, parent) {
  const relative = path.relative(parent, child);
  return relative !== '' && relative !== '..' && !relative.startsWith('..' + path.sep) && !path.isAbsolute(relative);
}
function fixture(qa) {
  const pack = path.join(qa, 'synthetic-ui-fixture');
  fs.mkdirSync(pack);
  // A deterministic tone tests WAV playback only; it is never labelled as Opus evidence.
  const rate = 8000;
  const length = rate * 2;
  const wav = Buffer.alloc(44 + length * 2);
  wav.write('RIFF'); wav.writeUInt32LE(wav.length - 8, 4); wav.write('WAVEfmt ', 8);
  wav.writeUInt32LE(16, 16); wav.writeUInt16LE(1, 20); wav.writeUInt16LE(1, 22);
  wav.writeUInt32LE(rate, 24); wav.writeUInt32LE(rate * 2, 28);
  wav.writeUInt16LE(2, 32); wav.writeUInt16LE(16, 34); wav.write('data', 36);
  wav.writeUInt32LE(length * 2, 40);
  for (let i = 0; i < length; i++) wav.writeInt16LE(Math.round(2000 * Math.sin(2 * Math.PI * 220 * i / rate)), 44 + i * 2);
  fs.writeFileSync(path.join(pack, 'synthetic tone.wav'), wav, {flag: 'wx'});
  fs.writeFileSync(path.join(pack, 'ATTRIBUTION.txt'), 'Synthetic UI QA tone only. No speech, codec or human quality evidence.\n', {flag: 'wx'});
  const samples = ['qa01', 'qa02'].map(id => ({id, duration_seconds: 2, tracks: [
    {id: 'original', label: 'ТЕСТ: исходник <без HTML>', section: 'reference'},
    {id: 'reference', label: 'ТЕСТ: PCM 8 кГц', section: 'reference'},
    {id: 'clean-off', label: 'ТЕСТ: без потерь, FEC выключен', section: 'clean'},
    {id: 'clean-on', label: 'ТЕСТ: без потерь, FEC включён', section: 'clean'},
    {id: 'plc', label: 'ТЕСТ: потери 20%, PLC', section: 'loss'},
    {id: 'fec', label: 'ТЕСТ: FEC включён; LBRR отсутствует, восстановлено FEC: 0; ' + 'длиннаяподпись'.repeat(9), section: 'loss'}
  ].filter(track => id !== 'qa02' || ['original', 'reference', 'clean-off'].includes(track.id)).map(track => ({
    ...track, path: 'synthetic tone.wav', condition: 'ui_fixture', recovery: 'not_codec_evidence'
  }))}));
  const data = {pack_id: 'SYNTHETIC-UI-QA-NOT-OPUS', settings: {
    fec_note: 'ТЕСТ: FEC включён, но LBRR отсутствует; восстановления FEC нет. <без HTML>'
  }, samples};
  fs.writeFileSync(path.join(pack, 'index.html'), fs.readFileSync(templatePath, 'utf8').replace('__PACK_DATA__', () => safeJSON(data)), {flag: 'wx'});
  return pack;
}
async function exportJSON(page, qa, filename) {
  const pending = page.waitForEvent('download');
  await page.locator('#export').click();
  const download = await pending;
  assert.equal(await download.failure(), null);
  const target = path.join(qa, filename);
  await download.saveAs(target);
  return JSON.parse(fs.readFileSync(target, 'utf8'));
}
async function metadata(page, count) {
  await page.waitForFunction(expected => {
    const audio = [...document.querySelectorAll('audio')];
    return audio.length === expected && audio.every(a => a.readyState >= 1 && Number.isFinite(a.duration) && a.duration > 0 && !a.error);
  }, count);
}
async function layout(page, width, qa, suffix = '') {
  await page.setViewportSize({width, height: width > 640 ? 900 : 844});
  const bad = await page.evaluate(() => {
    const issues = [];
    if (document.documentElement.scrollWidth > innerWidth) issues.push('document overflow');
    for (const node of document.querySelectorAll('audio, button, select, textarea, .track-name')) {
      if (!node.getClientRects().length) continue;
      const box = node.getBoundingClientRect();
      if (box.left < -1 || box.right > innerWidth + 1 || box.width <= 0) issues.push(node.tagName + ' bounds');
    }
    for (const row of document.querySelectorAll('.playback')) {
      const name = row.querySelector('.track-name').getBoundingClientRect();
      const audio = row.querySelector('audio').getBoundingClientRect();
      if (Math.min(name.right, audio.right) > Math.max(name.left, audio.left) + 1 &&
          Math.min(name.bottom, audio.bottom) > Math.max(name.top, audio.top) + 1) issues.push('label/audio overlap');
    }
    return issues;
  });
  assert.deepEqual(bad, [], `layout at ${width}px`);
  await page.screenshot({path: path.join(qa, `viewport-${width}${suffix}.png`), fullPage: true});
}

(async () => {
  assert.equal(process.argv.length, 4, 'Usage: node test_opus_ui.cjs <pack|--fixture> <fresh-qa>');
  const synthetic = process.argv[2] === '--fixture';
  const qa = path.resolve(process.argv[3]);
  assert(inside(qa, base), 'QA output must be under .artifacts/opus-listening-20260906');
  fs.mkdirSync(base, {recursive: true});
  // Resolve existing parents before writing to avoid accidentally following an external symlink.
  assert.equal(fs.realpathSync(base), base, 'artifact root must not be a symlink');
  const parent = fs.realpathSync(path.dirname(qa));
  assert(parent === base || inside(parent, base), 'QA parent must stay inside artifact root');
  let pack = synthetic ? null : fs.realpathSync(path.resolve(process.argv[2]));
  if (pack) {
    assert(inside(pack, base), 'pack must be inside artifact root');
    assert(!inside(qa, pack) && qa !== pack, 'QA must not modify an input pack');
    assert(fs.statSync(path.join(pack, 'index.html')).isFile());
  }
  fs.mkdirSync(qa, {recursive: false});
  if (synthetic) pack = fixture(qa);
  const indexPath = path.join(pack, 'index.html');
  const inputSHA = digest(indexPath);
  const browser = await chromium.launch({headless: true});
  try {
    const context = await browser.newContext({viewport: {width: 1120, height: 900}, acceptDownloads: true});
    const page = await context.newPage();
    page.setDefaultTimeout(15000);
    const errors = [];
    const remote = [];
    page.on('pageerror', error => errors.push(String(error)));
    await context.route(/^https?:\/\//, route => { remote.push(route.request().url()); return route.abort(); });
    page.on('request', request => {
      if (/^(https?|wss?):/.test(request.url())) remote.push(request.url());
    });
    await page.goto(pathToFileURL(indexPath).href);
    const data = await page.locator('#pack-data').evaluate(node => JSON.parse(node.textContent));
    if (typeof data.settings?.fec_note === 'string' && data.settings.fec_note.trim()) {
      assert.equal(await page.locator('#fec-note').textContent(), data.settings.fec_note);
      assert.equal(await page.locator('#fec-note').isVisible(), true);
    } else {
      assert.equal(await page.locator('#fec-note').isVisible(), false);
    }
    const count = data.samples.reduce((total, sample) => total + sample.tracks.length, 0);
    await metadata(page, count);
    assert.equal(await page.locator('.sample').count(), data.samples.length);
    assert.equal(await page.locator('select').count(), count);
    assert.equal(await page.locator('select').evaluateAll(nodes => nodes.every(n => n.value === '' && !n.required && !n.disabled)), true);
    assert.equal(await page.locator('audio').evaluateAll(nodes => nodes.every(a => a.paused && !a.autoplay && a.controls)), true);
    const players = [];
    for (let s = 0; s < data.samples.length; s++) {
      const sample = data.samples[s];
      const section = page.locator('.sample').nth(s);
      assert.equal(await section.getAttribute('data-sample-id'), sample.id);
      assert(sample.tracks.filter(t => t.section === 'reference').length >= 2, 'original and 8 kHz references are required');
      assert(sample.tracks.some(t => t.section === 'clean'), 'clean output is required');
      for (const [key, title] of [['reference', 'Исходники'], ['clean', 'Без потерь'], ['loss', 'Потери пакетов']]) {
        const group = section.locator(`[data-section="${key}"]`);
        const tracks = sample.tracks.filter(t => t.section === key);
        if (!tracks.length) {
          assert.equal(await group.count(), 0, 'no empty or invented groups');
          continue;
        }
        assert.equal(await group.locator('.group-title').textContent(), title);
        assert.equal(await group.locator('.track').count(), tracks.length);
        for (let t = 0; t < tracks.length; t++) {
          const track = tracks[t];
          const row = group.locator('.track').nth(t);
          assert.equal(await row.getAttribute('data-track-id'), track.id);
          assert.equal(await row.locator('.track-name').textContent(), track.label);
          const audio = row.locator('audio');
          assert.equal(await audio.isVisible(), true, 'no reveal or scoring gate');
          const info = await audio.evaluate(a => ({src: a.src, duration: a.duration}));
          assert(Math.abs(info.duration - sample.duration_seconds) <= 0.02, 'WAV duration must match sample');
          const source = fs.realpathSync(fileURLToPath(info.src));
          assert(inside(source, pack), 'audio must be local to input pack');
          players.push({sample_id: sample.id, track_id: track.id, ...info, sha256: digest(source)});
        }
      }
    }
    const attribution = page.locator('a[href="ATTRIBUTION.txt"]');
    assert.equal(await attribution.isVisible(), true);
    assert(fs.statSync(path.join(pack, 'ATTRIBUTION.txt')).size > 0);
    const empty = await exportJSON(page, qa, 'empty-export.json');
    assert.deepEqual(empty, {version: 1, pack_id: data.pack_id, ratings: []});
    // Exercise real media playback muted; do not merely dispatch synthetic play events.
    const audio = page.locator('audio');
    for (let i = 0; i < count; i++) {
      await audio.nth(i).evaluate(async a => { a.muted = true; a.currentTime = 0; await a.play(); });
      await page.waitForFunction(index => {
        const nodes = [...document.querySelectorAll('audio')];
        return nodes[index].currentTime > 0 && !nodes[index].paused && nodes.every((a, i) => i === index || a.paused);
      }, i);
    }
    await audio.evaluateAll(nodes => nodes.forEach(a => a.pause()));
    for (const width of [1120, 390, 320]) await layout(page, width, qa);
    const first = page.locator('.track').first();
    const second = page.locator('.track').nth(1);
    await first.locator('summary').click();
    await first.locator('select').selectOption('0');
    const testNote = 'AUTOMATED UI TEST, NOT HUMAN FEEDBACK: <script> & "заметка"';
    await first.locator('textarea').fill(testNote);
    await second.locator('summary').click();
    await second.locator('textarea').fill('AUTOMATED notes-only test');
    const feedback = await exportJSON(page, qa, 'test-entered-export-NOT-HUMAN.json');
    const firstTracks = data.samples[0].tracks.filter(t => t.section === 'reference');
    assert.deepEqual(feedback, {version: 1, pack_id: data.pack_id, ratings: [
      {sample_id: data.samples[0].id, track_id: firstTracks[0].id, rating: 0, notes: testNote},
      {sample_id: data.samples[0].id, track_id: firstTracks[1].id, rating: null, notes: 'AUTOMATED notes-only test'}
    ]});
    await layout(page, 320, qa, '-feedback');
    await first.locator('select').selectOption('');
    await first.locator('textarea').fill('');
    await second.locator('textarea').fill('');
    assert.deepEqual(await exportJSON(page, qa, 'cleared-export.json'), empty, 'blank must not become score zero');
    await page.reload();
    await metadata(page, count);
    assert.deepEqual(await exportJSON(page, qa, 'reload-export.json'), empty, 'no implicit persistence');
    assert.deepEqual(errors, []);
    assert.deepEqual(remote, [], 'no network requests');
    assert.equal(digest(indexPath), inputSHA, 'input page must not change');
    fs.writeFileSync(path.join(qa, 'ui-verification.json'), JSON.stringify({
      passed: true, synthetic_fixture: synthetic, pack_id: data.pack_id,
      page_sha256: inputSHA, template_sha256: digest(templatePath), test_sha256: digest(__filename),
      browser: browser.version(), players, immediate_reference_and_clean: true,
      actual_muted_playback: true, single_active_player: true, labels_verbatim: true,
      optional_export_roundtrip: true, no_implicit_persistence: true,
      viewport_widths: [1120, 390, 320], network_requests: remote, page_errors: errors,
      human_scoring: false, codec_quality_acceptance: false, hardware_acceptance: false
    }, null, 2), {flag: 'wx'});
    console.log(`PASS: ${count} local WAV players, immediate references/clean, playback exclusivity, optional export, desktop/mobile. Synthetic fixture: ${synthetic}. No human quality evidence.`);
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
