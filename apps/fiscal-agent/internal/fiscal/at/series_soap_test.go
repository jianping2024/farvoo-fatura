package at

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const seriesRespTpl = `<?xml version="1.0" encoding="UTF-8"?><S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body><ns0:%[1]sResponse xmlns:ns0="http://at.gov.pt/"><%[1]sResp>%[2]s<infoResultOper><codResultOper>%[3]s</codResultOper><msgResultOper>%[4]s</msgResultOper></infoResultOper></%[1]sResp></ns0:%[1]sResponse></S:Body></S:Envelope>`

func seriesXML(op, info, cod, msg string) string {
	return strings.NewReplacer("%[1]s", op, "%[2]s", info, "%[3]s", cod, "%[4]s", msg).Replace(seriesRespTpl)
}

func fakeAT(t *testing.T, handler func(body string) (int, string)) (*SOAPClient, *[]string) {
	t.Helper()
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		code, out := handler(string(raw))
		w.WriteHeader(code)
		_, _ = io.WriteString(w, out)
	}))
	t.Cleanup(srv.Close)
	pub, err := LoadPasswordPublicKey(filepath.Join(testdataCertDir(t), "ChavePublicaAT.cer"))
	if err != nil {
		t.Fatal(err)
	}
	return &SOAPClient{
		Env: EnvTest, URL: srv.URL, HTTP: srv.Client(), PubKey: pub,
		Now: func() time.Time { return time.Date(2026, 11, 2, 23, 30, 0, 0, time.UTC) },
	}, &bodies
}

func TestSOAPRegisterSeries_OK(t *testing.T) {
	c, bodies := fakeAT(t, func(string) (int, string) {
		return 200, seriesXML("registarSerie", `<infoSerie><serie>FT2026T1</serie><tipoDoc>FT</tipoDoc><codValidacaoSerie>AAJFJG8HRJ</codValidacaoSerie><estado>A</estado></infoSerie>`, "2001", "ok")
	})
	res, err := c.RegisterSeries(context.Background(), RegisterRequest{
		Username: "519655290/1", Password: "s3cret", SeriesCode: "FT2026T1", DocumentType: "ft", SoftwareCertNumber: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ValidationCode != "AAJFJG8HRJ" {
		t.Fatalf("code %q", res.ValidationCode)
	}
	b := (*bodies)[0]
	for _, want := range []string{
		`xmlns:wss="http://schemas.xmlsoap.org/ws/2002/12/secext"`,
		`<wss:Username>519655290/1</wss:Username>`,
		`<serie>FT2026T1</serie><tipoSerie>N</tipoSerie><classeDoc>SI</classeDoc><tipoDoc>FT</tipoDoc>`,
		`<numInicialSeq>1</numInicialSeq>`,
		`<dataInicioPrevUtiliz>2026-11-02</dataInicioPrevUtiliz>`, // Lisbon date (UTC+0 in Nov)
		`<numCertSWFatur>0</numCertSWFatur><meioProcessamento>PI</meioProcessamento>`,
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("request missing %s\n%s", want, b)
		}
	}
	if strings.Contains(b, "s3cret") {
		t.Fatal("plaintext password on the wire")
	}
}

func TestSOAPRegisterSeries_AlreadyRegisteredRecovers(t *testing.T) {
	c, bodies := fakeAT(t, func(body string) (int, string) {
		if strings.Contains(body, "registarSerie") {
			return 200, seriesXML("registarSerie", "", "4001", "ja registada")
		}
		return 200, seriesXML("consultarSeries", `<infoSerie><serie>GR2026T1</serie><tipoDoc>GR</tipoDoc><codValidacaoSerie>AAJFJ48HR6</codValidacaoSerie><estado>A</estado></infoSerie>`, "2000", "ok")
	})
	res, err := c.RegisterSeries(context.Background(), RegisterRequest{Username: "u/1", Password: "p", SeriesCode: "GR2026T1", DocumentType: "GR"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ValidationCode != "AAJFJ48HR6" || len(*bodies) != 2 {
		t.Fatalf("got %+v calls=%d", res, len(*bodies))
	}
	if !strings.Contains((*bodies)[1], `<classeDoc>MG</classeDoc><tipoDoc>GR</tipoDoc>`) {
		t.Fatalf("consult filter: %s", (*bodies)[1])
	}
}

func TestSOAPRegisterSeries_FaultAndBusinessErrors(t *testing.T) {
	c, _ := fakeAT(t, func(string) (int, string) {
		return 500, `<?xml version="1.0"?><env:Envelope xmlns:env="http://schemas.xmlsoap.org/soap/envelope/"><env:Body><env:Fault><faultcode>1199</faultcode><faultstring>Erro de Autenticação</faultstring></env:Fault></env:Body></env:Envelope>`
	})
	_, err := c.RegisterSeries(context.Background(), RegisterRequest{Username: "u/1", Password: "topsecret", SeriesCode: "FT1", DocumentType: "FT"})
	if err == nil || !strings.Contains(err.Error(), "fault 1199") || !strings.Contains(err.Error(), "WSE") || strings.Contains(err.Error(), "topsecret") {
		t.Fatalf("fault: %v", err)
	}
	c2, _ := fakeAT(t, func(string) (int, string) { return 200, seriesXML("registarSerie", "", "4049", "data invalida") })
	if _, err := c2.RegisterSeries(context.Background(), RegisterRequest{Username: "u/1", Password: "p", SeriesCode: "FT1", DocumentType: "FT"}); err == nil || !strings.Contains(err.Error(), "AT 4049") {
		t.Fatalf("business: %v", err)
	}
}

func TestSOAPRegisterSeries_LocalValidation(t *testing.T) {
	c, bodies := fakeAT(t, func(string) (int, string) { return 200, "" })
	bad := []RegisterRequest{
		{SeriesCode: "AT2026", DocumentType: "FT"},
		{SeriesCode: "FT..1", DocumentType: "FT"},
		{SeriesCode: "-FT1", DocumentType: "FT"},
		{SeriesCode: strings.Repeat("A", 36), DocumentType: "FT"},
		{SeriesCode: "FT1", DocumentType: "XX"},
		{SeriesCode: "FT1", DocumentType: "FT", SoftwareCertNumber: 519655290},
	}
	for _, r := range bad {
		r.Username, r.Password = "u/1", "p"
		if _, err := c.RegisterSeries(context.Background(), r); err == nil {
			t.Fatalf("want local reject for %+v", r)
		}
	}
	if len(*bodies) != 0 {
		t.Fatalf("invalid input must not reach AT (%d calls)", len(*bodies))
	}
}

func TestSOAPClient_MissingMaterialsNamesConfigFields(t *testing.T) {
	for _, k := range []string{"FISCAL_AT_CLIENT_CERT", "FISCAL_AT_TRANSPORT_CERT", "FISCAL_AT_CLIENT_CERT_PASS", "FISCAL_AT_TRANSPORT_CERT_PASS", "FISCAL_AT_PASSWORD_PUBKEY", "FISCAL_AT_TRANSPORT_PUBKEY", "FISCAL_AT_SERIES_URL"} {
		t.Setenv(k, "")
	}
	c := &SOAPClient{Env: EnvTest}
	_, err := c.ConsultSeries(context.Background(), "u/1", "p", "FT1")
	if err == nil || !strings.Contains(err.Error(), "fiscal_at_client_cert") {
		t.Fatalf("got %v", err)
	}
	p := &SOAPClient{Env: EnvProd}
	if _, err := p.ConsultSeries(context.Background(), "u/1", "p", "FT1"); err == nil || !strings.Contains(err.Error(), "fiscal_at_series_url") {
		t.Fatalf("prod without URL must fail closed: %v", err)
	}
}

func TestSOAPRegisterSeries_2001WithoutInfoSerie(t *testing.T) {
	// code only in message
	c, _ := fakeAT(t, func(string) (int, string) {
		return 200, seriesXML("registarSerie", "", "2001", "Série registada com sucesso. A situação ficou ativa e foi atribuído o seguinte código de validação : AAJFJ48HR6")
	})
	res, err := c.RegisterSeries(context.Background(), RegisterRequest{Username: "u/1", Password: "p", SeriesCode: "FT2026T01", DocumentType: "FT"})
	if err != nil || res.ValidationCode != "AAJFJ48HR6" {
		t.Fatalf("msg parse: %+v %v", res, err)
	}
	// neither infoSerie nor code in message → consult
	c2, bodies := fakeAT(t, func(body string) (int, string) {
		if strings.Contains(body, "registarSerie") {
			return 200, seriesXML("registarSerie", "", "2001", "Série registada com sucesso.")
		}
		return 200, seriesXML("consultarSeries", `<infoSerie><serie>FT2026T01</serie><tipoDoc>FT</tipoDoc><codValidacaoSerie>QWER5678</codValidacaoSerie><estado>A</estado></infoSerie>`, "2000", "ok")
	})
	res, err = c2.RegisterSeries(context.Background(), RegisterRequest{Username: "u/1", Password: "p", SeriesCode: "FT2026T01", DocumentType: "FT"})
	if err != nil || res.ValidationCode != "QWER5678" || len(*bodies) != 2 {
		t.Fatalf("consult fallback: %+v %v calls=%d", res, err, len(*bodies))
	}
}

// Real AT test responses (2026-11-02, NIF 519655290): 10-char validation codes,
// consult with results still returns 2002.
const (
	realRegistarGR     = `<?xml version="1.0" encoding="UTF-8"?><S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Header><WorkContext xmlns="http://oracle.com/weblogic/soap/workarea/">x</WorkContext></S:Header><S:Body><ns0:registarSerieResponse xmlns:ns0="http://at.gov.pt/"><registarSerieResp><infoSerie><serie>GR2026T01</serie><tipoSerie>N</tipoSerie><classeDoc>MG</classeDoc><tipoDoc>GR</tipoDoc><numInicialSeq>1</numInicialSeq><dataInicioPrevUtiliz>2026-11-02</dataInicioPrevUtiliz><meioProcessamento>PI</meioProcessamento><numCertSWFatur>0</numCertSWFatur><codValidacaoSerie>AAJFJ48HR6</codValidacaoSerie><dataRegisto>2026-11-02</dataRegisto><estado>A</estado><dataEstado>2026-11-02T12:30:39Z</dataEstado><nifComunicou>519655290</nifComunicou></infoSerie><infoResultOper><codResultOper>2001</codResultOper><msgResultOper>S&#233;rie registada com sucesso. A situa&#231;&#227;o ficou ativa e foi atribu&#237;do o seguinte c&#243;digo de valida&#231;&#227;o : AAJFJ48HR6</msgResultOper></infoResultOper></registarSerieResp></ns0:registarSerieResponse></S:Body></S:Envelope>`
	realRegistarFT4001 = `<?xml version="1.0" encoding="UTF-8"?><S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body><ns0:registarSerieResponse xmlns:ns0="http://at.gov.pt/"><registarSerieResp><infoResultOper><codResultOper>4001</codResultOper><msgResultOper>N&#227;o foi poss&#237;vel registar a S&#233;rie. A S&#233;rie FT2026T01 indicada para o tipo de documento Fatura j&#225; foi registada.</msgResultOper></infoResultOper></registarSerieResp></ns0:registarSerieResponse></S:Body></S:Envelope>`
	realConsultFT      = `<?xml version="1.0" encoding="UTF-8"?><S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body><ns0:consultarSeriesResponse xmlns:ns0="http://at.gov.pt/"><consultarSeriesResp><infoSerie><serie>FT2026T01</serie><tipoSerie>N</tipoSerie><classeDoc>SI</classeDoc><tipoDoc>FT</tipoDoc><numInicialSeq>1</numInicialSeq><dataInicioPrevUtiliz>2026-11-02</dataInicioPrevUtiliz><meioProcessamento>PI</meioProcessamento><numCertSWFatur>0</numCertSWFatur><codValidacaoSerie>AAJFJG8HRJ</codValidacaoSerie><dataRegisto>2026-11-02</dataRegisto><estado>A</estado><dataEstado>2026-11-02T12:27:49Z</dataEstado><nifComunicou>519655290</nifComunicou></infoSerie><infoResultOper><codResultOper>2002</codResultOper><msgResultOper>Pesquisa efetuada com sucesso.</msgResultOper></infoResultOper></consultarSeriesResp></ns0:consultarSeriesResponse></S:Body></S:Envelope>`
)

func TestSOAPRegisterSeries_RealATResponses(t *testing.T) {
	c, _ := fakeAT(t, func(string) (int, string) { return 200, realRegistarGR })
	res, err := c.RegisterSeries(context.Background(), RegisterRequest{Username: "u/1", Password: "p", SeriesCode: "GR2026T01", DocumentType: "GR"})
	if err != nil || res.ValidationCode != "AAJFJ48HR6" {
		t.Fatalf("GR 2001: %+v %v", res, err)
	}
	c2, _ := fakeAT(t, func(body string) (int, string) {
		if strings.Contains(body, "registarSerie") {
			return 200, realRegistarFT4001
		}
		return 200, realConsultFT
	})
	res, err = c2.RegisterSeries(context.Background(), RegisterRequest{Username: "u/1", Password: "p", SeriesCode: "FT2026T01", DocumentType: "FT"})
	if err != nil || res.ValidationCode != "AAJFJG8HRJ" {
		t.Fatalf("FT 4001→consult: %+v %v", res, err)
	}
}

// Series shares classifyFault: an auth-gateway fault gets the auth-layer hint, not "schema".
func TestSeriesFault_AuthLayerHint(t *testing.T) {
	_, err := parseSeriesEnvelope(500, []byte(`<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ns="http://autenticacao.portaldasfinancas.gov.pt/services/ValidateCredencialService/2012/11"><SOAP-ENV:Body><SOAP-ENV:Fault><faultcode>33</faultcode><faultstring>mismatched tag, expected hr at offset 241 of </faultstring><detail><ns:transactionID>x</ns:transactionID></detail></SOAP-ENV:Fault></SOAP-ENV:Body></SOAP-ENV:Envelope>`))
	if err == nil || !strings.Contains(err.Error(), authLayerHint) || strings.Contains(err.Error(), "schema") {
		t.Fatalf("got %v", err)
	}
}
