package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/api"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

// Options holds store path and bind settings.
type Options struct {
	DBPath                    string
	BindAddr                  string // e.g. 127.0.0.1:17880
	SoftwareCertificateNumber string
	StoreID                   string
	SigningKeyPEMPath         string
	Seed                      bool
	PrintSink                 worker.Sink
}

// Runtime is a started fiscal stack.
type Runtime struct {
	DB      *store.DB
	Service *service.FiscalService
	Worker  *worker.Worker
	Sink    *worker.MemorySink
	Server  *http.Server
	cancel  context.CancelFunc
}

// Start opens DB, seeds (optional), mounts HTTP, starts print worker.
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
	if opts.SigningKeyPEMPath == "" {
		return nil, fmt.Errorf("bootstrap: SigningKeyPEMPath required")
	}
	db, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}
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
	if opts.Seed {
		year := time.Now().Year()
		if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
			StoreID:                   opts.StoreID,
			TaxpayerNIF:               "517535009",
			LegalName:                 "Farvoo Demo Lda",
			Address:                   "Rua Demo 1",
			City:                      "Lisboa",
			PostalCode:                "1000-001",
			Timezone:                  "Europe/Lisbon",
			SoftwareCertificateNumber: opts.SoftwareCertificateNumber,
			SeriesCode:                fmt.Sprintf("FT%dDEMO01", year),
			ValidationCode:            "CSDF7T5H",
			FiscalYear:                year,
			OperatorID:                "op-demo-cashier",
			OperatorName:              "Demo Cashier",
			SigningKeyVersion:         1,
			InstallationID:            "inst-demo-001",
			DeviceID:                  "device-demo-001",
			DevicePublicKey:           "DEV-DEVICE-PUB",
		}, opts.SigningKeyPEMPath, pub); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	svc := service.New(db, sig)
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
	// Fail fast if bind failed
	select {
	case err := <-errCh:
		cancel()
		_ = db.Close()
		return nil, err
	case <-time.After(150 * time.Millisecond):
	}

	return &Runtime{DB: db, Service: svc, Worker: w, Sink: mem, Server: srv, cancel: cancel}, nil
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

// DefaultKeyPath resolves testdata key relative to module.
func DefaultKeyPath() string {
	candidates := []string{
		"internal/fiscal/testdata/dev_signing_key.pem",
		filepath.Join("apps", "fiscal-agent", "internal", "fiscal", "testdata", "dev_signing_key.pem"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

func serveAdminHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(adminHTML))
}

const adminHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8"/>
<title>Farvoo Fiscal Local — Issue FT</title>
<style>
body{font-family:ui-sans-serif,system-ui;max-width:640px;margin:2rem auto;padding:0 1rem;background:#f6f4ef;color:#1a1a1a}
h1{font-size:1.4rem;margin-bottom:.25rem}
p{color:#444}
button{background:#0b3d2e;color:#fff;border:0;padding:.6rem 1rem;border-radius:6px;cursor:pointer}
pre{background:#111;color:#d7ffd7;padding:1rem;border-radius:8px;overflow:auto;min-height:8rem}
label{display:block;margin:.5rem 0 .2rem;font-size:.85rem}
input{width:100%;padding:.4rem;box-sizing:border-box}
</style>
</head>
<body>
<h1>Farvoo Fiscal Local</h1>
<p>P0 场景：开一张 FT → SQLite → 本地打印队列</p>
<label>request_id</label>
<input id="rid" value=""/>
<button id="issue">开 FT</button>
<pre id="out">ready</pre>
<script>
const rid=document.getElementById('rid');
rid.value='req-'+Date.now();
document.getElementById('issue').onclick=async()=>{
  const out=document.getElementById('out');
  out.textContent='issuing…';
  const body={
    request_id: rid.value,
    operator_id: 'op-demo-cashier',
    document_type: 'FT',
    snapshot:{
      source_system:'farvoo',
      source_sale_id:'sale-'+rid.value,
      scope_type:'session',
      scope_id:'scope-'+rid.value,
      fiscal_purpose:'sale',
      lines:[{product_code:'DEMO1',display_name:'Prato Demo',saft_name:'Prato Demo',quantity:'1',unit_price_gross:'12.50',vat_rate:'0.23',product_type:'P',unit_of_measure:'UN'}],
      customer:{tax_id:'999999990',company_name:'Consumidor Final',country:'PT'},
      payments:[{method:'CASH',amount:'12.50'}]
    }
  };
  try{
    const r=await fetch('/local/v1/fiscal-documents',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const j=await r.json();
    out.textContent=JSON.stringify(j,null,2);
  }catch(e){out.textContent=String(e)}
};
</script>
</body>
</html>
`
