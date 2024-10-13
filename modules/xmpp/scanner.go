package xmpp

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults holds the output of the scan.
type ScanResults struct {
	// Banner is the server's response to the XMPP stream header.
	Banner string `json:"banner,omitempty"`

	ImplicitTLS bool           `json:"implicit_tls,omitempty"`
	TLSLog      *zgrab2.TLSLog `json:"tls,omitempty"`
}

// Flags are the XMPP-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	zgrab2.TLSFlags

	Verbose     bool `long:"verbose" description:"More verbose logging"`
	ImplicitTLS bool `long:"tls" description:"Attempt to connect via a TLS wrapped connection"`
}

// Module implements the zgrab2.Module interface.
type Module struct{}

// Scanner implements the zgrab2.Scanner interface.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the XMPP server.
type Connection struct {
	buffer  [10000]byte
	config  *Flags
	results ScanResults
	conn    net.Conn
}

// RegisterModule registers the XMPP zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("xmpp", "XMPP", module.Description(), 5222, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns the default flags object to be filled in with command-line arguments.
func (m *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (m *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description returns an overview of this module.
func (m *Module) Description() string {
	return "Scan for an XMPP service by initiating an XMPP connection"
}

// Validate ensures that the flags provided are valid.
func (f *Flags) Validate(args []string) error {
	return nil
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "xmpp"
}

// Init initializes the Scanner instance with the flags from the command line.
func (s *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, _ := flags.(*Flags)
	s.config = f
	return nil
}

// InitPerSender does nothing in this module.
func (s *Scanner) InitPerSender(senderID int) error {
	return nil
}

// GetName returns the configured name for the Scanner.
func (s *Scanner) GetName() string {
	return s.config.Name
}

// GetTrigger returns the Trigger defined in the Flags.
func (scanner *Scanner) GetTrigger() string {
	return scanner.config.Trigger
}

// xmppEndRegex matches the end of an XML response.
var xmppEndRegex = regexp.MustCompile(`</stream:stream>`)

// readResponse reads the XMPP response from the server.
func (conn *Connection) readResponse() (string, error) {
	respLen, err := zgrab2.ReadUntilRegex(conn.conn, conn.buffer[:], xmppEndRegex)
	if err != nil {
		return "", err
	}
	return string(conn.buffer[0:respLen]), nil
}

// sendStreamHeader sends the XMPP stream header to initiate communication without xmlns.
func (conn *Connection) sendStreamHeader(from, to string) error {
	header := fmt.Sprintf(
		"<?xml version='1.0'?><stream:stream from='%s' to='%s' version='1.0' xml:lang='en'>",
		from, to)
	_, err := conn.conn.Write([]byte(header))
	return err
}

// GetXMPPBanner sends the initial stream header and reads the server's response.
func (conn *Connection) GetXMPPBanner(from, to string) (bool, error) {
	// Send the initial stream header
	if err := conn.sendStreamHeader(from, to); err != nil {
		return false, fmt.Errorf("error sending XMPP stream header: %w", err)
	}

	// Read the server's response
	banner, err := conn.readResponse()
	if err != nil {
		return false, fmt.Errorf("error reading XMPP response: %w", err)
	}
	conn.results.Banner = banner

	// Check if the response includes any valid stream element
	return strings.Contains(banner, "<stream:stream"), nil
}

// Scan performs the XMPP scan.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	// Open a connection to the target
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	cn := conn
	defer cn.Close()

	results := ScanResults{}

	if s.config.ImplicitTLS {
		tlsConn, err := s.config.TLSFlags.GetTLSConnection(conn)
		if err != nil {
			return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error setting up TLS connection: %w", err)
		}
		results.ImplicitTLS = true
		results.TLSLog = tlsConn.GetLog()
		err = tlsConn.Handshake()
		if err != nil {
			return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("TLS handshake failed: %w", err)
		}
		cn = tlsConn
	}

	xmpp := Connection{conn: cn, config: s.config, results: results}

	// Send the XMPP stream header and check the banner
	success, err := xmpp.GetXMPPBanner("scanner-ip", t.IP.String())
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &xmpp.results, fmt.Errorf("error during XMPP banner grab: %w", err)
	}

	// If there's any valid information (even if an error), return SCAN_SUCCESS
	if success {
		return zgrab2.SCAN_SUCCESS, &xmpp.results, nil
	}

	// Otherwise, log the server response and return the error
	return zgrab2.SCAN_UNKNOWN_ERROR, &xmpp.results, nil
}
