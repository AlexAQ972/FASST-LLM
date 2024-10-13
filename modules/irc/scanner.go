package irc

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults contains the banner and full server response.
type ScanResults struct {
	Banner      string         `json:"banner,omitempty"`
	Response    string         `json:"response,omitempty"`
	Error       string         `json:"error,omitempty"`
	TLSLog      *zgrab2.TLSLog `json:"tls,omitempty"`
	ImplicitTLS bool           `json:"implicit_tls,omitempty"`
}

// Flags are the IRC-specific flags for the scanning plugin.
type Flags struct {
	zgrab2.BaseFlags
	zgrab2.TLSFlags

	Verbose     bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
	IRCAuthTLS  bool `long:"authtls" description:"Upgrade connection to TLS"`
	ImplicitTLS bool `long:"implicit-tls" description:"Start with a TLS-wrapped connection"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the IRC server.
type Connection struct {
	config  *Flags
	results ScanResults
	conn    net.Conn
	buffer  [10000]byte
}

// RegisterModule registers the IRC zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("irc", "IRC", module.Description(), 6667, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns a new Flags instance filled with default values.
func (m *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (m *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description provides a description of the IRC scanning module.
func (m *Module) Description() string {
	return "Scan for IRC services by sending NICK and USER commands and analyzing the response. Supports upgrading to TLS."
}

// Validate ensures valid flag configuration.
func (f *Flags) Validate(args []string) error {
	if f.IRCAuthTLS && f.ImplicitTLS {
		return fmt.Errorf("Cannot specify both --authtls and --implicit-tls")
	}
	return nil
}

// Help provides help information for the flags.
func (f *Flags) Help() string {
	return "IRC scanning plugin flags"
}

// Init initializes the scanner with command-line flags.
func (s *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, _ := flags.(*Flags)
	s.config = f
	return nil
}

// InitPerSender does nothing in this module.
func (s *Scanner) InitPerSender(senderID int) error {
	return nil
}

// Protocol returns the protocol identifier for the IRC scanner.
func (s *Scanner) Protocol() string {
	return "irc"
}

// GetName returns the scanner name.
func (s *Scanner) GetName() string {
	return "irc"
}

// GetTrigger returns the trigger for the scanner.
func (s *Scanner) GetTrigger() string {
	return s.config.Trigger
}

// isValidIRCResponse checks if the response follows the IRC protocol structure.
func (irc *Connection) isValidIRCResponse(response string) bool {
	// Check if it's a numeric response or a notice
	return strings.Contains(response, "001") || strings.Contains(response, "NOTICE") || strings.Contains(response, "ERROR")
}

// readResponse reads a response from the server.
func (irc *Connection) readResponse() (string, error) {
	irc.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := irc.conn.Read(irc.buffer[:])
	if err != nil {
		return "", err
	}
	return string(irc.buffer[:n]), nil
}

// sendCommand sends a command to the IRC server.
func (irc *Connection) sendCommand(cmd string) error {
	_, err := irc.conn.Write([]byte(cmd + "\r\n"))
	return err
}

// GetIRCBanner reads the initial banner from the server.
func (irc *Connection) GetIRCBanner() error {
	banner, err := irc.readResponse()
	if err != nil {
		return err
	}
	irc.results.Banner = banner
	return nil
}

// RegisterClient sends NICK and USER commands to register the IRC client.
func (irc *Connection) RegisterClient() error {
	if err := irc.sendCommand("NICK ScanningBot"); err != nil {
		return err
	}
	if err := irc.sendCommand("USER ScanningBot 0 * :Scanning Bot"); err != nil {
		return err
	}
	return nil
}

// SetupTLS performs a TLS handshake with the server.
func (irc *Connection) SetupTLS() error {
	var err error
	tlsConn, err := irc.config.TLSFlags.GetTLSConnection(irc.conn)
	if err != nil {
		return fmt.Errorf("error setting up TLS connection: %w", err)
	}
	irc.results.TLSLog = tlsConn.GetLog()

	err = tlsConn.Handshake()
	if err != nil {
		return fmt.Errorf("TLS handshake failed: %w", err)
	}
	irc.conn = tlsConn
	return nil
}

// Scan performs the IRC scan.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, err error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	cn := conn
	defer func() {
		cn.Close()
	}()

	results := ScanResults{}

	// If implicit TLS is specified, wrap the connection in TLS from the start
	if s.config.ImplicitTLS {
		tlsConn, err := s.config.TLSFlags.GetTLSConnection(conn)
		if err != nil {
			return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error setting up TLS connection: %w", err)
		}
		results.ImplicitTLS = true
		results.TLSLog = tlsConn.GetLog()
		err = tlsConn.Handshake()
		if err != nil {
			return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("TLS handshake failed: %w", err)
		}
		cn = tlsConn
	}

	irc := Connection{conn: cn, config: s.config, results: results}

	// Get the initial banner
	if err := irc.GetIRCBanner(); err != nil {
		return zgrab2.TryGetScanStatus(err), &irc.results, fmt.Errorf("error reading IRC banner: %w", err)
	}

	// If the --authtls flag is set, upgrade to TLS
	if s.config.IRCAuthTLS {
		if err := irc.SetupTLS(); err != nil {
			return zgrab2.TryGetScanStatus(err), &irc.results, fmt.Errorf("error setting up TLS: %w", err)
		}
	}

	// Register the client with NICK and USER commands
	if err := irc.RegisterClient(); err != nil {
		return zgrab2.TryGetScanStatus(err), &irc.results, fmt.Errorf("error sending registration commands: %w", err)
	}

	// Read and validate the server response
	response, err := irc.readResponse()
	if err != nil {
		irc.results.Error = fmt.Sprintf("Failed to read server response: %v", err)
		return zgrab2.SCAN_SUCCESS, &irc.results, nil
	}

	irc.results.Response = response
	if irc.isValidIRCResponse(response) {
		return zgrab2.SCAN_SUCCESS, &irc.results, nil
	}

	return zgrab2.SCAN_PROTOCOL_ERROR, &irc.results, fmt.Errorf("invalid IRC response")
}
