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
)

// DefaultSeriesTestURL is the AT Séries TEST endpoint (manual Específicos §2).
const DefaultSeriesTestURL = "https://servicos.portaldasfinancas.gov.pt:722/SeriesWSService"

// AT Séries result codes (manual Específicos §2.x.4).
const (
	seriesCodRegistered      = 2001
	seriesCodConsultOK       = 2000
	seriesCodConsultEmpty    = 2002
	seriesCodAlreadyRegister = 4001
)

var (
	seriesIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+([._-][A-Za-z0-9]+)*$`)
	// 2001 message: "... foi atribuído o seguinte código de validação: XXXXXXXX"
	msgValidationCode = regexp.MustCompile(`(?i)valida\S*\s*:\s*([A-Z0-9]{8,})\b`)
	// WSDL says length=8, but AT test returns 10 chars (2026-11-02); Portaria 195/2020: ≥8.
	validationCodePattern = regexp.MustCompile(`^[A-Z0-9]{8,}$`)
)

func validationCodeFromMsg(msg string) string {
	if m := msgValidationCode.FindStringSubmatch(msg); m != nil {
		return m[1]
	}
	return ""
}

// SeriesDocClass maps tipoDoc → classeDoc (manual §1.3.7–1.3.8). ONLY mapping.
func SeriesDocClass(docType string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(docType)) {
	case "FT", "FS", "FR", "ND", "NC":
		return "SI", nil
	case "GR", "GT", "GA", "GC", "GD":
		return "MG", nil
	case "CM", "CC", "FC", "FO", "NE", "OU", "OR", "PF", "RP", "RE", "CS", "LD", "RA":
		return "WD", nil
	case "RC", "RG":
		return "PY", nil
	default:
		return "", fmt.Errorf("at: unsupported document type %q for series", docType)
	}
}

// ValidateSeriesID applies manual §1.3.2 identifier rules before calling AT.
func ValidateSeriesID(id string) error {
	if id == "" || len(id) > 35 {
		return fmt.Errorf("at: series id must be 1..35 chars")
	}
	if !seriesIDPattern.MatchString(id) {
		return fmt.Errorf("at: series id %q: only [A-Za-z0-9] and single inner [._-]", id)
	}
	if strings.HasPrefix(strings.ToUpper(id), "AT") {
		return fmt.Errorf("at: series id %q: prefix AT is reserved", id)
	}
	return nil
}

// SOAPClient is the live AT Séries client (test/prod). Materials load lazily from
// LiveMaterialsFromEnv (bridged from config.json); tests inject URL/HTTP/PubKey.
type SOAPClient struct {
	Env      Env
	URL      string
	HTTP     *http.Client
	PubKey   *rsa.PublicKey
	Now      func() time.Time
	Trace    func(op string, status int, raw []byte) // optional; live probes only (no password in responses)
	initOnce sync.Once
	initErr  error
}

func (s *SOAPClient) init() error {
	s.initOnce.Do(func() {
		m := LiveMaterialsFromEnv()
		if s.URL == "" {
			s.URL = m.SeriesURL
		}
		if s.URL == "" {
			if s.Env != EnvTest {
				s.initErr = fmt.Errorf("at: series URL for env %s not configured (config.json fiscal_at_series_url)", s.Env)
				return
			}
			s.URL = DefaultSeriesTestURL
		}
		s.initErr = loadLiveMaterials(s.Env, &s.HTTP, &s.PubKey)
	})
	return s.initErr
}

func (s *SOAPClient) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// RegisterSeries calls registarSerie. 4001 (already registered at AT) recovers the
// existing ACTIVE validation code via consultarSeries (e.g. local DB lost).
func (s *SOAPClient) RegisterSeries(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	if err := ValidateSeriesID(req.SeriesCode); err != nil {
		return nil, err
	}
	docType := strings.ToUpper(strings.TrimSpace(req.DocumentType))
	class, err := SeriesDocClass(docType)
	if err != nil {
		return nil, err
	}
	if req.SoftwareCertNumber < 0 || req.SoftwareCertNumber > 9999 {
		return nil, fmt.Errorf("at: numCertSWFatur must be 0..9999 (software certificate number, not NIF), got %d", req.SoftwareCertNumber)
	}
	lisbon, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		lisbon = time.UTC
	}
	body := fmt.Sprintf(`<at:registarSerie xmlns:at="http://at.gov.pt/">`+
		`<serie>%s</serie><tipoSerie>N</tipoSerie><classeDoc>%s</classeDoc><tipoDoc>%s</tipoDoc>`+
		`<numInicialSeq>1</numInicialSeq><dataInicioPrevUtiliz>%s</dataInicioPrevUtiliz>`+
		`<numCertSWFatur>%d</numCertSWFatur><meioProcessamento>PI</meioProcessamento>`+
		`</at:registarSerie>`,
		xmlEscape(req.SeriesCode), class, docType, s.now().In(lisbon).Format("2006-01-02"), req.SoftwareCertNumber)

	resp, err := s.call(ctx, req.Username, req.Password, body)
	if err != nil {
		return nil, err
	}
	switch resp.Result.Cod {
	case seriesCodRegistered:
		// infoSerie is optional (minOccurs=0); code may only be in msgResultOper → then consult.
		if code := resp.activeCode(req.SeriesCode, docType); code != "" {
			return &RegisterResult{ValidationCode: code, RawMessage: resp.Result.Msg}, nil
		}
		if code := validationCodeFromMsg(resp.Result.Msg); code != "" {
			return &RegisterResult{ValidationCode: code, RawMessage: resp.Result.Msg}, nil
		}
		found, err := s.consult(ctx, req.Username, req.Password, req.SeriesCode, docType)
		if err != nil {
			return nil, fmt.Errorf("at: AT %d registered %s but consult for validation code failed: %w", resp.Result.Cod, req.SeriesCode, err)
		}
		if code := found.activeCode(req.SeriesCode, docType); code != "" {
			return &RegisterResult{ValidationCode: code, RawMessage: resp.Result.Msg}, nil
		}
		return nil, fmt.Errorf("at: AT %d registered %s but no validation code returned: %s", resp.Result.Cod, req.SeriesCode, resp.Result.Msg)
	case seriesCodAlreadyRegister:
		found, err := s.consult(ctx, req.Username, req.Password, req.SeriesCode, docType)
		if err != nil {
			return nil, err
		}
		if code := found.activeCode(req.SeriesCode, docType); code != "" {
			return &RegisterResult{ValidationCode: code, RawMessage: "already registered at AT (recovered): " + resp.Result.Msg}, nil
		}
		return nil, resp.Result.err()
	default:
		return nil, resp.Result.err()
	}
}

// ConsultSeries returns the ACTIVE validation code for seriesCode (any document type).
func (s *SOAPClient) ConsultSeries(ctx context.Context, username, password, seriesCode string) (*RegisterResult, error) {
	resp, err := s.consult(ctx, username, password, seriesCode, "")
	if err != nil {
		return nil, err
	}
	if code := resp.activeCode(seriesCode, ""); code != "" {
		return &RegisterResult{ValidationCode: code, RawMessage: resp.Result.Msg}, nil
	}
	return nil, fmt.Errorf("at: series %q has no active registration at AT (%d %s)", seriesCode, resp.Result.Cod, resp.Result.Msg)
}

func (s *SOAPClient) consult(ctx context.Context, username, password, seriesCode, docType string) (*seriesResp, error) {
	var b strings.Builder
	b.WriteString(`<at:consultarSeries xmlns:at="http://at.gov.pt/">`)
	if seriesCode != "" {
		b.WriteString(`<serie>` + xmlEscape(seriesCode) + `</serie>`)
	}
	if docType != "" {
		class, err := SeriesDocClass(docType)
		if err != nil {
			return nil, err
		}
		b.WriteString(`<classeDoc>` + class + `</classeDoc><tipoDoc>` + docType + `</tipoDoc>`)
	}
	b.WriteString(`</at:consultarSeries>`)
	resp, err := s.call(ctx, username, password, b.String())
	if err != nil {
		return nil, err
	}
	if resp.Result.Cod != seriesCodConsultOK && resp.Result.Cod != seriesCodConsultEmpty {
		return nil, resp.Result.err()
	}
	return resp, nil
}

// call sends one Séries SOAP request via postSOAP (errors never include the password).
func (s *SOAPClient) call(ctx context.Context, username, password, body string) (*seriesResp, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	status, raw, err := postSOAP(ctx, s.HTTP, s.URL, "", s.PubKey, s.now(), username, password, body, s.Trace)
	if err != nil {
		return nil, err
	}
	return parseSeriesEnvelope(status, raw)
}

type seriesInfo struct {
	Serie             string `xml:"serie"`
	TipoDoc           string `xml:"tipoDoc"`
	CodValidacaoSerie string `xml:"codValidacaoSerie"`
	Estado            string `xml:"estado"`
}

type seriesResult struct {
	Cod int    `xml:"codResultOper"`
	Msg string `xml:"msgResultOper"`
}

func (r seriesResult) err() error {
	return fmt.Errorf("at: AT %d: %s", r.Cod, strings.TrimSpace(r.Msg))
}

type seriesResp struct {
	Info   []seriesInfo `xml:"infoSerie"`
	Result seriesResult `xml:"infoResultOper"`
}

// activeCode returns the validation code of the ACTIVE (estado A) match, case-insensitive on serie.
func (r *seriesResp) activeCode(serie, docType string) string {
	for _, in := range r.Info {
		if !strings.EqualFold(strings.TrimSpace(in.Serie), serie) {
			continue
		}
		if docType != "" && !strings.EqualFold(strings.TrimSpace(in.TipoDoc), docType) {
			continue
		}
		if strings.TrimSpace(in.Estado) != "" && !strings.EqualFold(strings.TrimSpace(in.Estado), "A") {
			continue
		}
		if code := strings.TrimSpace(in.CodValidacaoSerie); validationCodePattern.MatchString(code) {
			return code
		}
	}
	return ""
}

func parseSeriesEnvelope(status int, raw []byte) (*seriesResp, error) {
	var env struct {
		Body struct {
			Fault *struct {
				Code   string `xml:"faultcode"`
				String string `xml:"faultstring"`
			} `xml:"Fault"`
			Op struct {
				Resp seriesResp `xml:",any"`
			} `xml:",any"`
		} `xml:"Body"`
	}
	if err := xml.NewDecoder(bytes.NewReader(raw)).Decode(&env); err != nil {
		return nil, fmt.Errorf("at: series HTTP %d: unparseable response: %w", status, err)
	}
	if f := env.Body.Fault; f != nil {
		return nil, fmt.Errorf("at: series fault %s: %s%s", strings.TrimSpace(f.Code), strings.TrimSpace(f.String), faultHint(strings.TrimSpace(f.Code)))
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("at: series HTTP %d", status)
	}
	if env.Body.Op.Resp.Result.Cod == 0 {
		return nil, fmt.Errorf("at: series response missing codResultOper")
	}
	return &env.Body.Op.Resp, nil
}
