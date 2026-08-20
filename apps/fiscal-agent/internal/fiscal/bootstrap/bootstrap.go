package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/api"
	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

// Options holds store path and bind settings.
type Options struct {
	DBPath                    string
	DataDir                   string // protect keys, device key
	BindAddr                  string
	SoftwareCertificateNumber string
	StoreID                   string
	SigningKeyPEMPath         string // used when Seed=true (M0) or as activate PEM source helper
	Seed                      bool   // legacy M0 seed; requires FISCAL_ALLOW_DEV_KEY=1 for issue
	PrintSink                 worker.Sink
	ATClient                  at.Client
}

// Runtime is a started fiscal stack.
type Runtime struct {
	DB      *store.DB
	Service *service.FiscalService
	Worker  *worker.Worker
	Sink    *worker.MemorySink
	Server  *http.Server
	DataDir string
	cancel  context.CancelFunc
}

// Start opens DB, optional seed, mounts HTTP, starts print worker.
func Start(opts Options) (*Runtime, error) {
	if opts.DBPath == "" {
		return nil, fmt.Errorf("bootstrap: DBPath required")
	}
	if opts.BindAddr == "" {
		opts.BindAddr = "127.0.0.1:17880"
	}
	if opts.StoreID == "" {
		opts.StoreID = "store-demo-001"
	}
	if opts.DataDir == "" {
		opts.DataDir = filepath.Join(filepath.Dir(opts.DBPath), "secure")
	}
	if err := os.MkdirAll(opts.DataDir, 0o700); err != nil {
		return nil, err
	}

	db, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}

	if opts.Seed {
		if opts.SigningKeyPEMPath == "" {
			_ = db.Close()
			return nil, fmt.Errorf("bootstrap: SigningKeyPEMPath required when Seed=true")
		}
		_ = os.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
		sig, err := signer.LoadPEMFile(opts.SigningKeyPEMPath, 1)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		pub, err := sig.PublicKeyPEM()
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		year := time.Now().Year()
		if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
			StoreID: opts.StoreID, TaxpayerNIF: "517535009", LegalName: "Farvoo Demo Lda",
			Address: "Rua Demo 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
			SoftwareCertificateNumber: opts.SoftwareCertificateNumber,
			SeriesCode:                fmt.Sprintf("FT%dDEMO01", year),
			ValidationCode:            "CSDF7T5H", FiscalYear: year,
			OperatorID: "op-demo-cashier", OperatorName: "Demo Cashier", SigningKeyVersion: 1,
			InstallationID: "inst-demo-001", DeviceID: "device-demo-001", DevicePublicKey: "DEV-DEVICE-PUB",
		}, opts.SigningKeyPEMPath, pub); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	svc := service.New(db, nil, opts.ATClient, opts.DataDir, opts.StoreID)
	// Warm signer if already activated / seeded
	if wrapped, _, ver, err := db.ActiveSigningKey(); err == nil {
		if sig, err2 := signer.NewUnwrappingSigner(opts.DataDir, wrapped, ver); err2 == nil {
			svc.SetSigner(sig)
		}
	}

	sink := opts.PrintSink
	mem := &worker.MemorySink{}
	if sink == nil {
		sink = mem
	}
	w := &worker.Worker{DB: db, Sink: sink}
	ctx, cancel := context.WithCancel(context.Background())
	go w.Loop(ctx, 200*time.Millisecond)

	mux := http.NewServeMux()
	api.Mount(mux, api.HandlerDeps{Fiscal: svc, StoreID: opts.StoreID})
	mux.HandleFunc("GET /", serveAdminHTML)
	mux.HandleFunc("GET /admin", serveAdminHTML)

	srv := &http.Server{Addr: opts.BindAddr, Handler: mux}
	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		cancel()
		_ = db.Close()
		return nil, err
	case <-time.After(150 * time.Millisecond):
	}

	return &Runtime{DB: db, Service: svc, Worker: w, Sink: mem, Server: srv, DataDir: opts.DataDir, cancel: cancel}, nil
}

// Close shuts down worker and HTTP.
func (rt *Runtime) Close() error {
	if rt.cancel != nil {
		rt.cancel()
	}
	if rt.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = rt.Server.Shutdown(ctx)
	}
	if rt.DB != nil {
		return rt.DB.Close()
	}
	return nil
}

func serveAdminHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(adminHTML))
}

const adminHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8"/>
<title>Farvoo Fiscal Local — M1 Setup + FT</title>
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
<h1>Farvoo Fiscal Local</h1>
<p>M1：身份 / AT 系列 / 激活 → 开 FT（无 seed）</p>
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
<p>需环境 FISCAL_ALLOW_LOCAL_PROVISION=1；粘贴产品 RSA PEM 或点「用服务器测试钥」。</p>
<textarea id="pem" rows="4" placeholder="-----BEGIN … PRIVATE KEY-----"></textarea>
<button id="btnAct">激活</button>
<button id="btnActFile">用 /testdata 提示</button>
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

<script>
const y=new Date().getFullYear();
document.getElementById('series').value='FT'+y+'DEMO01';
document.getElementById('rid').value='req-'+Date.now();
const out=document.getElementById('out');
const statusEl=document.getElementById('status');
async function j(method,path,body){
  const r=await fetch(path,{method,headers:body?{'Content-Type':'application/json'}:undefined,body:body?JSON.stringify(body):undefined});
  const t=await r.json();
  if(!r.ok) throw t;
  return t;
}
async function refresh(){ statusEl.textContent=JSON.stringify(await j('GET','/local/v1/setup/status'),null,2); }
document.getElementById('btnStatus').onclick=()=>refresh().catch(e=>statusEl.textContent=JSON.stringify(e,null,2));
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
document.getElementById('btnActFile').onclick=()=>{ out.textContent='在回归脚本中会自动读取 testdata PEM；浏览器请粘贴 PEM 到文本框。'; };
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
</script>
</body>
</html>
`
