// Package lpd contains the zgrab2 Module implementation for LPD (Line Printer Daemon).
//
// The scan performs a banner grab by sending a "Receive a printer job" command to the LPD service.
//
// The output includes the raw response from the server.

package lpd

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
	"net"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Response is the raw data received from the server.
	Response string `json:"response,omitempty"`

	// IsLPD indicates if the response confirms an LPD service.
	// This field is only included if the get_info flag is not set.
	IsLPD *bool `json:"is_lpd,omitempty"`
}

// Flags are the LPD-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags

	Verbose bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
	GetInfo bool `long:"get_info" description:"If true, send a different command to gather additional information from the LPD service."`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface, and holds the state for a single scan.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the LPD server.
type Connection struct {
	conn    net.Conn
	config  *Flags
	results ScanResults
}

// RegisterModule registers the lpd zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("lpd", "LPD", module.Description(), 515, &module)
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
	return "Grab an LPD banner"
}

// Validate flags
func (f *Flags) Validate(args []string) (err error) {
	// No specific validation required for LPD flags
	return nil
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "lpd"
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

// readResponse reads the LPD response from the server.
func (lpd *Connection) readResponse() (string, error) {
	buffer := make([]byte, 1024)
	n, err := lpd.conn.Read(buffer)
	if err != nil {
		return "", err
	}
	return string(buffer[:n]), nil
}

// sendCommand sends a command to the LPD server.
func (lpd *Connection) sendCommand(cmd string) error {
	_, err := lpd.conn.Write([]byte(cmd + "\n"))
	return err
}

// GetLPDResponse sends the appropriate command based on the flags and captures the response.
func (lpd *Connection) GetLPDResponse() error {
	var command string
	if lpd.config.GetInfo {
		command = "\x03queue:LPT1 \x01"
	} else {
		command = "\x02default"
	}

	err := lpd.sendCommand(command)
	if err != nil {
		return fmt.Errorf("error sending LPD command: %w", err)
	}

	response, err := lpd.readResponse()
	if err != nil {
		return fmt.Errorf("error reading LPD response: %w", err)
	}

	lpd.results.Response = response

	if !lpd.config.GetInfo {
		isLPD := len(response) > 0 && (response[0] == '\x00' || response[0] == '\x01')
		lpd.results.IsLPD = &isLPD
	}

	return nil
}

// Scan performs the configured scan on the LPD server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	var err error
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	lpd := Connection{conn: conn, config: s.config}
	err = lpd.GetLPDResponse()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &lpd.results, fmt.Errorf("error getting LPD response: %w", err)
	}

	return zgrab2.SCAN_SUCCESS, &lpd.results, nil
}
