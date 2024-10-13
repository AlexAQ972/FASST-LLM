// Package x11 implements the zgrab2 Module for scanning X11 services.
package x11

import (
	"encoding/binary"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Banner holds the initial response from the X11 server.
	Banner string `json:"banner,omitempty"`

	// ServerMajorVersion holds the major version of the X11 protocol returned by the server.
	ServerMajorVersion uint16 `json:"server_major_version,omitempty"`

	// ServerMinorVersion holds the minor version of the X11 protocol returned by the server.
	ServerMinorVersion uint16 `json:"server_minor_version,omitempty"`

	// ByteOrder indicates whether the server is using little-endian (0x6C) or big-endian (0x42).
	ByteOrder string `json:"byte_order,omitempty"`

	// Error holds any errors encountered during the scan.
	Error string `json:"error,omitempty"`
}

// Flags define the X11-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	Verbose bool `long:"verbose" description:"More verbose logging"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface, and holds the state
// for a single scan.
type Scanner struct {
	config    *Flags
	byteOrder byte // Byte order used for the scan (0x42 for big-endian or 0x6C for little-endian)
}

// Connection holds the state for a single connection to the X11 server.
type Connection struct {
	config    *Flags
	results   ScanResults
	conn      net.Conn
	byteOrder byte // Byte order used for parsing the response
}

// RegisterModule registers the X11 zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("x11", "X11", module.Description(), 6000, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns the default flags object.
func (m *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (m *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description returns a short description of this module.
func (m *Module) Description() string {
	return "Scan for X11 services"
}

// GetName returns the name of the scan target. This implements the zgrab2.Scanner interface.
func (s *Scanner) GetName() string {
	return "x11"
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "x11"
}

// Init initializes the Scanner with the provided flags.
func (s *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, _ := flags.(*Flags) // Use type assertion to retrieve the configuration flags
	s.config = f
	s.byteOrder = 0x6C // Default to little-endian for this example. You can make this configurable.
	return nil
}

// InitPerSender does nothing in this module.
func (s *Scanner) InitPerSender(senderID int) error {
	return nil
}

// GetTrigger returns an empty trigger string, as no specific trigger is used.
func (s *Scanner) GetTrigger() string {
	return ""
}

// Scan performs the X11 scan.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	x11 := Connection{conn: conn, config: s.config, byteOrder: s.byteOrder, results: ScanResults{}}

	// Send initial X11 message
	if err := x11.SendInitialMessage(); err != nil {
		return zgrab2.TryGetScanStatus(err), &x11.results, fmt.Errorf("error sending initial X11 message: %w", err)
	}

	// Read and verify the server response
	if err := x11.ReadServerResponse(); err != nil {
		return zgrab2.TryGetScanStatus(err), &x11.results, fmt.Errorf("error reading X11 response: %w", err)
	}

	return zgrab2.SCAN_SUCCESS, &x11.results, nil
}

// SendInitialMessage constructs and sends the initial X11 connection message.
func (x11 *Connection) SendInitialMessage() error {
	// Construct the X11 initial message (based on the byte order set by the scanner)
	message := make([]byte, 12)

	// Byte-order Byte: use the byte order set in the scanner (little-endian or big-endian)
	message[0] = x11.byteOrder

	// Protocol Major Version: 16-bit value 0x000B (X11)
	if x11.byteOrder == 0x42 { // Big-endian
		binary.BigEndian.PutUint16(message[2:], 0x000B)
		binary.BigEndian.PutUint16(message[4:], 0x0000)
	} else { // Little-endian
		binary.LittleEndian.PutUint16(message[2:], 0x000B)
		binary.LittleEndian.PutUint16(message[4:], 0x0000)
	}

	// Authorization Name and Data: Leave empty (0 length)
	// Authorization Name Length: 16-bit value 0x0000
	// Authorization Data Length: 16-bit value 0x0000

	// Send the message
	_, err := x11.conn.Write(message)
	return err
}

// ReadServerResponse reads and parses the server response to the initial X11 message.
func (x11 *Connection) ReadServerResponse() error {
	// Read the server's response (assuming a basic buffer size)
	buf := make([]byte, 1024)
	n, err := x11.conn.Read(buf)
	if err != nil {
		return err
	}

	// Save the response banner
	x11.results.Banner = string(buf[:n])

	// Parse the response and check the protocol version based on the byte order
	var majorVersion, minorVersion uint16
	if x11.byteOrder == 0x42 { // big-endian
		majorVersion = binary.BigEndian.Uint16(buf[2:4])
		minorVersion = binary.BigEndian.Uint16(buf[4:6])
	} else { // little-endian
		majorVersion = binary.LittleEndian.Uint16(buf[2:4])
		minorVersion = binary.LittleEndian.Uint16(buf[4:6])
	}

	x11.results.ServerMajorVersion = majorVersion
	x11.results.ServerMinorVersion = minorVersion

	// Check if the protocol version is 11 (X11 protocol)
	if majorVersion != 11 {
		x11.results.Error = fmt.Sprintf("unexpected X11 major version: %d", majorVersion)
		// We still consider this a valid response, so the scan is successful.
		return nil
	}

	// Handle a successful connection (status byte = 1)
	if buf[1] == 1 {
		// Success response; protocol version is valid.
		return nil
	}

	// If we reach here, the response is likely an authorization failure or other error.
	x11.results.Error = fmt.Sprintf("X11 connection failed or unauthorized: %s", x11.results.Banner)

	// Treat the scan as successful since we got a valid protocol response, even if it's a failure.
	return nil
}

// Help implements the zgrab2.ScanFlags interface for Flags. It returns the help string for the module.
func (f *Flags) Help() string {
	return "Flags for X11 scanner"
}

// Validate implements the zgrab2.ScanFlags interface, validating the flags.
func (f *Flags) Validate(args []string) error {
	return nil
}
