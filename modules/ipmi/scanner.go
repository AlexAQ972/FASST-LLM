// Package ipmi contains the zgrab2 Module implementation for IPMI.
package ipmi

import (
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	PingSent     string `json:"ping_sent,omitempty"`
	PongReceived string `json:"pong_received,omitempty"`
}

// Flags are the IPMI-specific command-line flags.
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

// RegisterModule registers the ipmi zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("ipmi", "IPMI", module.Description(), 623, &module)
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
	return "Scan for IPMI (remote framebuffer) support"
}

// Validate flags
func (f *Flags) Validate(args []string) (err error) {
	return
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "ipmi"
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

// IPMI Ping and Pong messages
const (
	IPMIPingMessage = "\x06\x00\xFF\x06\x00\x00\x11\xBE\x80\x00\x00\x04"
)

// Scan performs the configured scan on the IPMI server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	var err error

	// Determine which port to connect to: target.Port or scanner.config.BaseFlags.Port
	var port uint
	if t.Port != nil {
		port = *t.Port // Dereference target.Port if set
	} else {
		port = s.config.BaseFlags.Port // Use scanner's configured default port
	}

	// Establish the connection to the server
	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", t.IP.String(), port))
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	results := ScanResults{}

	// Send the IPMI Ping message
	pingMsg := []byte(IPMIPingMessage)
	_, err = conn.Write(pingMsg)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &results, fmt.Errorf("error sending IPMI Ping: %w", err)
	}
	results.PingSent = fmt.Sprintf("%x", pingMsg)

	// Set a timeout for the response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Receive the IPMI Pong message
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &results, fmt.Errorf("error reading IPMI Pong: %w", err)
	}
	results.PongReceived = fmt.Sprintf("%x", buf[:n])

	return zgrab2.SCAN_SUCCESS, &results, nil
}
