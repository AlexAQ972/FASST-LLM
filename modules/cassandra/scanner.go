// Package cassandra contains the zgrab2 Module implementation for Cassandra protocol.
package cassandra

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	Banner string `json:"banner,omitempty"`
}

// Flags are the Cassandra-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	Verbose bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface, and holds the state for a single scan.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the Cassandra server.
type Connection struct {
	buffer  [4096]byte
	config  *Flags
	results ScanResults
	conn    net.Conn
}

// RegisterModule registers the cassandra zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("cassandra", "Cassandra", module.Description(), 9042, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns the default flags object to be filled in with the command-line arguments.
func (m *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (m *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description returns an overview of this module.
func (m *Module) Description() string {
	return "Grab a Cassandra banner"
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
	return "cassandra"
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

// readResponse reads a response from the server.
func (conn *Connection) readResponse() (string, error) {
	respLen, err := zgrab2.ReadAvailable(conn.conn)
	if err != nil {
		return "", err
	}
	ret := string(respLen)
	return ret, nil
}

// sendStartupMessage sends the STARTUP message to the server.
func (conn *Connection) sendStartupMessage() error {
	body := map[string]string{"CQL_VERSION": "3.0.0"}
	bodyBuf := new(bytes.Buffer)
	binary.Write(bodyBuf, binary.BigEndian, uint16(len(body)))
	for key, value := range body {
		binary.Write(bodyBuf, binary.BigEndian, uint16(len(key)))
		bodyBuf.WriteString(key)
		binary.Write(bodyBuf, binary.BigEndian, uint16(len(value)))
		bodyBuf.WriteString(value)
	}

	frame := new(bytes.Buffer)
	frame.WriteByte(0x04)                            // version
	frame.WriteByte(0x00)                            // flags
	binary.Write(frame, binary.BigEndian, uint16(0)) // stream
	frame.WriteByte(0x01)                            // opcode (STARTUP)
	binary.Write(frame, binary.BigEndian, uint32(bodyBuf.Len()))
	frame.Write(bodyBuf.Bytes())

	_, err := conn.conn.Write(frame.Bytes())
	return err
}

// Scan performs the configured scan on the Cassandra server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	var err error
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	cn := &Connection{
		conn:   conn,
		config: s.config,
	}

	err = cn.sendStartupMessage()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error sending STARTUP message: %w", err)
	}

	response, err := cn.readResponse()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error reading response: %w", err)
	}

	cn.results.Banner = response
	return zgrab2.SCAN_SUCCESS, &cn.results, nil
}
