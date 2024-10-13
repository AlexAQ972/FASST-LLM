package rtsp

import (
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan, including the OPTIONS response.
type ScanResults struct {
	// OptionsResponse captures the response to the OPTIONS request.
	OptionsResponse string `json:"options_response,omitempty"`
}

// Flags define RTSP-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	Verbose bool `long:"verbose" description:"More verbose logging"`
}

// Module implements the zgrab2.Module interface for RTSP.
type Module struct{}

// Scanner implements the zgrab2.Scanner interface.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the RTSP server.
type Connection struct {
	conn    net.Conn
	config  *Flags
	results ScanResults
}

// RegisterModule registers the RTSP module with zgrab2.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("rtsp", "RTSP", module.Description(), 554, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns the default flag settings.
func (m *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (m *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description provides an overview of this module.
func (m *Module) Description() string {
	return "Grab an RTSP OPTIONS response"
}

// Validate flags.
func (f *Flags) Validate(args []string) error {
	return nil
}

// Help provides module-specific help.
func (f *Flags) Help() string {
	return ""
}

// Protocol returns the protocol identifier.
func (s *Scanner) Protocol() string {
	return "rtsp"
}

// Init initializes the Scanner instance.
func (s *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, _ := flags.(*Flags)
	s.config = f
	return nil
}

// InitPerSender does nothing in this module.
func (s *Scanner) InitPerSender(senderID int) error {
	return nil
}

// GetName returns the Scanner name.
func (s *Scanner) GetName() string {
	return s.config.Name
}

// GetTrigger returns the trigger, if set.
func (scanner *Scanner) GetTrigger() string {
	return scanner.config.Trigger
}

// readResponse reads a response from the server.
func (rtsp *Connection) readResponse() (string, error) {
	buffer := make([]byte, 4096)
	n, err := rtsp.conn.Read(buffer)
	if err != nil {
		return "", err
	}
	return string(buffer[:n]), nil
}

// sendOptionsRequest sends the OPTIONS request and returns the response.
func (rtsp *Connection) sendOptionsRequest() (string, error) {
	optionsRequest := "OPTIONS * RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: RTSPScanner/1.0\r\n\r\n"
	_, err := rtsp.conn.Write([]byte(optionsRequest))
	if err != nil {
		return "", err
	}
	return rtsp.readResponse()
}

// Scan performs the actual RTSP scan.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, err error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	rtsp := &Connection{conn: conn, config: s.config}
	response, err := rtsp.sendOptionsRequest()
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error sending OPTIONS request: %w", err)
	}

	rtsp.results.OptionsResponse = response
	// Consider the scan successful as long as there is a valid RTSP response
	if strings.HasPrefix(response, "RTSP/1.0") {
		return zgrab2.SCAN_SUCCESS, &rtsp.results, nil
	}
	return zgrab2.SCAN_APPLICATION_ERROR, &rtsp.results, fmt.Errorf("unexpected RTSP response: %s", response)
}
