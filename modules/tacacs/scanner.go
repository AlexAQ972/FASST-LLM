// Package tacacs contains the zgrab2 Module implementation for TACACS+.
package tacacs

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Banner is the initial data banner sent by the server.
	Banner string `json:"banner,omitempty"`

	// RawResp is the raw response from the TACACS+ server.
	RawResp []byte `json:"raw_resp,omitempty"`
}

// Flags are the TACACS+-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags

	Verbose bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface, and holds the state
// for a single scan.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the TACACS+ server.
type Connection struct {
	conn    net.Conn
	config  *Flags
	results ScanResults
}

// RegisterModule registers the TACACS+ zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("tacacs", "TACACS+", module.Description(), 49, &module)
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
	return "Perform a TACACS+ handshake and retrieve the server's response"
}

// Validate flags
func (f *Flags) Validate(args []string) (err error) {
	return nil
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "tacacs"
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

// sendTACACSPacket constructs and sends a minimal TACACS+ Authentication START packet.
func (c *Connection) sendTACACSPacket() ([]byte, error) {
	// Construct the TACACS+ Authentication START packet
	packet := make([]byte, 12) // 12 bytes for the header

	// Header fields
	packet[0] = 0xc0                                      // major_version
	packet[1] = 0x01                                      // type (Authentication)
	packet[2] = 0x01                                      // seq_no
	packet[3] = 0x00                                      // flags
	binary.BigEndian.PutUint32(packet[4:], rand.Uint32()) // session_id
	binary.BigEndian.PutUint32(packet[8:], uint32(0))     // length (0 for now)

	// Send the packet
	_, err := c.conn.Write(packet)
	if err != nil {
		return nil, err
	}

	// Read the response
	response := make([]byte, 1024)
	n, err := c.conn.Read(response)
	if err != nil {
		return nil, err
	}

	return response[:n], nil
}

// Scan performs the configured scan on the TACACS+ server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	c := Connection{conn: conn, config: s.config}

	// Send the TACACS+ packet
	response, err := c.sendTACACSPacket()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &c.results, fmt.Errorf("error sending TACACS+ packet: %w", err)
	}

	// Log the server response
	c.results.RawResp = response
	return zgrab2.SCAN_SUCCESS, &c.results, nil
}
