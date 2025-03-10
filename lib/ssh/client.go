// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssh

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Client implements a traditional SSH client that supports shells,
// subprocesses, port forwarding and tunneled dialing.
type Client struct {
	Conn

	forwards        forwardList // forwarded tcpip connections from the remote side
	mu              sync.Mutex
	channelHandlers map[string]chan NewChannel
}

// HandleChannelOpen returns a channel on which NewChannel requests
// for the given type are sent. If the type already is being handled,
// nil is returned. The channel is closed when the connection is closed.
func (c *Client) HandleChannelOpen(channelType string) <-chan NewChannel {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channelHandlers == nil {
		// The SSH channel has been closed.
		c := make(chan NewChannel)
		close(c)
		return c
	}

	ch := c.channelHandlers[channelType]
	if ch != nil {
		return nil
	}

	ch = make(chan NewChannel, 16)
	c.channelHandlers[channelType] = ch
	return ch
}

// NewClient creates a Client on top of the given connection.
func NewClient(c Conn, chans <-chan NewChannel, reqs <-chan *Request) *Client {
	conn := &Client{
		Conn:            c,
		channelHandlers: make(map[string]chan NewChannel, 1),
	}

	go conn.handleGlobalRequests(reqs)
	go conn.handleChannelOpens(chans)
	go func() {
		conn.Wait()
		conn.forwards.closeAll()
	}()
	go conn.forwards.handleChannels(conn.HandleChannelOpen("forwarded-tcpip"))
	return conn
}

// NewClientConn establishes an authenticated SSH connection using c
// as the underlying transport.  The Request and NewChannel channels
// must be serviced or the connection will hang.
func NewClientConn(c net.Conn, addr string, config *ClientConfig) (Conn, <-chan NewChannel, <-chan *Request, error) {
	fullConf := *config
	fullConf.SetDefaults()
	conn := &connection{
		sshConn: sshConn{conn: c},
	}

	if err := conn.clientHandshake(addr, &fullConf); err != nil {
		c.Close()
		return nil, nil, nil, fmt.Errorf("ssh: handshake failed: %v", err)
	}
	conn.mux = newMux(conn.transport)
	return conn, conn.mux.incomingChannels, conn.mux.incomingRequests, nil
}

// clientHandshake performs the client side key exchange. See RFC 4253 Section
// 7.
func (c *connection) clientHandshake(dialAddress string, config *ClientConfig) error {
	if config.ClientVersion != "" {
		c.clientVersion = []byte(config.ClientVersion)
	} else {
		c.clientVersion = []byte(packageVersion)
	}
	var err error
	c.serverVersion, err = exchangeVersions(c.sshConn.conn, c.clientVersion)
	if err != nil {
		return err
	}

	if config.ConnLog != nil {
		config.ConnLog.ServerID = new(EndpointId)
		config.ConnLog.ServerID.Raw = string(c.serverVersion)

		serverSplitId := strings.SplitN(string(c.serverVersion), " ", 2)
		if len(serverSplitId) == 2 {
			config.ConnLog.ServerID.Comment = serverSplitId[1]
		}

		serverSplitGroup := strings.SplitN(serverSplitId[0], "-", 3)
		if serverSplitGroup[0] == "SSH" {
			// If ID doesn't start with "SSH", don't attempt to parse.
			if len(serverSplitGroup) > 1 {
				config.ConnLog.ServerID.ProtoVersion = serverSplitGroup[1]
			}

			if len(serverSplitGroup) == 3 {
				config.ConnLog.ServerID.SoftwareVersion = serverSplitGroup[2]
			}
		}
	}
	if config.Verbose {
		if config.ConnLog != nil {
			//config.ConnLog.ClientIDString = string(c.clientVersion)
		}

		if config.ConnLog != nil {
			config.ConnLog.ClientID = new(EndpointId)
			config.ConnLog.ClientID.Raw = string(c.clientVersion)

			clientSplitId := strings.SplitN(string(c.clientVersion), " ", 2)
			if len(clientSplitId) == 2 {
				config.ConnLog.ClientID.Comment = clientSplitId[1]
			}

			clientSplitGroup := strings.SplitN(clientSplitId[0], "-", 3)
			if clientSplitGroup[0] == "SSH" {
				// If ID doesn't start with "SSH", don't attempt to parse.
				if len(clientSplitGroup) > 1 {
					config.ConnLog.ClientID.ProtoVersion = clientSplitGroup[1]
				}

				if len(clientSplitGroup) == 3 {
					config.ConnLog.ClientID.SoftwareVersion = clientSplitGroup[2]
				}
			}
		}
	}

	c.transport = newClientTransport(
		newTransport(c.sshConn.conn, config.Rand, true /* is client */),
		c.clientVersion, c.serverVersion, config, dialAddress, c.sshConn.RemoteAddr())

	if config.HelloOnly == true {
		return nil
	}

	if err := c.transport.requestInitialKeyChange(); err != nil {
		return err
	}

	// We just did the key change, so the session ID is established.
	c.sessionID = c.transport.getSessionID()

	return c.clientAuthenticate(config)
}

// verifyHostKeySignature verifies the host key obtained in the key
// exchange.
func verifyHostKeySignature(hostKey PublicKey, result *kexResult) error {
	sig, rest, ok := parseSignatureBody(result.Signature)
	if len(rest) > 0 || !ok {
		return errors.New("ssh: signature parse error")
	}

	return hostKey.Verify(result.H, sig)
}

// NewSession opens a new Session for this client. (A session is a remote
// execution of a program.)
func (c *Client) NewSession() (*Session, error) {
	ch, in, err := c.OpenChannel("session", nil)
	if err != nil {
		return nil, err
	}
	return newSession(ch, in)
}

func (c *Client) handleGlobalRequests(incoming <-chan *Request) {
	for r := range incoming {
		// This handles keepalive messages and matches
		// the behaviour of OpenSSH.
		r.Reply(false, nil)
	}
}

// handleChannelOpens channel open messages from the remote side.
func (c *Client) handleChannelOpens(in <-chan NewChannel) {
	for ch := range in {
		c.mu.Lock()
		handler := c.channelHandlers[ch.ChannelType()]
		c.mu.Unlock()

		if handler != nil {
			handler <- ch
		} else {
			ch.Reject(UnknownChannelType, fmt.Sprintf("unknown channel type: %v", ch.ChannelType()))
		}
	}

	c.mu.Lock()
	for _, ch := range c.channelHandlers {
		close(ch)
	}
	c.channelHandlers = nil
	c.mu.Unlock()
}

// Dial starts a client connection to the given SSH server. It is a
// convenience function that connects to the given network address,
// initiates the SSH handshake, and then sets up a Client.  For access
// to incoming channels and requests, use net.Dial with NewClientConn
// instead.
func Dial(network, addr string, config *ClientConfig) (*Client, error) {
	conn, err := net.DialTimeout(network, addr, config.Timeout)
	if err != nil {
		return nil, err
	}

	if config.Timeout != 0 {
		conn.SetDeadline(time.Now().Add(config.Timeout))
	}
	c, chans, reqs, err := NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	return NewClient(c, chans, reqs), nil
}

// BannerCallback is the function type used for treat the banner sent by
// the server. A BannerCallback receives the message sent by the remote server.
type BannerCallback func(message string) error

// A ClientConfig structure is used to configure a Client. It must not be
// modified after having been passed to an SSH function.
type ClientConfig struct {
	// Config contains configuration that is shared between clients and
	// servers.
	Config

	// User contains the username to authenticate as.
	User string

	// Auth contains possible authentication methods to use with the
	// server. Only the first instance of a particular RFC 4252 method will
	// be used during authentication.
	Auth []AuthMethod

	// HostKeyCallback, if not nil, is called during the cryptographic
	// handshake to validate the server's host key. A nil HostKeyCallback
	// implies that all host keys are accepted.
	HostKeyCallback func(hostname string, remote net.Addr, key PublicKey) error

	// BannerCallback is called during the SSH dance to display a custom
	// server's message. The client configuration can supply this callback to
	// handle it as wished. The function BannerDisplayStderr can be used for
	// simplistic display on Stderr.
	BannerCallback BannerCallback

	// ClientVersion contains the version identification string that will
	// be used for the connection. If empty, a reasonable default is used.
	ClientVersion string

	// HostKeyAlgorithms lists the key types that the client will
	// accept from the server as host key, in order of
	// preference. If empty, a reasonable default is used. Any
	// string returned from PublicKey.Type method may be used, or
	// any of the CertAlgoXxxx and KeyAlgoXxxx constants.
	HostKeyAlgorithms []string

	// Timeout is the maximum amount of time for the TCP connection to establish.
	//
	// A Timeout of zero means no timeout.
	Timeout time.Duration

	// If true, send the "none" Authentication Request to collect the advertised
	// userauth method names, but do not attempt to authenticate.
	DontAuthenticate bool
}
