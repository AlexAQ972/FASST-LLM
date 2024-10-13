package ldap

import (
	"bytes"
	"fmt"

	"github.com/go-asn1-ber/asn1-ber"
	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults contains the output of the LDAP scan.
type ScanResults struct {
	BindResponse string `json:"bind_response,omitempty"`
	RawResponse  string `json:"raw_response,omitempty"`
	TLSLog *zgrab2.TLSLog `json:"tls,omitempty"`
}

// Flags contains LDAP-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	zgrab2.TLSFlags

	Verbose bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
	ImplicitTLS bool `long:"tls" description:"Attempt to connect via a TLS wrapped connection"`
}

// Module implements the zgrab2.Module interface.
type Module struct{}

// Scanner implements the zgrab2.Scanner interface.
type Scanner struct {
	config *Flags
}

// RegisterModule registers the ldap zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("ldap", "LDAP", module.Description(), 389, &module)
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
	return "Probe for LDAP servers"
}

// Validate checks the flags for consistency.
func (f *Flags) Validate(args []string) error {
	return nil
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "ldap"
}

// Init initializes the Scanner with the command-line flags.
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

// Scan performs the LDAP scan.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	results := ScanResults{}
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

	// Construct a simple BindRequest
	bindRequest := []byte{
		0x30, 0x0c, // SEQUENCE
		0x02, 0x01, 0x01, // INTEGER (1) - messageID
		0x60, 0x07, // [APPLICATION 0] - bindRequest
		0x02, 0x01, 0x03, // INTEGER (3) - version
		0x04, 0x00, // OCTET STRING (0) - name (empty)
		0x80, 0x00, // [0] - simple auth (empty)
	}

	_, err = conn.Write(bindRequest)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &results, fmt.Errorf("error sending BindRequest: %w", err)
	}

	// Read the response
	packet, err := ber.ReadPacket(conn)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &results, fmt.Errorf("error reading BindResponse: %w", err)
	}

	results.RawResponse = ber.DecodeString(packet.Bytes())

	var buffer bytes.Buffer
	ber.WritePacket(&buffer, packet)
	results.BindResponse = buffer.String()

	// Validate the response
	if len(packet.Children) < 2 {
		return zgrab2.SCAN_UNKNOWN_ERROR, &results, fmt.Errorf("unexpected response format")
	}

	// Extract the resultCode
	resultCode, ok := packet.Children[1].Children[0].Value.(int64)
	if !ok {
		return zgrab2.SCAN_UNKNOWN_ERROR, &results, fmt.Errorf("error parsing resultCode")
	}

	// Check if the resultCode indicates a valid LDAP response
	if resultCode >= 0 && resultCode <= 80 { // Valid LDAP result codes
		return zgrab2.SCAN_SUCCESS, &results, nil
	}

	return zgrab2.SCAN_PROTOCOL_ERROR, &results, fmt.Errorf("unexpected LDAP result code: %d", resultCode)
}

// GetTrigger returns the Trigger defined in the Flags.
func (s *Scanner) GetTrigger() string {
	return s.config.Trigger
}
