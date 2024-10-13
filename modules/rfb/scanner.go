// Package rfb contains the zgrab2 Module implementation for RFB (Remote Framebuffer).
//
// The scan performs a banner grab and validates the received banner.
//
// The output is the banner and a validation status.
package rfb

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
	"net"
	"regexp"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	// Banner is the initial data banner sent by the server.
	Banner string `json:"banner,omitempty"`
}

// Flags are the RFB-specific command-line flags.
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

// Connection holds the state for a single connection to the RFB server.
type Connection struct {
	buffer  [10000]byte
	config  *Flags
	results ScanResults
	conn    net.Conn
}

// RegisterModule registers the rfb zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("rfb", "RFB", module.Description(), 5900, &module)
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
	return "Grab an RFB banner"
}

// Validate flags
func (f *Flags) Validate(args []string) error {
	return nil
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifer for the scanner.
func (s *Scanner) Protocol() string {
	return "rfb"
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

// rfbBannerRegex matches the RFB banner format "RFB xxx.yyy\n".
var rfbBannerRegex = regexp.MustCompile(`^RFB (\d{3})\.(\d{3})\n$`)

// readResponse reads the RFB banner from the server.
func (rfb *Connection) readResponse() (string, error) {
	respLen, err := zgrab2.ReadUntilRegex(rfb.conn, rfb.buffer[:], rfbBannerRegex)
	if err != nil {
		return "", err
	}
	ret := string(rfb.buffer[0:respLen])
	return ret, nil
}

// GetRFBanner reads the data sent by the server immediately after connecting.
// Returns true if and only if the server returns a valid RFB banner.
func (rfb *Connection) GetRFBanner() (bool, error) {
	banner, err := rfb.readResponse()
	if err != nil {
		return false, err
	}
	rfb.results.Banner = banner
	return rfbBannerRegex.MatchString(banner), nil
}

// Scan performs the configured scan on the RFB server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	var err error
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	results := ScanResults{}
	rfb := Connection{conn: conn, config: s.config, results: results}

	isValidBanner, err := rfb.GetRFBanner()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), &rfb.results, fmt.Errorf("error reading RFB banner: %w", err)
	}
	if !isValidBanner {
		return zgrab2.SCAN_APPLICATION_ERROR, &rfb.results, fmt.Errorf("invalid RFB banner: %s", rfb.results.Banner)
	}
	return zgrab2.SCAN_SUCCESS, &rfb.results, nil
}
