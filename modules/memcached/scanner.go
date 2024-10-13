// Package memcached contains the zgrab2 Module implementation for Memcached.
package memcached

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
	"net"
	"regexp"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Version is the version string returned by the server.
	Version string `json:"version,omitempty"`
}

// Flags are the Memcached-specific command-line flags.
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

// Connection holds the state for a single connection to the Memcached server.
type Connection struct {
	config  *Flags
	results ScanResults
	conn    net.Conn
}

// RegisterModule registers the Memcached zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("memcached", "Memcached", module.Description(), 11211, &module)
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
	return "Grab a Memcached version"
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
	return "memcached"
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

// memcachedEndRegex matches zero or more lines followed by "VERSION" and the version string.
var memcachedEndRegex = regexp.MustCompile(`^VERSION (\S+)\r\n$`)

// readResponse reads a response from the server.
func (mem *Connection) readResponse() (string, error) {
	buffer := make([]byte, 256)
	n, err := mem.conn.Read(buffer)
	if err != nil {
		return "", err
	}
	return string(buffer[:n]), nil
}

// sendCommand sends a command and waits for / reads / returns the response.
func (mem *Connection) sendCommand(cmd string) (string, error) {
	mem.conn.Write([]byte(cmd + "\r\n"))
	return mem.readResponse()
}

// GetMemcachedVersion sends the version command to the server and reads the response.
func (mem *Connection) GetMemcachedVersion() error {
	resp, err := mem.sendCommand("version")
	if err != nil {
		return err
	}
	matches := memcachedEndRegex.FindStringSubmatch(resp)
	if len(matches) < 2 {
		return fmt.Errorf("invalid response: %s", resp)
	}
	mem.results.Version = matches[1]
	return nil
}

// Scan performs the configured scan on the Memcached server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	mem := Connection{conn: conn, config: s.config}
	err = mem.GetMemcachedVersion()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &mem.results, fmt.Errorf("error getting memcached version: %w", err)
	}
	return zgrab2.SCAN_SUCCESS, &mem.results, nil
}
