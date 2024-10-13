// Package tftp provides a zgrab2 module that probes for the TFTP service.
package tftp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/zmap/zgrab2"
)

// TFTP Opcodes
const (
	TFTP_RRQ   uint16 = 1 // Opcode for Read Request
	TFTP_DATA  uint16 = 3 // Opcode for Data Packet
	TFTP_ERROR uint16 = 5 // Opcode for Error Packet
)

// Flags holds the command-line flags for the scanner.
type Flags struct {
	zgrab2.BaseFlags
	zgrab2.UDPFlags
	Verbose bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
}

// Module implements the zgrab2.Module interface.
type Module struct {
}

// Scanner implements the zgrab2.Scanner interface.
type Scanner struct {
	config *Flags
}

// Results is the struct that is returned to the zgrab2 framework from Scan().
type Results struct {
	ResponseMessage string `json:"response_message,omitempty"`
}

// RegisterModule registers the TFTP module with zgrab2.
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("tftp", "TFTP", module.Description(), 69, &module) // TFTP uses port 69
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns a new Flags instance for the module.
func (module *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new Scanner instance.
func (module *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description returns an overview of the module.
func (module *Module) Description() string {
	return "Scan for TFTP"
}

// Validate checks the flags are valid.
func (cfg *Flags) Validate(args []string) error {
	return nil
}

// Help returns the module's help string (needed to implement zgrab2.ScanFlags).
func (cfg *Flags) Help() string {
	return "This module scans for TFTP services."
}

// Init initializes the scanner.
func (scanner *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, ok := flags.(*Flags)
	if !ok {
		return fmt.Errorf("invalid flag type")
	}
	scanner.config = f
	return nil
}

// InitPerSender initializes the scanner for a given sender.
func (scanner *Scanner) InitPerSender(senderID int) error {
	return nil
}

// Protocol returns the protocol identifier for the scanner.
func (scanner *Scanner) Protocol() string {
	return "tftp"
}

// GetName returns the scanner's name (needed to implement zgrab2.Scanner).
func (scanner *Scanner) GetName() string {
	return "tftp"
}

// GetTrigger returns an empty string as TFTP does not need a special trigger.
func (scanner *Scanner) GetTrigger() string {
	return ""
}

// createRRQMessage creates the RRQ (Read Request) message for the TFTP protocol.
func createRRQMessage(filename, mode string) []byte {
	// Create a buffer for the RRQ message
	buf := make([]byte, 2+len(filename)+1+len(mode)+1)
	// Set opcode to RRQ
	binary.BigEndian.PutUint16(buf[0:2], TFTP_RRQ)
	// Append the filename and mode, both followed by a null byte
	copy(buf[2:], filename)
	buf[2+len(filename)] = 0
	copy(buf[3+len(filename):], mode)
	buf[len(buf)-1] = 0
	return buf
}

// SendRRQ sends an RRQ message to the server and waits for a response.
func (scanner *Scanner) SendRRQ(sock net.Conn, filename, mode string) ([]byte, error) {
	rrq := createRRQMessage(filename, mode)
	// Send the RRQ message
	_, err := sock.Write(rrq)
	if err != nil {
		return nil, err
	}

	// Wait for the response from the server
	buf := make([]byte, 516) // Maximum TFTP packet size (512 bytes + header)
	n, err := sock.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// decodeTFTPResponse decodes the response from the server.
func decodeTFTPResponse(response []byte) (string, error) {
	if len(response) < 2 {
		return "", errors.New("invalid response length")
	}
	opcode := binary.BigEndian.Uint16(response[0:2])
	switch opcode {
	case TFTP_DATA:
		blockNumber := binary.BigEndian.Uint16(response[2:4])
		data := response[4:]
		return fmt.Sprintf("Received DATA block #%d (%d bytes)", blockNumber, len(data)), nil
	case TFTP_ERROR:
		errorCode := binary.BigEndian.Uint16(response[2:4])
		var errorMessage string
		if len(response) > 4 {
			errorMessage = string(response[4 : len(response)-1]) // Trim the null byte
		}
		return fmt.Sprintf("Received ERROR code %d: %s", errorCode, errorMessage), nil
	default:
		return "", fmt.Errorf("Unknown TFTP response opcode: %d", opcode)
	}
}

// Scan performs the TFTP scan by sending an RRQ message and receiving the response.
func (scanner *Scanner) Scan(t zgrab2.ScanTarget) (zgrab2.ScanStatus, interface{}, error) {
	// Open a UDP connection to the target
	sock, err := t.OpenUDP(&scanner.config.BaseFlags, &scanner.config.UDPFlags)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, err
	}
	defer sock.Close()

	// Send the RRQ message for a dummy file
	response, err := scanner.SendRRQ(sock, "dummyfile", "octet")
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, err
	}

	// Decode and log the response
	resultMessage, err := decodeTFTPResponse(response)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, err
	}

	log.Info(resultMessage)

	// Return success status with the response message
	result := &Results{
		ResponseMessage: resultMessage,
	}
	return zgrab2.SCAN_SUCCESS, result, nil
}
