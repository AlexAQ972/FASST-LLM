package mqtt

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// ScanResults is the output of the scan.
type ScanResults struct {
	SessionPresent    bool           `json:"session_present,omitempty"`
	ConnectReturnCode byte           `json:"connect_return_code,omitempty"`
	Response          string         `json:"response,omitempty"`
	TLSLog            *zgrab2.TLSLog `json:"tls,omitempty"`
}

// Flags are the MQTT-specific command-line flags.
type Flags struct {
	zgrab2.BaseFlags
	zgrab2.TLSFlags

	Verbose  bool   `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
	V5       bool   `long:"v5" description:"Scanning MQTT v5.0. Otherwise scanning MQTT v3.1.1"`
	UseTLS   bool   `long:"tls" description:"Use TLS for the MQTT connection"`
	Username string `long:"username" description:"Username for MQTT authentication"`
	Password string `long:"password" description:"Password for MQTT authentication"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface, and holds the state
// for a single scan.
type Scanner struct {
	config *Flags
}

// Connection holds the state for a single connection to the MQTT server.
type Connection struct {
	conn    net.Conn
	config  *Flags
	results ScanResults
}

// RegisterModule registers the MQTT zgrab2 module.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("mqtt", "MQTT", module.Description(), 1883, &module)
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
	return "Perform an MQTT scan"
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
	return "mqtt"
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

// SendMQTTConnectPacket constructs and sends an MQTT CONNECT packet to the server.
func (mqtt *Connection) SendMQTTConnectPacket(v5 bool) error {
	var packet []byte

	if v5 {
		// MQTT v5 CONNECT packet (unchanged from the original)
		packet = []byte{
			// Fixed Header
			0x10, // Control Packet Type (CONNECT) and flags
			0x17, // Remaining Length (23 bytes)

			// Variable Header
			0x00, 0x04, 'M', 'Q', 'T', 'T', // Protocol Name
			0x05,       // Protocol Level (MQTT v5.0)
			0x02,       // Connect Flags (Clean Start)
			0x00, 0x3C, // Keep Alive (60 seconds)

			// Properties
			0x00, // Properties Length (0)

			// Payload
			0x00, 0x0A, 'M', 'Q', 'T', 'T', 'C', 'l', 'i', 'e', 'n', 't', // Client Identifier
		}
	} else {
		// MQTT v3.1.1 Connect Packet with Username and Password
		usernameFlag := 0
		passwordFlag := 0
		var usernameField []byte
		var passwordField []byte
		clientID := "MQTTClient" // Replace with the actual client identifier

		if mqtt.config.Username != "" {
			usernameFlag = 1
			usernameField = []byte(mqtt.config.Username)
		}

		if mqtt.config.Password != "" {
			passwordFlag = 1
			passwordField = []byte(mqtt.config.Password)
		}

		// Calculate the remaining length
		remainingLength := 10 + 2 + len(clientID) // 10 bytes for fixed fields + 2 bytes for clientID length + clientID length
		if usernameFlag == 1 {
			remainingLength += 2 + len(usernameField) // Add 2 bytes for username length + username length
		}
		if passwordFlag == 1 {
			remainingLength += 2 + len(passwordField) // Add 2 bytes for password length + password length
		}

		connectFlags := byte(0x02) // Clean Start flag

		if usernameFlag == 1 {
			connectFlags |= 0x80 // Set the Username flag
		}
		if passwordFlag == 1 {
			connectFlags |= 0x40 // Set the Password flag
		}

		packet = []byte{
			// Fixed Header
			0x10, byte(remainingLength), // Control Packet Type (CONNECT) and remaining length

			// Variable Header
			0x00, 0x04, 'M', 'Q', 'T', 'T', // Protocol Name
			0x04,         // Protocol Level (MQTT v3.1.1)
			connectFlags, // Connect Flags (set Username and Password flags accordingly)
			0x00, 0x3C,   // Keep Alive (60 seconds)

			// Payload
			0x00, 0x0A, 'M', 'Q', 'T', 'T', 'C', 'l', 'i', 'e', 'n', 't', // Client Identifier
		}

		// Add Username to Payload
		if usernameFlag == 1 {
			usernameLen := make([]byte, 2)
			binary.BigEndian.PutUint16(usernameLen, uint16(len(usernameField)))
			packet = append(packet, usernameLen...)
			packet = append(packet, usernameField...)
		}

		// Add Password to Payload
		if passwordFlag == 1 {
			passwordLen := make([]byte, 2)
			binary.BigEndian.PutUint16(passwordLen, uint16(len(passwordField)))
			packet = append(packet, passwordLen...)
			packet = append(packet, passwordField...)
		}
	}

	_, err := mqtt.conn.Write(packet)
	return err
}

// ReadMQTTv3Packet reads and parses the CONNACK packet from the server.
func (mqtt *Connection) ReadMQTTv3Packet() error {
	response := make([]byte, 4)
	_, err := mqtt.conn.Read(response)
	if err != nil {
		return err
	}

	mqtt.results.Response = fmt.Sprintf("%X", response)

	// DISCONNECT packet
	if ((response[0] & 0xF0) == 0xE0) && (response[1] == 0x00) {
		return nil
	}

	// Check if the response is a valid CONNACK packet
	if response[0] != 0x20 || response[1] != 0x02 {
		return fmt.Errorf("invalid CONNACK packet")
	}

	mqtt.results.SessionPresent = (response[2] & 0x01) == 0x01
	mqtt.results.ConnectReturnCode = response[3]

	return nil
}

// ReadMQTTv5Packet reads and parses the CONNACK or DISCONNECT packet from the server for MQTT v5.0.
func (mqtt *Connection) ReadMQTTv5Packet() error {
	// Read the first byte to determine the packet type
	firstByte := make([]byte, 1)
	_, err := io.ReadFull(mqtt.conn, firstByte)
	if err != nil {
		return err
	}

	packetType := firstByte[0] >> 4

	// Read the remaining length
	remainingLengthBytes, err := readVariableByteInteger(mqtt.conn)
	if err != nil {
		return err
	}

	// Convert remaining length bytes to integer
	remainingLength, _ := binary.Uvarint(remainingLengthBytes)

	// Allocate the packet buffer with the correct size
	packet := make([]byte, 1+len(remainingLengthBytes)+int(remainingLength))
	packet[0] = firstByte[0]
	copy(packet[1:], remainingLengthBytes)

	// Read the rest of the packet
	_, err = io.ReadFull(mqtt.conn, packet[1+len(remainingLengthBytes):])
	if err != nil {
		return err
	}

	// Store the original response
	mqtt.results.Response = fmt.Sprintf("%X", packet)

	// Process the packet based on its type
	switch packetType {
	case 2: // CONNACK
		return mqtt.processConnAck(packet)
	case 14: // DISCONNECT
		return mqtt.processDisconnect(packet)
	default:
		return fmt.Errorf("unexpected packet type: %d", packetType)
	}
}

func (mqtt *Connection) processConnAck(packet []byte) error {
	if len(packet) < 4 {
		return fmt.Errorf("invalid CONNACK packet length")
	}

	mqtt.results.SessionPresent = (packet[2] & 0x01) == 0x01
	mqtt.results.ConnectReturnCode = packet[3]

	// Process properties if present
	if len(packet) > 4 {
		propertiesLength, n := binary.Uvarint(packet[4:])
		propertiesStart := 4 + n
		propertiesEnd := propertiesStart + int(propertiesLength)

		if propertiesEnd > len(packet) {
			return fmt.Errorf("invalid properties length in CONNACK")
		}
	}

	return nil
}

func (mqtt *Connection) processDisconnect(packet []byte) error {
	if len(packet) < 2 {
		return fmt.Errorf("invalid DISCONNECT packet length")
	}

	// Process properties if present
	if len(packet) > 3 {
		propertiesLength, n := binary.Uvarint(packet[3:])
		propertiesStart := 3 + n
		propertiesEnd := propertiesStart + int(propertiesLength)

		if propertiesEnd > len(packet) {
			return fmt.Errorf("invalid properties length in DISCONNECT")
		}
	}

	return nil
}

func readVariableByteInteger(r io.Reader) ([]byte, error) {
	var result []byte
	for i := 0; i < 4; i++ {
		b := make([]byte, 1)
		_, err := r.Read(b)
		if err != nil {
			return nil, err
		}
		result = append(result, b[0])
		if b[0]&0x80 == 0 {
			break
		}
	}
	if len(result) == 4 && result[3]&0x80 != 0 {
		return nil, fmt.Errorf("invalid variable byte integer")
	}
	return result, nil
}

// Scan performs the configured scan on the MQTT server.
func (s *Scanner) Scan(t zgrab2.ScanTarget) (status zgrab2.ScanStatus, result interface{}, thrown error) {
	conn, err := t.Open(&s.config.BaseFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error opening connection: %w", err)
	}
	defer conn.Close()

	mqtt := Connection{conn: conn, config: s.config}

	if s.config.UseTLS {
		tlsConn, err := s.config.TLSFlags.GetTLSConnection(conn)
		if err != nil {
			return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error getting TLS connection: %w", err)
		}
		mqtt.results.TLSLog = tlsConn.GetLog()

		if err := tlsConn.Handshake(); err != nil {
			return zgrab2.TryGetScanStatus(err), &mqtt.results, fmt.Errorf("error during TLS handshake: %w", err)
		}

		mqtt.conn = tlsConn
	}

	if err := mqtt.SendMQTTConnectPacket(s.config.V5); err != nil {
		return zgrab2.TryGetScanStatus(err), nil, fmt.Errorf("error sending CONNECT packet: %w", err)
	}

	if s.config.V5 {
		err = mqtt.ReadMQTTv5Packet()
	} else {
		err = mqtt.ReadMQTTv3Packet()
	}

	if err != nil {
		return zgrab2.TryGetScanStatus(err), &mqtt.results, fmt.Errorf("error reading CONNACK packet: %w", err)
	}

	return zgrab2.SCAN_SUCCESS, &mqtt.results, nil
}