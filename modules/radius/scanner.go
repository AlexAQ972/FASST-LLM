// Package radius provides a zgrab2 module that probes for the RADIUS service.
package radius

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

const (
	AccessRequest   = 1
	AccessAccept    = 2
	AccessReject    = 3
	AccessChallenge = 11
	RadiusPort      = 1812
)

var (
	ErrInvalidCode      = errors.New("invalid RADIUS code")
	ErrInvalidResponse  = errors.New("invalid RADIUS response")
	ErrTooShortResponse = errors.New("response is too short")
)

type RADIUSHeader struct {
	Code          uint8
	Identifier    uint8
	Length        uint16
	Authenticator [16]byte
}

func (hdr *RADIUSHeader) Encode() ([]byte, error) {
	buf := make([]byte, 20)
	buf[0] = hdr.Code
	buf[1] = hdr.Identifier
	binary.BigEndian.PutUint16(buf[2:4], hdr.Length)
	copy(buf[4:20], hdr.Authenticator[:])
	return buf, nil
}

type RADIUSAttribute struct {
	Type   uint8
	Length uint8
	Value  []byte
}

func (attr *RADIUSAttribute) Encode() ([]byte, error) {
	buf := make([]byte, 2+len(attr.Value))
	buf[0] = attr.Type
	buf[1] = attr.Length
	copy(buf[2:], attr.Value)
	return buf, nil
}

// Flags holds the command-line flags for the scanner.
type Flags struct {
	zgrab2.BaseFlags
	Verbose bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
}

// Help returns the module's help string, which is required to implement zgrab2.ScanFlags.
func (f *Flags) Help() string {
	return "Module to scan RADIUS servers"
}

// Validate checks that the flags are valid, required to implement zgrab2.ScanFlags.
func (f *Flags) Validate(args []string) error {
	return nil
}

// Module is the zgrab2 module implementation.
type Module struct{}

// Scanner holds the state for a single scan.
type Scanner struct {
	config *Flags
}

// RegisterModule registers the module with zgrab2.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("radius", "RADIUS", module.Description(), RadiusPort, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns a flags instance to be populated with the command line args.
func (module *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new RADIUS scanner instance.
func (module *Module) NewScanner() zgrab2.Scanner {
	return &Scanner{}
}

// Description returns an overview of this module.
func (module *Module) Description() string {
	return "Scan for RADIUS"
}

// Init initializes the scanner.
func (scanner *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, ok := flags.(*Flags)
	if !ok {
		return errors.New("invalid flags type")
	}
	scanner.config = f
	return nil
}

// InitPerSender initializes the scanner for a given sender.
func (scanner *Scanner) InitPerSender(senderID int) error {
	return nil
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "radius"
}

// GetName returns the module's name.
func (scanner *Scanner) GetName() string {
	return "radius"
}

// GetTrigger returns an empty trigger since no specific trigger is used for this scan.
func (scanner *Scanner) GetTrigger() string {
	return ""
}

// buildAccessRequest constructs the Access-Request packet with required attributes.
func buildAccessRequest(identifier uint8, ipAddr net.IP, port uint16) ([]byte, error) {
	// Build the Access-Request header
	header := &RADIUSHeader{
		Code:          AccessRequest,
		Identifier:    identifier,
		Authenticator: generateRandomAuthenticator(),
	}

	// Build attributes
	userNameAttr := RADIUSAttribute{
		Type:   1,
		Length: uint8(2 + len("test")),
		Value:  []byte("test"),
	}

	nasIPAttr := RADIUSAttribute{
		Type:   4,
		Length: 6,
		Value:  ipAddr.To4(),
	}

	nasPortAttr := RADIUSAttribute{
		Type:   5,
		Length: 6,
		Value:  []byte{0, 0, byte(port >> 8), byte(port & 0xff)},
	}

	// Encode the header and attributes
	headerBytes, err := header.Encode()
	if err != nil {
		return nil, err
	}

	userNameBytes, err := userNameAttr.Encode()
	if err != nil {
		return nil, err
	}

	nasIPBytes, err := nasIPAttr.Encode()
	if err != nil {
		return nil, err
	}

	nasPortBytes, err := nasPortAttr.Encode()
	if err != nil {
		return nil, err
	}

	// Combine everything into one packet
	packet := append(headerBytes, userNameBytes...)
	packet = append(packet, nasIPBytes...)
	packet = append(packet, nasPortBytes...)

	// Set the final packet length
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet, nil
}

// SendAccessRequest sends an Access-Request to the RADIUS server and waits for a response.
func (scanner *Scanner) SendAccessRequest(sock net.Conn) (*RADIUSHeader, error) {
	identifier := generateRandomIdentifier()
	ipAddr := net.ParseIP("192.168.0.1") // Placeholder, replace with actual client IP

	packet, err := buildAccessRequest(identifier, ipAddr, RadiusPort)
	if err != nil {
		return nil, err
	}

	_, err = sock.Write(packet)
	if err != nil {
		return nil, err
	}

	// Read the response
	buf := make([]byte, 512)
	n, err := sock.Read(buf)
	if err != nil {
		return nil, err
	}

	// Ensure the response is at least 20 bytes (minimum RADIUS packet size)
	if n < 20 {
		return nil, ErrTooShortResponse
	}

	// Parse the RADIUS response
	response := &RADIUSHeader{}
	response.Code = buf[0]
	response.Identifier = buf[1]
	response.Length = binary.BigEndian.Uint16(buf[2:4])
	copy(response.Authenticator[:], buf[4:20])

	return response, nil
}

// ValidateResponse checks if the response is valid and logs the outcome.
func (scanner *Scanner) ValidateResponse(header *RADIUSHeader) error {
	switch header.Code {
	case AccessAccept:
		log.Info("Received Access-Accept")
	case AccessReject:
		log.Info("Received Access-Reject")
	case AccessChallenge:
		log.Info("Received Access-Challenge")
	default:
		return fmt.Errorf("%w: received code %d", ErrInvalidCode, header.Code)
	}
	return nil
}

// Scan scans the target for RADIUS service.
func (scanner *Scanner) Scan(t zgrab2.ScanTarget) (zgrab2.ScanStatus, interface{}, error) {
	sock, err := t.OpenUDP(&scanner.config.BaseFlags, nil)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, err
	}
	defer sock.Close()

	// Send the Access-Request
	respHeader, err := scanner.SendAccessRequest(sock)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, err
	}

	// Validate the response
	err = scanner.ValidateResponse(respHeader)
	if err != nil {
		return zgrab2.SCAN_PROTOCOL_ERROR, nil, err
	}

	return zgrab2.SCAN_SUCCESS, respHeader, nil
}

// generateRandomAuthenticator generates a random 16-byte authenticator.
func generateRandomAuthenticator() [16]byte {
	var authenticator [16]byte
	for i := range authenticator {
		authenticator[i] = byte(time.Now().UnixNano() % 256)
	}
	return authenticator
}

// generateRandomIdentifier generates a random identifier for RADIUS request.
func generateRandomIdentifier() uint8 {
	return uint8(time.Now().UnixNano() % 256)
}
