package bootstrap

// Admin HTML for Local Fiscal setup + FT issue (embedded; shared by fiscal-local and Agent).
const adminHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8"/>
<title>Farvoo Fiscal — Setup + FT</title>
<style>
body{font-family:ui-sans-serif,system-ui;max-width:720px;margin:1.5rem auto;padding:0 1rem;background:#f6f4ef;color:#1a1a1a}
h1{font-size:1.35rem;margin:0 0 .25rem}
h2{font-size:1.05rem;margin:1.25rem 0 .4rem}
p,label{color:#444;font-size:.9rem}
button{background:#0b3d2e;color:#fff;border:0;padding:.55rem 1rem;border-radius:6px;cursor:pointer;margin:.25rem .25rem 0 0}
pre{background:#111;color:#d7ffd7;padding:1rem;border-radius:8px;overflow:auto;min-height:6rem;font-size:.8rem}
input,textarea{width:100%;padding:.4rem;box-sizing:border-box;margin:0 0 .4rem;border:1px solid #ccc;border-radius:4px}
.row{display:grid;grid-template-columns:1fr 1fr;gap:.5rem}
.box{background:#fff;border:1px solid #e2ddd3;border-radius:8px;padding:.75rem 1rem;margin:.75rem 0}
</style>
</head>
<body>
<h1>Farvoo Fiscal</h1>
<p>身份 / AT 系列 / 激活 → 开 FT（主 Agent 内嵌）</p>
<div class="box"><strong>状态</strong> <button id="btnStatus">刷新 status</button>
<pre id="status">…</pre></div>
<div class="box">
<h2>1. 纳税人</h2>
<div class="row">
<input id="nif" placeholder="NIF" value="517535009"/>
<input id="legal" placeholder="Legal name" value="Farvoo Demo Lda"/>
</div>
<input id="addr" placeholder="Address" value="Rua Demo 1"/>
<div class="row">
<input id="city" placeholder="City" value="Lisboa"/>
<input id="postal" placeholder="Postal" value="1000-001"/>
</div>
<button id="btnTax">保存纳税人</button>
</div>
<div class="box">
<h2>2. AT 凭证</h2>
<input id="atuser" placeholder="username NIF/nn" value="517535009/37"/>
<input id="atpass" type="password" placeholder="password" value="demo-pass"/>
<button id="btnAT">保存 AT 凭证</button>
</div>
<div class="box">
<h2>3. 注册系列 (mock AT)</h2>
<input id="series" placeholder="series_code" value=""/>
<button id="btnSeries">注册 FT 系列</button>
</div>
<div class="box">
<h2>4. 激活开票</h2>
<p>可粘贴产品 RSA PEM 激活（默认已开本地供给；也可在 config.json 设 fiscal_allow_local_provision）。</p>
<textarea id="pem" rows="4" placeholder="-----BEGIN … PRIVATE KEY-----"></textarea>
<button id="btnAct">激活</button>
</div>
<div class="box">
<h2>5. 开票员</h2>
<button id="btnOp">确保 cashier</button>
</div>
<div class="box">
<h2>6. 开 FT</h2>
<input id="rid" value=""/>
<button id="issue">开 FT</button>
<pre id="out">ready</pre>
</div>
<div class="box">
<h2>7. 账单草稿（同步）</h2>
<p>open 草稿可整桌开 FT（散客+现金）；成功后硬删该桌全部草稿。</p>
<button id="btnDrafts">刷新 bill-drafts</button>
<div id="draftActions"></div>
<pre id="drafts">…</pre>
</div>
<script>
const y=new Date().getFullYear();
document.getElementById('series').value='FT'+y+'DEMO01';
document.getElementById('rid').value='req-'+Date.now();
const out=document.getElementById('out');
const statusEl=document.getElementById('status');
const draftsEl=document.getElementById('drafts');
const draftActions=document.getElementById('draftActions');
async function j(method,path,body){
  const r=await fetch(path,{method,headers:body?{'Content-Type':'application/json'}:undefined,body:body?JSON.stringify(body):undefined});
  const t=await r.json();
  if(!r.ok) throw t;
  return t;
}
async function refresh(){ statusEl.textContent=JSON.stringify(await j('GET','/local/v1/setup/status'),null,2); }
async function refreshDrafts(){
  const data=await j('GET','/local/v1/bill-drafts');
  draftsEl.textContent=JSON.stringify(data,null,2);
  draftActions.innerHTML='';
  (data.drafts||[]).filter(d=>d.status==='open').forEach(d=>{
    const b=document.createElement('button');
    b.textContent='开 FT：'+(d.table_display_name||d.source_sale_id)+' ('+d.id.slice(0,8)+'…)';
    b.onclick=async()=>{
      out.textContent='issuing from draft…';
      try{
        const res=await j('POST','/local/v1/bill-drafts/'+encodeURIComponent(d.id)+'/issue',{operator_id:'op-demo-cashier'});
        out.textContent=JSON.stringify(res,null,2);
        await refreshDrafts();
      }catch(e){ out.textContent=JSON.stringify(e,null,2); }
    };
    draftActions.appendChild(b);
  });
}
document.getElementById('btnStatus').onclick=()=>refresh().catch(e=>statusEl.textContent=JSON.stringify(e,null,2));
document.getElementById('btnDrafts').onclick=()=>refreshDrafts().catch(e=>draftsEl.textContent=JSON.stringify(e,null,2));
document.getElementById('btnTax').onclick=async()=>{
  try{ await j('PUT','/local/v1/setup/taxpayer',{tax_registration_number:nif.value,legal_name:legal.value,address_detail:addr.value,city:city.value,postal_code:postal.value,country:'PT',timezone:'Europe/Lisbon',software_certificate_number:'0'}); await refresh(); out.textContent='taxpayer ok'; }
  catch(e){out.textContent=JSON.stringify(e,null,2)}
};
document.getElementById('btnAT').onclick=async()=>{
  try{ await j('PUT','/local/v1/setup/at-credentials',{username:atuser.value,password:atpass.value}); await refresh(); out.textContent='at ok'; }
  catch(e){out.textContent=JSON.stringify(e,null,2)}
};
document.getElementById('btnSeries').onclick=async()=>{
  try{ await j('POST','/local/v1/setup/series/register',{series_code:series.value,document_type:'FT',fiscal_year:y}); await refresh(); out.textContent='series ok'; }
  catch(e){out.textContent=JSON.stringify(e,null,2)}
};
document.getElementById('btnAct').onclick=async()=>{
  try{ await j('POST','/local/v1/setup/activate',{product_private_key_pem:pem.value}); await refresh(); out.textContent='activated'; }
  catch(e){out.textContent=JSON.stringify(e,null,2)}
};
document.getElementById('btnOp').onclick=async()=>{
  try{ await j('PUT','/local/v1/setup/operator',{id:'op-demo-cashier',role:'cashier',display_name:'Demo Cashier'}); await refresh(); out.textContent='operator ok'; }
  catch(e){out.textContent=JSON.stringify(e,null,2)}
};
document.getElementById('issue').onclick=async()=>{
  out.textContent='issuing…';
  const body={request_id:rid.value,operator_id:'op-demo-cashier',document_type:'FT',snapshot:{
    source_system:'farvoo',source_sale_id:'sale-'+rid.value,scope_type:'session',scope_id:'scope-'+rid.value,fiscal_purpose:'sale',
    lines:[{product_code:'DEMO1',display_name:'Prato Demo',saft_name:'Prato Demo',quantity:'1',unit_price_gross:'12.50',vat_rate:'0.23',product_type:'P',unit_of_measure:'UN'}],
    customer:{tax_id:'999999990',company_name:'Consumidor Final',country:'PT'},
    payments:[{method:'CASH',amount:'12.50'}]
  }};
  try{ out.textContent=JSON.stringify(await j('POST','/local/v1/fiscal-documents',body),null,2); }
  catch(e){ out.textContent=JSON.stringify(e,null,2); }
};
refresh().catch(()=>{});
refreshDrafts().catch(()=>{});
</script>
</body>
</html>
`
