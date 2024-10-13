// Package amqp contains the zgrab2 Module implementation for AMQP.
//
// The scan performs a banner grab by sending the AMQP protocol header
// and awaiting the server's response.
//
// The output is the server's response, indicating whether it supports AMQP.
package amqp

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Banner is the initial data banner sent by the server.
	Banner string `json:"banner,omitempty"`

	// TLSLog is the standard shared TLS handshake log.
	// Only present if the TLS flag is set.
	TLSLog *zgrab2.TLSLog `json:"tls,omitempty"`
}

// Flags are the AMQP-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	zgrab2.TLSFlags

	Verbose     bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
	ImplicitTLS bool `long:"implicit-tls" description:"Attempt to connect via a TLS wrapped connection"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface, and holds the state
// for a single scan.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the AMQP server.
type Connection struct {
	config  *Flags
	results ScanResults
	conn    net.Conn
}

// RegisterModule registers the amqp zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("amqp", "AMQP", module.Description(), 5672, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns the default flags object to be filled in with the
// command-line arguments.
func (m *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (m *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description returns an overview of this module.
func (m *Module) Description() string {
	return "Grab an AMQP banner"
}

// Validate flags
func (f *Flags) Validate(args []string) error {
	return nil
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "amqp"
}

// Init initializes the Scanner instance with the flags from the command
// line.
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

// readResponse reads the server's response.
func (amqp *Connection) readResponse() (string, error) {
	buffer := make([]byte, 10000)
	n, err := amqp.conn.Read(buffer)
	if err != nil {
		return "", err
	}
	return string(buffer[:n]), nil
}

// GetAMQPBanner reads the data sent by the server immediately after connecting.
func (amqp *Connection) GetAMQPBanner() (bool, error) {
	// AMQP protocol header
	header := []byte{0x41, 0x4D, 0x51, 0x50, 0x00, 0x01, 0x00, 0x00}
	_, err := amqp.conn.Write(header)
	if err != nil {
		return false, err
	}
	banner, err := amqp.readResponse()
	if err != nil {
		return false, err
	}
	amqp.results.Banner = banner

	// Validate the response
	if len(banner) >= 8 && banner[:4] == "AMQP" {
		return true, nil
	}
	return false, fmt.Errorf("invalid AMQP response: %s", banner)
}

// SetupTLS sets up a TLS connection if the ImplicitTLS flag is set.
func (amqp *Connection) SetupTLS() error {
	tlsConn, err := amqp.config.TLSFlags.GetTLSConnection(amqp.conn)
	if err != nil {
		return fmt.Errorf("error setting up TLS connection: %w", err)
	}
	amqp.results.TLSLog = tlsConn.GetLog()
	err = tlsConn.Handshake()
	if err != nil {
		return fmt.Errorf("TLS handshake failed: %w", err)
	}
	amqp.conn = tlsConn
	return nil
}

// Scan performs the configured scan on the AMQP server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	var err error
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	cn := conn
	defer func() {
		cn.Close()
	}()

	results := ScanResults{}
	amqp := Connection{conn: cn, config: s.config, results: results}

	if s.config.ImplicitTLS {
		if err := amqp.SetupTLS(); err != nil {
			return zgrab2.TryGetScanStatus(err), &amqp.results, err
		}
	}

	isValidBanner, err := amqp.GetAMQPBanner()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &amqp.results, fmt.Errorf("error reading AMQP banner: %w", err)
	}
	if !isValidBanner {
		return zgrab2.TryGetScanStatus(fmt.Errorf("invalid AMQP banner")), &amqp.results, nil
	}
	return zgrab2.SCAN_SUCCESS, &amqp.results, nil
}
