// Package sip contains the zgrab2 Module implementation for SIP.
//
// The scan performs an OPTIONS request to query the SIP server's capabilities.
// It supports both UDP and TCP connections.
//
// The output includes the server's response and any supported methods or capabilities.
package sip

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults contains the output of the SIP scan.
type ScanResults struct {
	// RawResponse is the full, raw response from the SIP server, decoded as UTF-8.
	RawResponse string `json:"raw_response,omitempty"`

	// StatusCode is the numeric status code from the response.
	StatusCode string `json:"status_code,omitempty"`

	// StatusLine is the full status line from the response.
	StatusLine string `json:"status_line,omitempty"`

	// Methods is a list of supported methods reported by the server.
	Methods []string `json:"methods,omitempty"`

	// Headers contains all headers from the response.
	Headers map[string]string `json:"headers,omitempty"`

	// TLSLog is the standard shared TLS handshake log.
	TLSLog *zgrab2.TLSLog `json:"tls,omitempty"`
}

// Flags defines the SIP-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	zgrab2.TLSFlags

	Verbose     bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
	UseTCP      bool `long:"tcp" description:"Use TCP instead of UDP for SIP scanning"`
	ImplicitTLS bool `long:"tls" description:"Attempt to connect via a TLS wrapped connection"`
	Timeout     uint `long:"tcp-timeout" default:"5" description:"Set connection timeout in seconds"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface.
type Scanner struct {
	config *Flags
}

// RegisterModule registers the sip zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("sip", "SIP", module.Description(), 5060, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns a default Flags object.
func (m *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (m *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description returns an overview of this module.
func (m *Module) Description() string {
	return "Probe for SIP (Session Initiation Protocol) servers using UDP or TCP"
}

// Validate checks that the flags are valid.
func (f *Flags) Validate(args []string) error {
	return nil
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "sip"
}

// Init initializes the Scanner with the command-line flags.
func (s *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, _ := flags.(*Flags)
	s.config = f
	return nil
}

// InitPerSender initializes the scanner for a given sender.
func (s *Scanner) InitPerSender(senderID int) error {
	return nil
}

// GetName returns the scanner name defined in the Flags.
func (s *Scanner) GetName() string {
	return s.config.Name
}

// GetTrigger returns the Trigger defined in the Flags.
func (scanner *Scanner) GetTrigger() string {
	return scanner.config.Trigger
}

// Scan performs the SIP scan.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (zgrab2.ScanStatus, interface{}, error) {
	results := &ScanResults{}

	var port uint
	if t.Port != nil {
		port = *t.Port
	} else {
		port = s.config.Port
	}

	var conn net.Conn
	var err error

	if s.config.UseTCP {
		conn, err = s.dialTCP(t.IP, int(port))
		if err != nil {
			return zgrab2.TryGetScanStatus(err), results, fmt.Errorf("error dialing: %w", err)
		}
		if s.config.ImplicitTLS {
			tlsConn, err := s.config.TLSFlags.GetTLSConnection(conn)
			if err != nil {
				return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error setting up TLS connection: %w", err)
			}
			results.TLSLog = tlsConn.GetLog()
			err = tlsConn.Handshake()
			if err != nil {
				return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("TLS handshake failed: %w", err)
			}
			conn = tlsConn
		}
	} else {
		conn, err = s.dialUDP(t.IP, int(port))
		if err != nil {
			return zgrab2.TryGetScanStatus(err), results, fmt.Errorf("error dialing: %w", err)
		}
	}

	defer conn.Close()

	// Craft and send OPTIONS request
	request := s.craftOptionsRequest(t)
	_, err = conn.Write([]byte(request))
	if err != nil {
		return zgrab2.TryGetScanStatus(err), results, fmt.Errorf("error sending OPTIONS request: %w", err)
	}

	// Read response
	buffer := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(time.Duration(s.config.Timeout) * time.Second))
	n, err := conn.Read(buffer)

	// Decode response as UTF-8
	decodedResponse := s.decodeUTF8(buffer[:n])
	results.RawResponse = decodedResponse

	if err != nil {
		// Even if there's an error, we still return the raw response
		return zgrab2.TryGetScanStatus(err), results, fmt.Errorf("error reading response: %w", err)
	}

	s.parseResponse(decodedResponse, results)

	return zgrab2.SCAN_SUCCESS, results, nil
}

// dialUDP establishes a UDP connection.
func (s *Scanner) dialUDP(ip net.IP, port int) (net.Conn, error) {
	return net.DialUDP("udp", nil, &net.UDPAddr{IP: ip, Port: port})
}

// dialTCP establishes a TCP connection.
func (s *Scanner) dialTCP(ip net.IP, port int) (net.Conn, error) {
	return net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip.String(), port), time.Duration(s.config.Timeout)*time.Second)
}

// decodeUTF8 decodes the input bytes as UTF-8, replacing invalid sequences.
func (s *Scanner) decodeUTF8(input []byte) string {
	if utf8.Valid(input) {
		return string(input)
	}

	// If input is not valid UTF-8, replace invalid sequences
	return string(utf8.RuneError)
}

// parseResponse extracts relevant information from the SIP response.
func (s *Scanner) parseResponse(response string, results *ScanResults) {
	lines := strings.Split(response, "\r\n")
	if len(lines) > 0 {
		results.StatusLine = lines[0]
		statusRegex := regexp.MustCompile(`SIP/2.0 (\d{3})`)
		if matches := statusRegex.FindStringSubmatch(lines[0]); len(matches) > 1 {
			results.StatusCode = matches[1]
		}
	}

	results.Headers = make(map[string]string)
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			results.Headers[key] = value

			if strings.ToLower(key) == "allow" {
				results.Methods = strings.Split(value, ",")
				for i, method := range results.Methods {
					results.Methods[i] = strings.TrimSpace(method)
				}
			}
		}
	}
}

// craftOptionsRequest creates a SIP OPTIONS request.
func (s *Scanner) craftOptionsRequest(t zgrab2.ScanTarget) string {
	// Craft the necessary fields
	callID := fmt.Sprintf("%d", time.Now().UnixNano())
	branch := fmt.Sprintf("z9hG4bK-%d", time.Now().UnixNano())
	tag := fmt.Sprintf("%d", time.Now().UnixNano())

	protocol := "UDP"
	if s.config.UseTCP {
		protocol = "TCP"
	}

	// Construct the request
	request := fmt.Sprintf(
		"OPTIONS sip:%s SIP/2.0\r\n"+
			"Via: SIP/2.0/%s %s:%d;branch=%s;rport\r\n"+
			"Max-Forwards: 70\r\n"+
			"To: <sip:%s>\r\n"+
			"From: <sip:zgrab2@localhost>;tag=%s\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: 1 OPTIONS\r\n"+
			"Contact: <sip:zgrab2@localhost>\r\n"+
			"Accept: application/sdp\r\n"+
			"Content-Length: 0\r\n"+
			"User-Agent: zgrab2/0.1\r\n"+
			"\r\n",
		t.Domain,
		protocol, t.IP.String(), t.Port,
		branch,
		t.Domain,
		tag,
		callID,
	)

	return request
}
