package doq

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
	"github.com/zmap/zgrab2"
)

// Flags holds the command-line flags for the scanner.
type Flags struct {
	zgrab2.BaseFlags
	Verbose    bool `long:"verbose" description:"More verbose logging, include debug fields in the scan results"`
	SkipVerify bool `long:"skip-verify" description:"Skip certificate verification during the scan"`
}

// Module is the zgrab2 module implementation
type Module struct {
}

// Scanner holds the state for a single scan
type Scanner struct {
	config *Flags
}

// RegisterModule registers the module with zgrab2
func RegisterModule() {
	var module Module
	_, err := zgrab2.AddCommand("doq", "DoQ", module.Description(), 853, &module)
	if err != nil {
		log.Fatal(err)
	}
}

// NewFlags returns a flags instance to be populated with the command line args
func (module *Module) NewFlags() interface{} {
	return new(Flags)
}

// NewScanner returns a new DoQ scanner instance
func (module *Module) NewScanner() zgrab2.Scanner {
	return new(Scanner)
}

// Description returns an overview of this module.
func (module *Module) Description() string {
	return "Scan for DoQ (DNS over QUIC) services"
}

// Validate checks that the flags are valid
func (cfg *Flags) Validate(args []string) error {
	return nil
}

// Help returns the module's help string
func (cfg *Flags) Help() string {
	return "Help for DoQ module"
}

// Init initializes the scanner
func (scanner *Scanner) Init(flags zgrab2.ScanFlags) error {
	f, _ := flags.(*Flags)
	scanner.config = f
	return nil
}

// InitPerSender initializes the scanner for a given sender
func (scanner *Scanner) InitPerSender(senderID int) error {
	return nil
}

// Protocol returns the protocol identifier for the scanner.
func (s *Scanner) Protocol() string {
	return "doq"
}

// GetName returns the scanner's name
func (scanner *Scanner) GetName() string {
	return "doq"
}

// GetTrigger returns the trigger for the scan
func (scanner *Scanner) GetTrigger() string {
	return scanner.config.Trigger
}

// SendAndReceive sends a DNS query over QUIC and receives the response
func (scanner *Scanner) SendAndReceive(target zgrab2.ScanTarget, port int) (*dns.Msg, error) {
	// Context and TLS configuration with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: scanner.config.SkipVerify,
		NextProtos:         []string{"doq"},
	}

	// Establish a QUIC connection to the specified port
	addr := fmt.Sprintf("%s:%d", target.IP, port)
	session, err := quic.DialAddr(ctx, addr, tlsConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to establish QUIC connection: %w", err)
	}
	defer session.CloseWithError(0, "closing")

	// Open a stream
	stream, err := session.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open QUIC stream: %w", err)
	}
	defer stream.Close()

	// Construct a minimal DNS query
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	msg.Id = 0

	// Encode the DNS query
	dnsData, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack DNS query: %w", err)
	}

	// Write the length prefix and DNS query to the QUIC stream
	lengthPrefix := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthPrefix, uint16(len(dnsData)))
	if _, err := stream.Write(lengthPrefix); err != nil {
		return nil, fmt.Errorf("failed to write DNS query length prefix: %w", err)
	}
	if _, err := stream.Write(dnsData); err != nil {
		return nil, fmt.Errorf("failed to write DNS query: %w", err)
	}

	// Read the response from the server
	responsePrefix := make([]byte, 2)
	if _, err := io.ReadFull(stream, responsePrefix); err != nil {
		return nil, fmt.Errorf("failed to read DNS response length prefix: %w", err)
	}
	responseLength := binary.BigEndian.Uint16(responsePrefix)

	responseData := make([]byte, responseLength)
	if _, err := io.ReadFull(stream, responseData); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("unexpected EOF while reading DNS response")
		}
		return nil, fmt.Errorf("failed to read DNS response: %w", err)
	}

	// Parse the DNS response
	responseMsg := new(dns.Msg)
	if err := responseMsg.Unpack(responseData); err != nil {
		return nil, fmt.Errorf("failed to unpack DNS response: %w", err)
	}

	return responseMsg, nil
}

// Scan performs the scan using the provided target and configuration
func (scanner *Scanner) Scan(target zgrab2.ScanTarget) (zgrab2.ScanStatus, interface{}, error) {
	// Determine the port to use for the connection
	var port int
	if target.Port != nil {
		port = int(*target.Port)
	} else {
		port = int(scanner.config.BaseFlags.Port)
	}

	// Connect to the specified port
	result := &dns.Msg{}
	resp, err := scanner.SendAndReceive(target, port)
	if err != nil {
		return zgrab2.TryGetScanStatus(err), nil, err
	}

	result = resp
	return zgrab2.SCAN_SUCCESS, result, nil
}
