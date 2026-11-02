package at

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// DefaultTransportTestURL is the AT Documentos de Transporte TEST endpoint.
const DefaultTransportTestURL = "https://servicos.portaldasfinancas.gov.pt:701/sgdtws/documentosTransporte"

// transportNS is the WSDL targetNamespace; also the SOAPAction (documentosTransporte.wsdl).
const transportNS = "https://servicos.portaldasfinancas.gov.pt/sgdtws/documentosTransporte/"

var (
	transportDocNoPattern = regexp.MustCompile(`^[a-zA-Z0-9./_\-]+ [a-zA-Z0-9]*/[0-9]+$`)
	postalPTPattern       = regexp.MustCompile(`\b(\d{4}-\d{3})\b`)
	nifPattern            = regexp.MustCompile(`^\d{9}$`)
)

// ParseAddressPT splits a free-text PT address ("Rua X 1, 1000-001 Lisboa") into AT
// AddressStructurePT. Without a postal code the whole text is the detail.
func ParseAddressPT(s string) TransportAddress {
	s = strings.Join(strings.Fields(s), " ")
	loc := postalPTPattern.FindStringIndex(s)
	if loc == nil {
		return TransportAddress{Detail: truncRunes(s, 100)}
	}
	trim := func(v string) string { return strings.Trim(v, " ,;-") }
	return TransportAddress{
		Detail:     truncRunes(trim(s[:loc[0]]), 100),
		PostalCode: s[loc[0]:loc[1]],
		City:       truncRunes(trim(s[loc[1]:]), 50),
	}
}

func truncRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// SOAPTransportClient is the live AT envioDocumentoTransporte client (GR/GT).
// Shares mTLS + UsernameToken with SOAPClient (loadLiveMaterials / postSOAP).
type SOAPTransportClient struct {
	Env      Env
	URL      string
	HTTP     *http.Client
	PubKey   *rsa.PublicKey
	Now      func() time.Time
	Trace    func(op string, status int, raw []byte) // optional; live probes only
	initOnce sync.Once
	initErr  error
}

func (c *SOAPTransportClient) init() error {
	c.initOnce.Do(func() {
		if c.URL == "" {
			c.URL = LiveMaterialsFromEnv().TransportURL
		}
		if c.URL == "" {
			if c.Env != EnvTest {
				c.initErr = fmt.Errorf("at: transport unavailable: URL for env %s not configured (config.json fiscal_at_transport_url)", c.Env)
				return
			}
			c.URL = DefaultTransportTestURL
		}
		if err := loadLiveMaterials(c.Env, &c.HTTP, &c.PubKey); err != nil {
			c.initErr = fmt.Errorf("at: transport unavailable: %w", err)
		}
	})
	return c.initErr
}

type xmlAddress struct {
	Detail     string `xml:"Addressdetail,omitempty"`
	City       string `xml:"City,omitempty"`
	PostalCode string `xml:"PostalCode,omitempty"`
	Country    string `xml:"Country"`
}

func toXMLAddress(a TransportAddress) xmlAddress {
	return xmlAddress{Detail: a.Detail, City: a.City, PostalCode: a.PostalCode, Country: "PT"}
}

type xmlLine struct {
	ProductDescription string `xml:"ProductDescription"`
	Quantity           string `xml:"Quantity"`
	UnitOfMeasure      string `xml:"UnitOfMeasure"`
	UnitPrice          string `xml:"UnitPrice"`
}

// xmlStockMovement follows the WSDL StockMovement sequence. The literal "tns:" prefix keeps
// children unqualified (schema has no elementFormDefault).
type xmlStockMovement struct {
	XMLName               xml.Name    `xml:"tns:envioDocumentoTransporteRequestElem"`
	NS                    string      `xml:"xmlns:tns,attr"`
	TaxRegistrationNumber string      `xml:"TaxRegistrationNumber"`
	CompanyName           string      `xml:"CompanyName"`
	CompanyAddress        xmlAddress  `xml:"CompanyAddress"`
	DocumentNumber        string      `xml:"DocumentNumber"`
	ATCUD                 string      `xml:"ATCUD,omitempty"` // current AT schema (absent in the mirrored WSDL)
	MovementStatus        string      `xml:"MovementStatus"`
	MovementDate          string      `xml:"MovementDate"`
	MovementType          string      `xml:"MovementType"`
	CustomerTaxID         string      `xml:"CustomerTaxID"`
	CustomerAddress       *xmlAddress `xml:"CustomerAddress,omitempty"`
	CustomerName          string      `xml:"CustomerName,omitempty"`
	AddressTo             xmlAddress  `xml:"AddressTo"`
	AddressFrom           xmlAddress  `xml:"AddressFrom"`
	MovementStartTime     string      `xml:"MovementStartTime"`
	Lines                 []xmlLine   `xml:"Line"`
}

// buildTransportBody validates locally (no AT call on bad input) and marshals the request.
func buildTransportBody(doc TransportDocument) (string, error) {
	if !nifPattern.MatchString(doc.TaxRegistrationNumber) {
		return "", fmt.Errorf("at: transport issuer NIF must be 9 digits")
	}
	if strings.TrimSpace(doc.CompanyName) == "" {
		return "", fmt.Errorf("at: transport company name required")
	}
	if !transportDocNoPattern.MatchString(doc.DocumentNumber) || len(doc.DocumentNumber) > 60 {
		return "", fmt.Errorf("at: transport document number %q must look like \"GR SERIE/N\"", doc.DocumentNumber)
	}
	switch doc.MovementType {
	case "GR", "GT", "GA", "GC", "GD":
	default:
		return "", fmt.Errorf("at: transport movement type %q", doc.MovementType)
	}
	if _, err := time.Parse("2006-01-02T15:04:05", doc.MovementStartTime); err != nil {
		return "", fmt.Errorf("at: transport movement start %q must be YYYY-MM-DDTHH:MM:SS", doc.MovementStartTime)
	}
	if len(doc.Lines) == 0 {
		return "", fmt.Errorf("at: transport lines required")
	}
	status := doc.MovementStatus
	if status == "" {
		status = "N"
	}
	date := doc.MovementDate
	if date == "" {
		date = doc.MovementStartTime[:10]
	}
	cust := strings.TrimSpace(doc.CustomerTaxID)
	if cust == "" {
		cust = "999999990"
	}
	m := xmlStockMovement{
		NS:                    transportNS,
		TaxRegistrationNumber: doc.TaxRegistrationNumber,
		CompanyName:           truncRunes(strings.TrimSpace(doc.CompanyName), 100),
		CompanyAddress:        toXMLAddress(doc.CompanyAddress),
		DocumentNumber:        doc.DocumentNumber,
		ATCUD:                 strings.TrimSpace(doc.ATCUD),
		MovementStatus:        status,
		MovementDate:          date,
		MovementType:          doc.MovementType,
		CustomerTaxID:         cust,
		CustomerName:          truncRunes(strings.TrimSpace(doc.CustomerName), 100),
		AddressTo:             toXMLAddress(ParseAddressPT(doc.ShipToAddress)),
		AddressFrom:           toXMLAddress(ParseAddressPT(doc.ShipFromAddress)),
		MovementStartTime:     doc.MovementStartTime,
	}
	if ca := doc.CustomerAddress; ca.Detail != "" || ca.City != "" || ca.PostalCode != "" {
		x := toXMLAddress(ca)
		m.CustomerAddress = &x
	}
	for _, ln := range doc.Lines {
		uom := strings.TrimSpace(ln.UnitOfMeasure)
		if uom == "" {
			uom = "UN"
		}
		m.Lines = append(m.Lines, xmlLine{
			ProductDescription: truncRunes(strings.TrimSpace(ln.Description), 200),
			Quantity:           ln.Quantity, UnitOfMeasure: truncRunes(uom, 20), UnitPrice: ln.UnitPrice,
		})
	}
	out, err := xml.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("at: transport marshal: %w", err)
	}
	return string(out), nil
}

// RegisterDocument calls envioDocumentoTransporte. ReturnCode 0 + ATDocCodeID = success.
// Errors containing "unavailable" are connectivity/auth (retryable); others are AT rejections.
func (c *SOAPTransportClient) RegisterDocument(ctx context.Context, doc TransportDocument) (*TransportResult, error) {
	body, err := buildTransportBody(doc)
	if err != nil {
		return nil, err
	}
	if err := c.init(); err != nil {
		return nil, err
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	status, raw, err := postSOAP(ctx, c.HTTP, c.URL, transportNS, c.PubKey, now, doc.Username, doc.Password, body, c.Trace)
	if err != nil {
		return nil, fmt.Errorf("at: transport unavailable: %w", err)
	}
	return parseTransportEnvelope(status, raw)
}

func parseTransportEnvelope(status int, raw []byte) (*TransportResult, error) {
	var env struct {
		Body struct {
			Fault *struct {
				Code   string `xml:"faultcode"`
				String string `xml:"faultstring"`
			} `xml:"Fault"`
			Resp struct {
				Statuses []struct {
					Code int    `xml:"ReturnCode"`
					Msg  string `xml:"ReturnMessage"`
				} `xml:"ResponseStatus"`
				DocumentNumber string `xml:"DocumentNumber"`
				ATDocCodeID    string `xml:"ATDocCodeID"`
			} `xml:",any"`
		} `xml:"Body"`
	}
	if err := xml.NewDecoder(bytes.NewReader(raw)).Decode(&env); err != nil {
		return nil, fmt.Errorf("at: transport unavailable: HTTP %d unparseable response: %w", status, err)
	}
	if f := env.Body.Fault; f != nil {
		code := strings.TrimSpace(f.Code)
		return nil, fmt.Errorf("at: transport unavailable: fault %s: %s%s", code, strings.TrimSpace(f.String), faultHint(code))
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("at: transport unavailable: HTTP %d", status)
	}
	r := env.Body.Resp
	if len(r.Statuses) == 0 {
		return nil, fmt.Errorf("at: transport unavailable: response missing ReturnCode")
	}
	var msgs []string
	for _, st := range r.Statuses {
		msgs = append(msgs, fmt.Sprintf("%d %s", st.Code, strings.TrimSpace(st.Msg)))
	}
	if r.Statuses[0].Code != 0 || strings.TrimSpace(r.ATDocCodeID) == "" {
		hint := ""
		if r.Statuses[0].Code == 54 { // observed 2026-11-02: sub-user without transport profile
			hint = " (AT sub-user lacks the WDT transport-documents profile)"
		}
		return nil, fmt.Errorf("at: AT transport rejected: %s%s", strings.Join(msgs, "; "), hint)
	}
	return &TransportResult{ATDocCodeID: strings.TrimSpace(r.ATDocCodeID), RawMessage: strings.Join(msgs, "; ")}, nil
}
