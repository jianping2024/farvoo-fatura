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
	"farvoo-fiscal-agent/internal/fiscal/uievents"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

// Options holds store path and bind settings.
type Options struct {
	DBPath                    string
	DataDir                   string
	BindAddr                  string
	SoftwareCertificateNumber string
	StoreID                   string
	SigningKeyPEMPath         string
	Seed                      bool
	PrintSink                 worker.Sink // Memory UAT / fiscal-local; Agent uses PrintBytesFn
	ATClient                  at.Client
	StationPrintersFn         worker.StationPrintersFn // live config.station_printers
	StationMetaFn             func() []api.StationMeta // cloud print_stations labels; may be nil
	PrintBytesFn              worker.PrintBytesFn      // Agent: parsePrinterTarget+printToTarget ONLY
}

// Runtime is a started fiscal stack (HTTP optional).
type Runtime struct {
	DB      *store.DB
	Service *service.FiscalService
	Worker  *worker.Worker
	Sink    *worker.MemorySink
	Server  *http.Server
	Mux     *http.ServeMux
	DataDir string
	StoreID string
	cancel  context.CancelFunc
}

// StartCore opens DB, optional seed, starts print worker — no Listen.
// ONLY core bootstrap path used by Agent embed and fiscal-local.
func StartCore(opts Options) (*Runtime, error) {
	if opts.DBPath == "" {
		return nil, fmt.Errorf("bootstrap: DBPath required")
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
	if wrapped, _, ver, err := db.ActiveSigningKey(); err == nil {
		if sig, err2 := signer.NewUnwrappingSigner(opts.DataDir, wrapped, ver); err2 == nil {
			svc.SetSigner(sig)
		}
	}

	mem := &worker.MemorySink{}
	sink := opts.PrintSink
	if sink == nil {
		sink = mem
	}
	w := &worker.Worker{
		DB: db, Sink: sink,
		StationPrintersFn: opts.StationPrintersFn,
		PrintBytesFn:      opts.PrintBytesFn,
	}
	ctx, cancel := context.WithCancel(context.Background())
	go w.Loop(ctx, 200*time.Millisecond)

	mux := http.NewServeMux()
	hub := uievents.NewHub()
	db.OnBillDraftsChanged = func(openCount int, tableHint, kind string) {
		hub.NotifyBillDraftsChanged(uievents.BillDraftsChangedPayload{
			OpenCount:        openCount,
			TableDisplayName: tableHint,
			Kind:             kind,
		})
	}
	MountRoutes(mux, api.HandlerDeps{
		Fiscal: svc, StoreID: opts.StoreID,
		StationPrintersFn: opts.StationPrintersFn,
		StationMetaFn:     opts.StationMetaFn,
		UIEvents:          hub,
	})

	return &Runtime{
		DB: db, Service: svc, Worker: w, Sink: mem, Mux: mux,
		DataDir: opts.DataDir, StoreID: opts.StoreID, cancel: cancel,
	}, nil
}

// MountRoutes registers fiscal API + Admin UI on mux (ONLY HTTP mount path).
func MountRoutes(mux *http.ServeMux, deps api.HandlerDeps) {
	api.Mount(mux, deps)
	registerFiscalUIRoutes(mux)
	mux.HandleFunc("GET /", serveAdminHTML)
	mux.HandleFunc("GET /admin", serveAdminHTML)
	mux.HandleFunc("GET /fiscal", serveAdminHTML)
	mux.HandleFunc("GET /fiscal/", serveAdminHTML)
}

// Listen starts HTTP on BindAddr using rt.Mux.
func (rt *Runtime) Listen(bindAddr string) error {
	if bindAddr == "" {
		bindAddr = "127.0.0.1:17880"
	}
	if rt.Mux == nil {
		return fmt.Errorf("bootstrap: mux nil")
	}
	srv := &http.Server{Addr: bindAddr, Handler: rt.Mux}
	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-time.After(150 * time.Millisecond):
	}
	rt.Server = srv
	return nil
}

// Start = StartCore + Listen (fiscal-local / agent embed convenience).
func Start(opts Options) (*Runtime, error) {
	if opts.BindAddr == "" {
		opts.BindAddr = "127.0.0.1:17880"
	}
	rt, err := StartCore(opts)
	if err != nil {
		return nil, err
	}
	if err := rt.Listen(opts.BindAddr); err != nil {
		_ = rt.Close()
		return nil, err
	}
	return rt, nil
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
