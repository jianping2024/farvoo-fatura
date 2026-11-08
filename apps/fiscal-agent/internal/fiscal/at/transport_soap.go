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
	nifPattern            = regexp.MustCompile(`^\d{9}$`)
)

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
				c.initErr = unavailable(fmt.Sprintf("URL for env %s not configured (config.json fiscal_at_transport_url)", c.Env))
				return
			}
			c.URL = DefaultTransportTestURL
		}
		if err := loadLiveMaterials(c.Env, &c.HTTP, &c.PubKey); err != nil {
			c.initErr = unavailable(err.Error())
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
	return xmlAddress{
		Detail:     truncRunes(strings.TrimSpace(a.Detail), 100),
		City:       truncRunes(strings.TrimSpace(a.City), 50),
		PostalCode: strings.TrimSpace(a.PostalCode),
		Country:    "PT",
	}
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

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrTransportInvalid}, a...)...)
}

// buildTransportBody checks wire-level constraints (no AT call on bad input) and marshals
// the request. Business rules (complete PT addresses, customer) are validated by the caller.
func buildTransportBody(doc TransportDocument) (string, error) {
	if !nifPattern.MatchString(doc.TaxRegistrationNumber) {
		return "", invalid("issuer NIF must be 9 digits")
	}
	if strings.TrimSpace(doc.CompanyName) == "" {
		return "", invalid("company name required")
	}
	if !transportDocNoPattern.MatchString(doc.DocumentNumber) || len(doc.DocumentNumber) > 60 {
		return "", invalid("document number %q must look like \"GR SERIE/N\"", doc.DocumentNumber)
	}
	switch doc.MovementType {
	case "GR", "GT", "GA", "GC", "GD":
	default:
		return "", invalid("movement type %q", doc.MovementType)
	}
	if _, err := time.Parse("2006-01-02T15:04:05", doc.MovementStartTime); err != nil {
		return "", invalid("movement start %q must be YYYY-MM-DDTHH:MM:SS", doc.MovementStartTime)
	}
	if len(doc.Lines) == 0 {
		return "", invalid("lines required")
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
		AddressTo:             toXMLAddress(doc.ShipTo),
		AddressFrom:           toXMLAddress(doc.ShipFrom),
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
// Failures are classified: ErrTransportInvalid | ErrTransportUnavailable | ErrTransportRejected.
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
		return nil, unavailable(err.Error())
	}
	// A new document needs ATDocCodeID. An annulment (MovementStatus=A) succeeds on ReturnCode 0
	// even when AT does not assign a second code; the stored code stays as issued.
	return parseTransportEnvelope(status, raw, strings.TrimSpace(doc.MovementStatus) != "A")
}

func parseTransportEnvelope(status int, raw []byte, requireDocCode bool) (*TransportResult, error) {
	var env struct {
		Body struct {
			Fault *soapFault `xml:"Fault"`
			Resp  struct {
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
		return nil, unavailable(fmt.Sprintf("HTTP %d unparseable response: %v", status, err))
	}
	if f := env.Body.Fault; f != nil {
		return nil, faultError(f)
	}
	if status != http.StatusOK {
		return nil, unavailable(fmt.Sprintf("HTTP %d", status))
	}
	r := env.Body.Resp
	if len(r.Statuses) == 0 {
		return nil, unavailable("response missing ReturnCode")
	}
	var msgs []string
	for _, st := range r.Statuses {
		msgs = append(msgs, fmt.Sprintf("%d %s", st.Code, strings.TrimSpace(st.Msg)))
	}
	code := r.Statuses[0].Code
	docCode := strings.TrimSpace(r.ATDocCodeID)
	if code != 0 || (requireDocCode && docCode == "") {
		return nil, returnCodeError(code, strings.Join(msgs, "; "))
	}
	return &TransportResult{ATDocCodeID: docCode, RawMessage: strings.Join(msgs, "; ")}, nil
}
