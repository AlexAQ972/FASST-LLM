// Package bolt contains the zgrab2 Module implementation for Bolt.
package bolt

import (
	"encoding/binary"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Initial identification bytes sent by the server.
	Identification string `json:"identification,omitempty"`

	// Server response to the version negotiation message.
	VersionResponse string `json:"version_response,omitempty"`

	// ProtocolVersion is the version of the Bolt protocol supported by the server.
	ProtocolVersion uint32 `json:"protocol_version,omitempty"`
}

// Flags are the Bolt-specific command-line flags.
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

// Connection holds the state for a single connection to the Bolt server.
type Connection struct {
	config  *Flags
	results ScanResults
	conn    net.Conn
}

// RegisterModule registers the bolt zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("bolt", "Bolt", module.Description(), 7687, &module)
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
	return "Scan for Bolt protocol support"
}

// Validate flags
func (f *Flags) Validate(args []string) (err error) {
	return
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifer for the scanner.
func (s *Scanner) Protocol() string {
	return "bolt"
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

// sendBytes sends a byte slice to the server.
func (conn *Connection) sendBytes(data []byte) error {
	_, err := conn.conn.Write(data)
	return err
}

// readBytes reads a specified number of bytes from the server.
func (conn *Connection) readBytes(numBytes int) ([]byte, error) {
	buffer := make([]byte, numBytes)
	_, err := conn.conn.Read(buffer)
	return buffer, err
}

// establishConnection establishes a TCP connection to the target.
func (conn *Connection) establishConnection(target zgrab2.ScanTarget) error {
	c, err := target.Open(&conn.config.BaseFlags)
	if err != nil {
		return fmt.Errorf("error opening connection: %w", err)
	}
	conn.conn = c
	return nil
}

// Scan performs the configured scan on the Bolt server, as follows:
//   - Establish a TCP connection
//   - Send the identification bytes
//   - Send the version negotiation message
//   - Receive and validate the server response
//   - Output the results
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	var err error
	conn := &Connection{config: s.config}

	if err = conn.establishConnection(t); err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error establishing connection: %w", err)
	}
	defer conn.conn.Close()

	// Send identification bytes
	identification := []byte{0x60, 0x60, 0xB0, 0x17}
	if err = conn.sendBytes(identification); err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error sending identification bytes: %w", err)
	}
	conn.results.Identification = fmt.Sprintf("%x", identification)

	// Send version negotiation message
	versionMsg := []byte{
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x03,
		0x00, 0x00, 0x00, 0x00,
	}
	if err = conn.sendBytes(versionMsg); err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error sending version negotiation message: %w", err)
	}

	// Receive and validate server response
	response, err := conn.readBytes(4)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error reading server response: %w", err)
	}
	conn.results.VersionResponse = fmt.Sprintf("%x", response)

	// Interpret the response as a 32-bit integer
	if len(response) != 4 {
		return zgrab2.SCAN_PROTOCOL_ERROR, &conn.results, fmt.Errorf("invalid response length: expected 4 bytes, got %d", len(response))
	}
	version := binary.BigEndian.Uint32(response)
	conn.results.ProtocolVersion = version

	if version == 0 {
		return zgrab2.SCAN_APPLICATION_ERROR, &conn.results, fmt.Errorf("invalid protocol version: %d", version)
	}

	return zgrab2.SCAN_SUCCESS, &conn.results, nil
}
