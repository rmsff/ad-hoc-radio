// Host-only playback QA; test text is not user quality evidence.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {pathToFileURL}=require('node:url');const {chromium}=require('playwright');
(async()=>{
  assert.equal(process.argv.length,4);
  const pack=fs.realpathSync(process.argv[2]),qa=path.resolve(process.argv[3]);
  const base=fs.realpathSync(path.resolve(__dirname,'../../.artifacts/codec-comparison-20260906'));
  assert(qa.startsWith(base+path.sep)&&!qa.startsWith(pack+path.sep)&&qa!==pack);
  fs.mkdirSync(qa,{recursive:false});const data=JSON.parse(fs.readFileSync(path.join(pack,'listening-data.json')));
  const browser=await chromium.launch({headless:true});
  try{
    const context=await browser.newContext({acceptDownloads:true});const page=await context.newPage();const errors=[],remote=[];
    page.on('pageerror',e=>errors.push(String(e)));await context.route(/^https?:/,r=>{remote.push(r.request().url());return r.abort()});
    await page.goto(pathToFileURL(path.join(pack,'index.html')).href);let played=0;
    for(const s of data.samples){
      await page.selectOption('#sample',s.id);
      for(const condition of ['clean','loss']){
        await page.selectOption('#condition',condition);await page.selectOption('#codec','all');
        const expected=2+s.tracks.filter(t=>t.codec!=='reference'&&(condition==='clean'?t.condition==='clean':t.condition!=='clean')).length;
        await page.waitForFunction(n=>{const a=[...document.querySelectorAll('audio')];return a.length===n&&a.every(x=>x.readyState>=1&&!x.error&&x.duration>0)},expected);
        const audio=page.locator('audio');
        for(let i=0;i<expected;i++){
          await audio.nth(i).evaluate(async a=>{a.muted=true;await a.play()});
          await page.waitForFunction(i=>{const a=[...document.querySelectorAll('audio')];return a[i].currentTime>0&&!a[i].paused&&a.every((x,j)=>j===i||x.paused)},i);played++;
        }
        await audio.evaluateAll(a=>a.forEach(x=>x.pause()));
      }
    }
    await page.selectOption('#condition','clean');await page.selectOption('#codec','codec2-2400');
    assert.equal(await page.locator('#tracks audio').count(),1);
    await page.fill('#feedback','AUTOMATED QA, NOT HUMAN SCORE');
    await page.selectOption('#sample',data.samples[0].id);assert.equal(await page.inputValue('#feedback'),'');
    await page.selectOption('#sample',data.samples[1].id);assert.equal(await page.inputValue('#feedback'),'AUTOMATED QA, NOT HUMAN SCORE');
    const pending=page.waitForEvent('download');await page.click('#export');const download=await pending;await download.saveAs(path.join(qa,'test-feedback.json'));
    const feedback=JSON.parse(fs.readFileSync(path.join(qa,'test-feedback.json')));assert.equal(Object.keys(feedback.answers).length,1);
    for(const width of [1120,390,320]){
      await page.setViewportSize({width,height:900});
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
      assert(await page.locator('audio,select,textarea,button').evaluateAll(nodes=>nodes.every(n=>{const b=n.getBoundingClientRect();return b.width>0&&b.left>=0&&b.right<=innerWidth+1})));
      await page.screenshot({path:path.join(qa,`viewport-${width}.png`),fullPage:true});
    }
    assert.deepEqual(errors,[]);assert.deepEqual(remote,[]);
    fs.writeFileSync(path.join(qa,'verification.json'),JSON.stringify({played,errors,remote,viewports:[1120,390,320],feedback:'test-only',status:'pass'},null,2),{flag:'wx'});
    console.log(JSON.stringify({played,status:'pass'}));
  }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
