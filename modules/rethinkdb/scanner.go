// Package rethinkdb contains the zgrab2 Module implementation for RethinkDB.
//
// The scan performs a handshake with the RethinkDB server and retrieves the
// initial response message.
package rethinkdb

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Banner is the initial data sent by the server.
	Banner string `json:"banner,omitempty"`

	// HandshakeResponse is the response to the initial handshake message.
	HandshakeResponse string `json:"handshake_response,omitempty"`
}

// Flags are the RethinkDB-specific command-line flags.
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

// Connection holds the state for a single connection to the RethinkDB server.
type Connection struct {
	buffer  [10000]byte
	config  *Flags
	results ScanResults
	conn    net.Conn
}

// RegisterModule registers the rethinkdb zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("rethinkdb", "RethinkDB", module.Description(), 28015, &module)
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
	return "Perform a handshake with a RethinkDB server and retrieve the initial response message."
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
	return "rethinkdb"
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

// readResponse reads a response from the RethinkDB server until null character.
func (rdb *Connection) readResponse() (string, error) {
	var response []byte
	buffer := make([]byte, 1)
	for {
		_, err := rdb.conn.Read(buffer)
		if err != nil {
			return "", err
		}
		if buffer[0] == 0 {
			break
		}
		response = append(response, buffer[0])
	}
	return string(response), nil
}

// sendHandshake sends the initial handshake message to the RethinkDB server.
func (rdb *Connection) sendHandshake() error {
	// Send the magic number: c3 bd c2 34
	magicNumber := []byte{0xc3, 0xbd, 0xc2, 0x34}
	_, err := rdb.conn.Write(magicNumber)
	return err
}

// Scan performs the configured scan on the RethinkDB server, as follows:
//   - Send the initial handshake message.
//   - Read and validate the response.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	var err error
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	rdb := Connection{conn: conn, config: s.config}
	if err := rdb.sendHandshake(); err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error sending handshake: %w", err)
	}

	response, err := rdb.readResponse()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error reading response: %w", err)
	}

	rdb.results.HandshakeResponse = response

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		return zgrab2.SCAN_PROTOCOL_ERROR, &rdb.results, fmt.Errorf("error parsing JSON response: %w", err)
	}

	if strings.Contains(response, "success") {
		return zgrab2.SCAN_SUCCESS, &rdb.results, nil
	}
	return zgrab2.SCAN_APPLICATION_ERROR, &rdb.results, nil
}
