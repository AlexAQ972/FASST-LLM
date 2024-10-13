package terraria

import (
	"encoding/binary"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// Terraria version string
const terrariaVersion = "Terraria244"

// ScanResults is the output of the scan.
type ScanResults struct {
	ServerResponse string `json:"server_response,omitempty"`
}

// Flags are the Terraria-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	Verbose bool `long:"verbose" description:"More verbose logging"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the Terraria server.
type Connection struct {
	conn    net.Conn
	config  *Flags
	results ScanResults
}

// RegisterModule registers the terraria zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("terraria", "Terraria", module.Description(), 7777, &module)
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
	return "Scan a Terraria server by sending a Connect Request and checking for valid responses."
}

// Init initializes the Scanner instance with the flags from the command line.
func (s *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, _ := flags.(*Flags)
	s.config = f
	return nil
}

// GetName returns the configured name for the Scanner.
func (s *Scanner) GetName() string {
	return s.config.Name
}

// GetTrigger returns the Trigger defined in the Flags.
func (s *Scanner) GetTrigger() string {
	return s.config.Trigger
}

// Help returns this module's help string.
func (f *Flags) Help() string {
	return "This module scans Terraria servers by sending a Connect Request."
}

// Validate ensures that the flag values are valid.
func (f *Flags) Validate(args []string) error {
	return nil
}

// Protocol returns the protocol identifier for the scanner (Terraria).
func (s *Scanner) Protocol() string {
	return "terraria"
}

// InitPerSender does nothing in this module.
func (s *Scanner) InitPerSender(senderID int) error {
	return nil
}

// Scan performs the configured scan on the Terraria server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	terraria := Connection{conn: conn, config: s.config}

	// Step 1: Send the Connect Request
	if err := terraria.SendConnectRequest(); err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error sending Connect Request: %w", err)
	}

	// Step 2: Wait for and validate the response
	if err := terraria.ReadAndValidateResponse(); err != nil {
		return zgrab2.TryGetScanStatus(err), &terraria.results, fmt.Errorf("error reading server response: %w", err)
	}

	// Step 3: Return scan success and results
	return zgrab2.SCAN_SUCCESS, &terraria.results, nil
}

// SendConnectRequest sends a Terraria Connect Request to the server.
func (terraria *Connection) SendConnectRequest() error {
	versionBytes := []byte(terrariaVersion)
	packetLength := uint16(len(versionBytes) + 1 + 2) // +1 for the Packet ID, +2 for the length itself

	packetID := byte(1) // Packet ID for Connect Request

	// Build the packet
	packet := make([]byte, 2+len(versionBytes)+1)       // 2 bytes for length, 1 byte for packet ID
	binary.LittleEndian.PutUint16(packet, packetLength) // First 2 bytes: packet length (including itself)
	packet[2] = packetID                                // Next byte: packet ID
	copy(packet[3:], versionBytes)                      // Remaining bytes: version string

	// Send the packet
	_, err := terraria.conn.Write(packet)
	return err
}

// ReadAndValidateResponse reads the server's response and validates it.
func (terraria *Connection) ReadAndValidateResponse() error {
	buffer := make([]byte, 1024)
	n, err := terraria.conn.Read(buffer)
	if err != nil {
		return err
	}

	response := buffer[:n]
	terraria.results.ServerResponse = string(response)

	// Validate the response based on Terraria protocol
	if err := terraria.ValidateResponse(response); err != nil {
		return err
	}

	return nil
}

// ValidateResponse checks the format of the server's response.
func (terraria *Connection) ValidateResponse(response []byte) error {
	if len(response) < 3 {
		return fmt.Errorf("invalid response length")
	}

	// First two bytes: Packet Length
	packetLength := binary.LittleEndian.Uint16(response[:2])

	// Adjust for protocol differences: We expect the server's packet length to include its own size (2 bytes).
	if int(packetLength) != len(response) {
		return fmt.Errorf("invalid packet length: expected %d, got %d", len(response), packetLength)
	}

	// Third byte: Packet ID
	packetID := response[2]
	switch packetID {
	case 2: // Password Required
		log.Info("Server requires a password.")
	case 3: // Continue Connecting
		log.Info("Server allows continued connection.")
	default:
		return fmt.Errorf("unknown Packet ID: %d", packetID)
	}

	return nil
}
