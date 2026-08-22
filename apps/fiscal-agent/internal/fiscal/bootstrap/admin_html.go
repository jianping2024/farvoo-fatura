package bootstrap

// Admin HTML for Local Fiscal setup + FT issue (embedded; shared by fiscal-local and Agent).
// Feedback: ONLY FiscalUI.showToast (ui/toast.js) — restaurant-ordering Toast contract.
// Do not inline a second toast / banner / flash in this page.
const adminHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8"/>
<title>Farvoo Fiscal — Setup + FT</title>
<link rel="stylesheet" href="/fiscal-ui/toast.css"/>
<style>
body{font-family:ui-sans-serif,system-ui;max-width:920px;margin:1.5rem auto;padding:0 1rem;background:#f6f4ef;color:#1a1a1a}
h1{font-size:1.35rem;margin:0 0 .25rem}
h2{font-size:1.05rem;margin:1.25rem 0 .4rem}
p,label{color:#444;font-size:.9rem}
button{background:#0b3d2e;color:#fff;border:0;padding:.55rem 1rem;border-radius:6px;cursor:pointer;margin:.25rem .25rem 0 0}
button:disabled{opacity:.55;cursor:wait}
pre{background:#111;color:#d7ffd7;padding:1rem;border-radius:8px;overflow:auto;min-height:6rem;font-size:.8rem}
input,textarea{width:100%;padding:.4rem;box-sizing:border-box;margin:0 0 .4rem;border:1px solid #ccc;border-radius:4px}
.row{display:grid;grid-template-columns:1fr 1fr;gap:.5rem}
.box{background:#fff;border:1px solid #e2ddd3;border-radius:8px;padding:.75rem 1rem;margin:.75rem 0}
</style>
</head>
<body>
<h1>Farvoo Fiscal</h1>
<p>身份 / AT 系列 / 激活 → 开 FT（主 Agent 内嵌）</p>
<div class="box"><strong>状态</strong> <button id="btnStatus" type="button">刷新 status</button>
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
<button id="btnTax" type="button">保存纳税人</button>
</div>
<div class="box">
<h2>2. AT 凭证</h2>
<input id="atuser" placeholder="username NIF/nn" value="517535009/37"/>
<input id="atpass" type="password" placeholder="password" value="demo-pass"/>
<button id="btnAT" type="button">保存 AT 凭证</button>
</div>
<div class="box">
<h2>3. 注册系列 (mock AT)</h2>
<input id="series" placeholder="series_code" value=""/>
<button id="btnSeries" type="button">注册 FT 系列</button>
</div>
<div class="box">
<h2>4. 激活开票</h2>
<p>粘贴产品 RSA 私钥 PEM（须含 BEGIN/END）。空点激活会失败。</p>
<textarea id="pem" rows="4" placeholder="-----BEGIN … PRIVATE KEY-----"></textarea>
<button id="btnAct" type="button">激活</button>
</div>
<div class="box">
<h2>5. 开票员</h2>
<button id="btnOp" type="button">确保 cashier</button>
</div>
<div class="box">
<h2>6. 商品主档（LOCAL）</h2>
<p>薄商品 · REMOTE_SYNC 由账单同步写入，本区只维护 LOCAL。</p>
<div class="row">
<input id="pCode" placeholder="product_code" value="LOCAL-TEA"/>
<input id="pName" placeholder="display_name" value="花茶套餐"/>
</div>
<div class="row">
<input id="pSaft" placeholder="saft_name" value="Cha sortido"/>
<input id="pPrice" placeholder="unit_price_gross" value="4.50"/>
</div>
<input id="pVat" placeholder="vat_rate percent" value="13.00"/>
<button id="btnSaveProduct" type="button">保存 LOCAL 商品</button>
<button id="btnListProducts" type="button">刷新列表</button>
<pre id="productsOut">…</pre>
</div>
<div class="box">
<h2>7. 客户主档</h2>
<div class="row">
<input id="cNif" placeholder="customer_tax_id (9位)" value="123456789"/>
<input id="cName" placeholder="company_name" value="Cliente Demo"/>
</div>
<button id="btnSaveCustomer" type="button">保存客户</button>
<button id="btnListCustomers" type="button">刷新列表</button>
<pre id="customersOut">…</pre>
</div>
<div class="box">
<h2>8. 手动开 FT</h2>
<p>从 LOCAL 商品或临时行开票 · 唯一入口 <code>/fiscal-documents/manual</code></p>
<label>打印档口 <select id="manualStationSel"><option value="">（加载中…）</option></select></label>
<input id="manualRid" value=""/>
<input id="manualCode" placeholder="product_code（LOCAL）" value="LOCAL-TEA"/>
<input id="manualQty" placeholder="qty" value="1"/>
<div class="row">
<input id="manualNif" placeholder="NIF（空=散客）"/>
<input id="manualPay" placeholder="payment CASH" value="CASH"/>
</div>
<button id="btnManualIssue" type="button">手动开 FT</button>
<pre id="manualOut">ready</pre>
</div>
<div class="box">
<h2>9. 账单草稿工作台</h2>
<p>整桌 / 按人开 FT；每人独立 NIF（空=散客）；丢弃只删草稿不删票。</p>
<p>税票出纸复用档口绑定（TCP/USB）。无映射请先配置打印机。</p>
<label>打印档口 <select id="stationSel"><option value="">（加载中…）</option></select>
<button id="btnPrinters" type="button">刷新档口</button>
<a href="http://127.0.0.1:17892/configure" target="_blank" rel="noopener">管理档口打印机</a>
</label>
<button id="btnDrafts" type="button">刷新草稿</button>
<div id="draftActions"></div>
<pre id="drafts">…</pre>
</div>
<script src="/fiscal-ui/toast.js"></script>
<script>
const y=new Date().getFullYear();
document.getElementById('series').value='FT'+y+'DEMO01';
document.getElementById('manualRid').value='req-manual-'+Date.now();
const productsOut=document.getElementById('productsOut');
const customersOut=document.getElementById('customersOut');
const manualOut=document.getElementById('manualOut');
const out=manualOut;
const statusEl=document.getElementById('status');
const draftsEl=document.getElementById('drafts');
const draftActions=document.getElementById('draftActions');
const toast=FiscalUI.showToast;
const errText=FiscalUI.formatError;

async function j(method,path,body){
  let r, t;
  try{
    r=await fetch(path,{method,headers:body?{'Content-Type':'application/json'}:undefined,body:body?JSON.stringify(body):undefined});
  }catch(net){
    throw {error:'network', message:String(net&&net.message||net)};
  }
  try{ t=await r.json(); }catch(_){
    const raw=await r.text();
    throw {error:'bad_response', message:raw||('HTTP '+r.status)};
  }
  if(!r.ok) throw t;
  return t;
}
async function withBusy(btn, labelBusy, fn){
  const prev=btn.textContent;
  btn.disabled=true;
  btn.textContent=labelBusy||'处理中…';
  try{ await fn(); }
  finally { btn.disabled=false; btn.textContent=prev; }
}
const STATION_LS='fiscal_admin_station_id';
async function refreshPrinters(){
  const sel=document.getElementById('stationSel');
  const data=await j('GET','/local/v1/printers');
  const stations=data.stations||[];
  const prev=localStorage.getItem(STATION_LS)||sel.value||'';
  sel.innerHTML='';
  if(!stations.length){
    const o=document.createElement('option');
    o.value=''; o.textContent='（无档口 — 请先绑定）';
    sel.appendChild(o);
    return [];
  }
  stations.forEach(s=>{
    const o=document.createElement('option');
    o.value=s.id;
    o.textContent=(s.printer||s.id)+' ['+s.id.slice(0,8)+'…]';
    sel.appendChild(o);
  });
  if(prev && stations.some(s=>s.id===prev)) sel.value=prev;
  else sel.value=stations[0].id;
  localStorage.setItem(STATION_LS, sel.value);
  return stations;
}
function requireStation(){
  const sid=(document.getElementById('stationSel').value||'').trim();
  if(!sid){
    toast('请先绑定并选择打印档口（17892/configure）','error');
    throw {error:'validation_failed', message:'station_id required'};
  }
  localStorage.setItem(STATION_LS, sid);
  return sid;
}
async function refresh(){ statusEl.textContent=JSON.stringify(await j('GET','/local/v1/setup/status'),null,2); }
function issuedSet(scopes){
  const m={};
  (scopes||[]).forEach(s=>{
    const t=s.scope_type||s.ScopeType||'';
    const id=s.scope_id||s.ScopeID||'';
    if(t&&id) m[t+':'+id]=s;
  });
  return m;
}
async function refreshDrafts(){
  const data=await j('GET','/local/v1/bill-drafts');
  draftsEl.textContent=JSON.stringify(data,null,2);
  draftActions.innerHTML='';
  const opens=(data.drafts||[]).filter(d=>d.status==='open');
  for(const d of opens){
    const wrap=document.createElement('div');
    wrap.style.margin='.5rem 0';
    wrap.style.padding='.5rem';
    wrap.style.border='1px solid #ddd';
    wrap.style.borderRadius='6px';
    const title=document.createElement('div');
    title.textContent=(d.table_display_name||'')+' '+d.source_sale_id+' ['+(d.scope_type||'?')+']';
    wrap.appendChild(title);
    let detail;
    try{ detail=await j('GET','/local/v1/bill-drafts/'+encodeURIComponent(d.id)); }
    catch(e){ toast(errText(e),'error'); draftActions.appendChild(wrap); continue; }
    const issued=issuedSet(detail.issued_scopes);
    const disc=document.createElement('button');
    disc.type='button';
    disc.textContent='丢弃草稿';
    disc.onclick=async()=>{
      await withBusy(disc,'丢弃中…',async()=>{
        try{
          await j('POST','/local/v1/bill-drafts/'+encodeURIComponent(d.id)+'/discard',{});
          toast('草稿已丢弃','success');
          await refreshDrafts();
        }catch(e){ toast(errText(e),'error'); throw e; }
      });
    };
    wrap.appendChild(disc);
    if((d.scope_type||detail.payload?.scope_type)==='whole_table'){
      const nif=document.createElement('input');
      nif.placeholder='NIF（空=散客）';
      nif.style.marginTop='.35rem';
      wrap.appendChild(nif);
      const b=document.createElement('button');
      b.type='button';
      b.textContent='开整桌 FT';
      b.onclick=async()=>{
        await withBusy(b,'开票中…',async()=>{
          try{
            const res=await j('POST','/local/v1/bill-drafts/'+encodeURIComponent(d.id)+'/issue',{
              operator_id:'op-demo-cashier', mode:'whole_table', station_id:requireStation(),
              customer_nif:nif.value.trim(), customer_name:nif.value.trim()?('Cliente '+nif.value.trim()):''
            });
            const no=res.InvoiceNo||res.invoice_no||'';
            let msg='整桌开票成功 '+no+(res.cleanup_pending?'（草稿待清理）':'');
            const pj=res.PrintJobID||res.print_job_id;
            if(pj){
              try{
                const job=await j('GET','/local/v1/print-jobs/'+encodeURIComponent(pj));
                if(job.job_status==='FAILED'||job.job_status==='PRINT_FAILED'){
                  msg+=' — 已开票但未出纸: '+(job.last_error||job.job_status);
                  toast(msg,'error');
                } else toast(msg,'success');
              }catch(_){ toast(msg,'success'); }
            } else toast(msg,'success');
            out.textContent=JSON.stringify(res,null,2);
            await refreshDrafts();
          }catch(e){
            toast(errText(e),'error');
            out.textContent=JSON.stringify(e,null,2);
            throw e;
          }
        });
      };
      wrap.appendChild(b);
    } else {
      const splits=(detail.payload&&detail.payload.splits)||[];
      splits.forEach(sp=>{
        const row=document.createElement('div');
        row.style.marginTop='.4rem';
        const key='person:'+sp.scope_id;
        const done=issued[key];
        const lab=document.createElement('span');
        lab.textContent=(sp.name||sp.scope_id)+(done?(' ✓ '+(done.invoice_no||done.InvoiceNo||'')):'');
        row.appendChild(lab);
        if(!done){
          const nif=document.createElement('input');
          nif.placeholder='NIF 此人（空=散客）';
          nif.style.display='block';
          nif.style.margin='.2rem 0';
          row.appendChild(nif);
          const b=document.createElement('button');
          b.type='button';
          b.textContent='开此人 FT';
          b.onclick=async()=>{
            await withBusy(b,'开票中…',async()=>{
              try{
                const res=await j('POST','/local/v1/bill-drafts/'+encodeURIComponent(d.id)+'/issue',{
                  operator_id:'op-demo-cashier', mode:'person', scope_id:sp.scope_id, station_id:requireStation(),
                  customer_nif:nif.value.trim(), customer_name:nif.value.trim()?(sp.name||nif.value.trim()):''
                });
                const no=res.InvoiceNo||res.invoice_no||'';
                toast('按人开票成功 '+no,'success');
                out.textContent=JSON.stringify(res,null,2);
                await refreshDrafts();
              }catch(e){
                toast(errText(e),'error');
                out.textContent=JSON.stringify(e,null,2);
                throw e;
              }
            });
          };
          row.appendChild(b);
        }
        wrap.appendChild(row);
      });
    }
    draftActions.appendChild(wrap);
  }
}
document.getElementById('btnStatus').onclick=()=>withBusy(btnStatus,'刷新中…',async()=>{
  try{ await refresh(); toast('状态已刷新','success'); }
  catch(e){ statusEl.textContent=JSON.stringify(e,null,2); toast(errText(e),'error'); throw e; }
});
document.getElementById('btnPrinters').onclick=()=>withBusy(btnPrinters,'刷新中…',async()=>{
  try{
    const st=await refreshPrinters();
    toast(st.length?('档口 '+st.length+' 个'):'无档口映射','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
document.getElementById('btnDrafts').onclick=()=>withBusy(btnDrafts,'刷新中…',async()=>{
  try{ await refreshDrafts(); toast('草稿已刷新','success'); }
  catch(e){ draftsEl.textContent=JSON.stringify(e,null,2); toast(errText(e),'error'); throw e; }
});
document.getElementById('stationSel').onchange=()=>{
  const v=document.getElementById('stationSel').value;
  if(v) localStorage.setItem(STATION_LS, v);
};
refreshPrinters().catch(()=>{});
document.getElementById('btnTax').onclick=()=>withBusy(btnTax,'保存中…',async()=>{
  try{
    await j('PUT','/local/v1/setup/taxpayer',{tax_registration_number:nif.value,legal_name:legal.value,address_detail:addr.value,city:city.value,postal_code:postal.value,country:'PT',timezone:'Europe/Lisbon',software_certificate_number:'0'});
    await refresh();
    toast('纳税人保存成功','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
document.getElementById('btnAT').onclick=()=>withBusy(btnAT,'保存中…',async()=>{
  try{
    await j('PUT','/local/v1/setup/at-credentials',{username:atuser.value,password:atpass.value});
    await refresh();
    toast('AT 凭证保存成功','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
document.getElementById('btnSeries').onclick=()=>withBusy(btnSeries,'注册中…',async()=>{
  try{
    await j('POST','/local/v1/setup/series/register',{series_code:series.value,document_type:'FT',fiscal_year:y});
    await refresh();
    toast('系列注册成功','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
document.getElementById('btnAct').onclick=()=>withBusy(btnAct,'激活中…',async()=>{
  try{
    if(!pem.value.trim()){ throw {error:'validation_failed', message:'请先粘贴产品私钥 PEM'}; }
    await j('POST','/local/v1/setup/activate',{product_private_key_pem:pem.value});
    await refresh();
    toast('开票已激活','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
async function refreshManualPrinters(){
  const sel=document.getElementById('manualStationSel');
  const data=await j('GET','/local/v1/printers');
  const stations=data.stations||[];
  sel.innerHTML='';
  stations.forEach(s=>{
    const o=document.createElement('option');
    o.value=s.id; o.textContent=(s.printer||s.id);
    sel.appendChild(o);
  });
  if(stations.length) sel.value=stations[0].id;
}
document.getElementById('btnSaveProduct').onclick=()=>withBusy(btnSaveProduct,'保存中…',async()=>{
  try{
    const res=await j('POST','/local/v1/products',{product_code:pCode.value,display_name:pName.value,saft_name:pSaft.value,unit_price_gross:pPrice.value,vat_rate:pVat.value});
    productsOut.textContent=JSON.stringify(res,null,2);
    toast('LOCAL 商品已保存','success');
  }catch(e){ toast(errText(e),'error'); productsOut.textContent=JSON.stringify(e,null,2); throw e; }
});
document.getElementById('btnListProducts').onclick=()=>withBusy(btnListProducts,'刷新中…',async()=>{
  try{
    const res=await j('GET','/local/v1/products');
    productsOut.textContent=JSON.stringify(res,null,2);
    toast('商品列表已刷新','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
document.getElementById('btnSaveCustomer').onclick=()=>withBusy(btnSaveCustomer,'保存中…',async()=>{
  try{
    const res=await j('POST','/local/v1/customers',{customer_tax_id:cNif.value,company_name:cName.value});
    customersOut.textContent=JSON.stringify(res,null,2);
    toast('客户已保存','success');
  }catch(e){ toast(errText(e),'error'); customersOut.textContent=JSON.stringify(e,null,2); throw e; }
});
document.getElementById('btnListCustomers').onclick=()=>withBusy(btnListCustomers,'刷新中…',async()=>{
  try{
    const res=await j('GET','/local/v1/customers');
    customersOut.textContent=JSON.stringify(res,null,2);
    toast('客户列表已刷新','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
document.getElementById('btnManualIssue').onclick=()=>withBusy(btnManualIssue,'开票中…',async()=>{
  manualOut.textContent='issuing…';
  const sid=(document.getElementById('manualStationSel').value||'').trim();
  const body={
    request_id:manualRid.value, operator_id:'op-demo-cashier', station_id:sid||undefined,
    customer_nif:manualNif.value.trim(), payment_method:manualPay.value.trim()||'CASH',
    lines:[{product_code:manualCode.value.trim(), quantity:manualQty.value.trim()||'1'}]
  };
  try{
    const res=await j('POST','/local/v1/fiscal-documents/manual',body);
    manualOut.textContent=JSON.stringify(res,null,2);
    toast('手动 FT 成功 '+(res.invoice_no||''),'success');
    document.getElementById('manualRid').value='req-manual-'+Date.now();
  }catch(e){
    manualOut.textContent=JSON.stringify(e,null,2);
    toast(errText(e),'error');
    throw e;
  }
});
refreshManualPrinters().catch(()=>{});
document.getElementById('btnOp').onclick=()=>withBusy(btnOp,'保存中…',async()=>{
  try{
    await j('PUT','/local/v1/setup/operator',{id:'op-demo-cashier',role:'cashier',display_name:'Demo Cashier'});
    await refresh();
    toast('开票员已保存','success');
  }catch(e){ toast(errText(e),'error'); throw e; }
});
refresh().catch(e=>{ statusEl.textContent=JSON.stringify(e,null,2); toast(errText(e),'error'); });
refreshDrafts().catch(e=>{ draftsEl.textContent=JSON.stringify(e,null,2); toast(errText(e),'error'); });
document.getElementById('btnListProducts').click();
document.getElementById('btnListCustomers').click();
</script>
</body>
</html>
`
