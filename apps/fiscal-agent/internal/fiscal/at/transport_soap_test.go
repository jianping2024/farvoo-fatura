package at

import (
	"context"
	"strings"
	"testing"
)

func sampleTransportDoc() TransportDocument {
	return TransportDocument{
		Username: "519655290/1", Password: "s3cret",
		TaxRegistrationNumber: "519655290", CompanyName: "FAINA VAIDOSA LDA",
		CompanyAddress: TransportAddress{Detail: "Rua 1", City: "Lisboa", PostalCode: "1000-001"},
		DocumentNumber: "GR GR2026T01/1", ATCUD: "AAJFJ48HR6-1", MovementType: "GR", MovementStartTime: "2026-11-02T15:00:00",
		CustomerTaxID: "509442013", CustomerName: "Cliente <GR>",
		ShipFromAddress: "Rua A 1, 1000-001 Lisboa", ShipToAddress: "Rua B 2 4000-123 Porto",
		Lines: []TransportLine{{Description: "Item", Quantity: "2", UnitOfMeasure: "UN", UnitPrice: "10.00"}},
	}
}

func TestParseAddressPT(t *testing.T) {
	got := ParseAddressPT("  Rua B  2, 4000-123 Porto ")
	if got != (TransportAddress{Detail: "Rua B 2", PostalCode: "4000-123", City: "Porto"}) {
		t.Fatalf("%+v", got)
	}
	if got := ParseAddressPT("Armazém central"); got.Detail != "Armazém central" || got.PostalCode != "" {
		t.Fatalf("%+v", got)
	}
}

func TestBuildTransportBody(t *testing.T) {
	body, err := buildTransportBody(sampleTransportDoc())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<tns:envioDocumentoTransporteRequestElem xmlns:tns="https://servicos.portaldasfinancas.gov.pt/sgdtws/documentosTransporte/">`,
		`<TaxRegistrationNumber>519655290</TaxRegistrationNumber><CompanyName>FAINA VAIDOSA LDA</CompanyName>`,
		`<DocumentNumber>GR GR2026T01/1</DocumentNumber><ATCUD>AAJFJ48HR6-1</ATCUD><MovementStatus>N</MovementStatus><MovementDate>2026-11-02</MovementDate><MovementType>GR</MovementType><CustomerTaxID>509442013</CustomerTaxID>`,
		`<CustomerName>Cliente &lt;GR&gt;</CustomerName>`,
		`<AddressTo><Addressdetail>Rua B 2</Addressdetail><City>Porto</City><PostalCode>4000-123</PostalCode><Country>PT</Country></AddressTo>`,
		`<AddressFrom><Addressdetail>Rua A 1</Addressdetail><City>Lisboa</City><PostalCode>1000-001</PostalCode><Country>PT</Country></AddressFrom><MovementStartTime>2026-11-02T15:00:00</MovementStartTime>`,
		`<Line><ProductDescription>Item</ProductDescription><Quantity>2</Quantity><UnitOfMeasure>UN</UnitOfMeasure><UnitPrice>10.00</UnitPrice></Line>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s\n%s", want, body)
		}
	}
	for _, mut := range []func(*TransportDocument){
		func(d *TransportDocument) { d.DocumentNumber = "GR/req-123" }, // old provisional form
		func(d *TransportDocument) { d.TaxRegistrationNumber = "1234" },
		func(d *TransportDocument) { d.MovementStartTime = "2026-11-02T15:00:00Z" },
		func(d *TransportDocument) { d.Lines = nil },
	} {
		d := sampleTransportDoc()
		mut(&d)
		if _, err := buildTransportBody(d); err == nil {
			t.Fatalf("want local reject: %+v", d)
		}
	}
}

func TestSOAPTransport_Responses(t *testing.T) {
	cases := []struct {
		status int
		resp   string
		code   string
		errHas string
	}{
		{200, `<S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body><ns2:envioDocumentoTransporteResponseElem xmlns:ns2="` + transportNS + `"><ResponseStatus><ReturnCode>0</ReturnCode><ReturnMessage>Documento registado com sucesso.</ReturnMessage></ResponseStatus><DocumentNumber>GR GR2026T01/1</DocumentNumber><ATDocCodeID>123456789</ATDocCodeID></ns2:envioDocumentoTransporteResponseElem></S:Body></S:Envelope>`, "123456789", ""},
		// real AT test response 2026-11-02 (with ATCUD)
		{200, `<?xml version="1.0" encoding="UTF-8"?><S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Header><WorkContext xmlns="http://oracle.com/weblogic/soap/workarea/">x</WorkContext></S:Header><S:Body><ns0:envioDocumentoTransporteResponseElem xmlns:ns0="https://servicos.portaldasfinancas.gov.pt/sgdtws/documentosTransporte/"><ResponseStatus><ReturnCode>0</ReturnCode><ReturnMessage>OK</ReturnMessage></ResponseStatus><DocumentNumber>GR GR2026T01/2</DocumentNumber><ATCUD>AAJFJ48HR6-2</ATCUD><ATDocCodeID>1108714649</ATDocCodeID></ns0:envioDocumentoTransporteResponseElem></S:Body></S:Envelope>`, "1108714649", ""},
		// real AT test response: sub-user without WDT
		{200, `<S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body><ns0:envioDocumentoTransporteResponseElem xmlns:ns0="` + transportNS + `"><ResponseStatus><ReturnCode>54</ReturnCode><ReturnMessage>N&#227;o tem permiss&#245;es para aceder a esta opera&#231;&#227;o.</ReturnMessage></ResponseStatus></ns0:envioDocumentoTransporteResponseElem></S:Body></S:Envelope>`, "", "WDT"},
		{200, `<S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body><ns2:envioDocumentoTransporteResponseElem xmlns:ns2="` + transportNS + `"><ResponseStatus><ReturnCode>-100</ReturnCode><ReturnMessage>Data de inicio invalida</ReturnMessage></ResponseStatus></ns2:envioDocumentoTransporteResponseElem></S:Body></S:Envelope>`, "", "rejected: -100"},
		{500, `<env:Envelope xmlns:env="http://schemas.xmlsoap.org/soap/envelope/"><env:Body><env:Fault><faultcode>1199</faultcode><faultstring>Erro</faultstring></env:Fault></env:Body></env:Envelope>`, "", "unavailable: fault 1199"},
	}
	for _, tc := range cases {
		var gotBody string
		sc, _ := fakeAT(t, func(body string) (int, string) { gotBody = body; return tc.status, tc.resp })
		c := &SOAPTransportClient{Env: EnvTest, URL: sc.URL, HTTP: sc.HTTP, PubKey: sc.PubKey}
		res, err := c.RegisterDocument(context.Background(), sampleTransportDoc())
		if tc.errHas == "" {
			if err != nil || res.ATDocCodeID != tc.code {
				t.Fatalf("ok case: %+v %v", res, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), tc.errHas) {
			t.Fatalf("want %q, got %v", tc.errHas, err)
		}
		if strings.Contains(gotBody, "s3cret") || !strings.Contains(gotBody, `<wss:Username>519655290/1</wss:Username>`) {
			t.Fatal("token missing or plaintext password on the wire")
		}
	}
}
