package onvif

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// ActionHandler handles a specific ONVIF SOAP action.
type ActionHandler func(ctx context.Context, body []byte, auth *AuthResult) (interface{}, error)

// soapResponseEnvelope wraps a response payload in a SOAP envelope.
type soapResponseEnvelope struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`
	Header  SOAPHeader  `xml:"Header"`
	Body    interface{} `xml:"Body"`
}

// soapResponseBody wraps the action response element.
type soapResponseBody struct {
	Response interface{} `xml:",any"`
}

type Server struct {
	httpServer      *http.Server
	auth            *Auth
	actions         map[string]ActionHandler
	config          ConfigProvider
	snapshotHandler http.Handler // handles GET /snapshot
	discoveryHandler http.Handler // handles WS-Discovery HTTP probes
}

// ConfigProvider provides auth and media configuration. Kept as interface for testability.
type ConfigProvider interface {
	ONVIFUsername() string
	ONVIFPassword() string
	ONVIFPort() int
	RTSPPort() int
	DeviceIP() string
	CameraWidth() int
	CameraHeight() int
	CameraFPS() int
	CameraBitrate() int
}

// New creates a new ONVIF server.
func New(cfg ConfigProvider) *Server {
	return &Server{
		auth:    &Auth{Username: cfg.ONVIFUsername(), Password: cfg.ONVIFPassword()},
		actions: make(map[string]ActionHandler),
		config:  cfg,
	}
}

// RegisterAction registers a SOAP action handler.
func (s *Server) RegisterAction(action string, handler ActionHandler) {
	s.actions[action] = handler
}

// SetDiscoveryHandler sets the handler for WS-Discovery HTTP probe requests.
func (s *Server) SetDiscoveryHandler(h http.Handler) {
	s.discoveryHandler = h
}

// parseSOAPRequest parses a raw SOAP request body and extracts the action name
// from the first child element of the SOAP Body.
func parseSOAPRequest(data []byte) (action string, bodyContent []byte, err error) {
	var envelope SOAPEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return "", nil, fmt.Errorf("parsing SOAP envelope: %w", err)
	}

	trimmed := strings.TrimSpace(envelope.Body.RawXML)
	if trimmed == "" {
		return "", nil, nil
	}

	// Extract action name from first element in body
	decoder := xml.NewDecoder(strings.NewReader(trimmed))
	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", nil, fmt.Errorf("parsing SOAP body: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, []byte(trimmed), nil
		}
	}

	return "", nil, nil
}

// parseAndAuth parses a SOAP request, authenticates, and extracts the action name.
func (s *Server) parseAndAuth(r *http.Request) (action string, bodyContent []byte, err error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", nil, fmt.Errorf("reading request body: %w", err)
	}

	var envelope SOAPEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return "", nil, fmt.Errorf("parsing SOAP envelope: %w", err)
	}

	// Authenticate
	authResult := &AuthResult{}
	if envelope.Header.Security != nil && envelope.Header.Security.UsernameToken != nil {
		if authErr := s.auth.Validate(envelope.Header.Security.UsernameToken); authErr != nil {
			return "", nil, fmt.Errorf("authentication failed: %w", authErr)
		}
		authResult.Username = envelope.Header.Security.UsernameToken.Username
		authResult.OK = true
	}

	// Extract action from body
	action, bodyContent, err = parseSOAPRequest(data)
	if err != nil {
		return "", nil, err
	}

	return action, bodyContent, nil
}

// writeSOAPResponse wraps response data in a SOAP envelope and writes it.
func writeSOAPResponse(w http.ResponseWriter, data interface{}) error {
	body := soapResponseBody{Response: data}
	env := soapResponseEnvelope{
		Header: SOAPHeader{},
		Body:   body,
	}

	output, err := xml.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling SOAP response: %w", err)
	}

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	if _, err := w.Write(output); err != nil {
		return fmt.Errorf("writing response: %w", err)
	}
	return nil
}

// writeSOAPFault returns a SOAP 1.2 fault response.
func writeSOAPFault(w http.ResponseWriter, code, reason string) error {
	fault := SOAPFault{
		Code: SOAPFaultCode{Value: code},
		Reason: SOAPFaultReason{Text: reason},
	}
	env := soapFaultEnvelope{
		Header: SOAPHeader{},
		Body:   soapFaultBody{Fault: fault},
	}

	output, err := xml.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling SOAP fault: %w", err)
	}

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	if _, err := w.Write(output); err != nil {
		return fmt.Errorf("writing fault response: %w", err)
	}
	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route /snapshot endpoint before SOAP processing.
	if r.URL.Path == "/snapshot" && s.snapshotHandler != nil {
		s.snapshotHandler.ServeHTTP(w, r)
		return
	}

	if r.Method != http.MethodPost {
		writeSOAPFault(w, "soap:Sender", fmt.Sprintf("unsupported method: %s", r.Method))
		return
	}

	// Read body once for both discovery check and SOAP processing
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeSOAPFault(w, "soap:Sender", "failed to read request body")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(data))

	// WS-Discovery probe interception — check before regular SOAP action routing
	if s.discoveryHandler != nil && bytes.Contains(data, []byte("Probe")) && bytes.Contains(data, []byte("discovery")) {
		s.discoveryHandler.ServeHTTP(w, r)
		return
	}

	// Continue with regular SOAP processing
	action, bodyContent, err := s.parseAndAuth(r)
	if err != nil {
		if strings.Contains(err.Error(), "authentication failed") {
			writeSOAPFault(w, "soap:Client", err.Error())
		} else {
			writeSOAPFault(w, "soap:Client", err.Error())
		}
		return
	}

	if action == "" {
		writeSOAPFault(w, "soap:Client", "no action found in SOAP body")
		return
	}

	handler, ok := s.actions[action]
	if !ok {
		writeSOAPFault(w, "soap:Sender", fmt.Sprintf("unsupported action: %s", action))
		return
	}

	result, err := handler(r.Context(), bodyContent, &AuthResult{OK: true})
	if err != nil {
		writeSOAPFault(w, "soap:Receiver", err.Error())
		return
	}

	if err := writeSOAPResponse(w, result); err != nil {
		log.Printf("onvif: failed to write response for %s: %v", action, err)
	}
}

// Start starts the ONVIF HTTP server.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.config.ONVIFPort())
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("onvif: server starting on %s", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return s.Stop()
	case err := <-errCh:
		return err
	}
}

// Stop stops the server gracefully.
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Close()
}

// MarshalSOAP helper for tests: marshals data into a SOAP envelope bytes.
func MarshalSOAP(data interface{}) ([]byte, error) {
	body := soapResponseBody{Response: data}
	env := soapResponseEnvelope{
		Header: SOAPHeader{},
		Body:   body,
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(env); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
