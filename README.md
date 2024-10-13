FASST-LLM enhances ZGrab2.0
=========

This is a scanner based on ZGrab2.0, enhanced by FASST-LLM. [ZGrab2.0](https://github.com/zmap/zgrab2) is a fast, modular application-layer network scanner. This project uses FASST-LLM to add scanning tools for 22 new services.

FASST-LLM is a Framework for Automated Service Scanning Tool generation using Large Language Models. FASST-LLM fully leverages the powerful capabilities of large language models in processing natural language and generating code.

## New Services

- AMQP1.0
- BOLT
- Cassandra
- DoQ
- IPMI
- IRC
- LDAP
- LPD
- Memcached
- MQTT
- PPTP
- RADIUS
- RethinDB
- RFB
- RTSP
- SIP
- SOCKS5
- TACACS+
- Terraria
- TFTP
- X11
- XMPP

## Installation

#### Build from Source

You can build from source:

```cmd
git clone xxx
cd xxx
make
./zgrab2
```

## Usage

The usage of this scanner is the same as ZGrab2.0. See the README of ZGrab2.0 in [ZGrab2.0_README](https://github.com/zmap/zgrab2/blob/master/README.md)

#### Option

Some services include optional parameters. Use ```./zgrab2 {service} --help``` to view the parameters and their functions.

```shell
$ ./zgrab2 sip --help
Usage:
  ./zgrab2 [OPTIONS] sip [sip-OPTIONS]

Probe for SIP (Session Initiation Protocol) servers using UDP or TCP

Application Options:
  /o, /output-file:                    Output filename, use - for stdout
                                       (default: -)
  /f, /input-file:                     Input filename, use - for stdin
                                       (default: -)
  /m, /metadata-file:                  Metadata filename, use - for stderr
                                       (default: -)
  /l, /log-file:                       Log filename, use - for stderr (default:
                                       -)
  /s, /senders:                        Number of send goroutines to use
                                       (default: 1000)
      /debug                           Include debug fields in the output.
      /flush                           Flush after each line of output.
      /gomaxprocs:                     Set GOMAXPROCS (default: 0)
      /connections-per-host:           Number of times to connect to each host
                                       (results in more output) (default: 1)
      /read-limit-per-host:            Maximum total kilobytes to read for a
                                       single host (default 96kb) (default: 96)
      /prometheus:                     Address to use for Prometheus server
                                       (e.g. localhost:8080). If empty,
                                       Prometheus is disabled.
      /dns:                            Address of a custom DNS server for
                                       lookups. Default port is 53.

Help Options:
  /?                                   Show this help message
  /h, /help                            Show this help message

[sip command options]
      /p, /port:                       Specify port to grab on (default: 5060)
      /n, /name:                       Specify name for output json, only
                                       necessary if scanning multiple modules
                                       (default: sip)
      /t, /timeout:                    Set connection timeout (0 = no timeout)
                                       (default: 10s)
      /g, /trigger:                    Invoke only on targets with specified tag
      /m, /maxbytes:                   Maximum byte read limit per scan (0 =
                                       defaults)
          /heartbleed                  Check if server is vulnerable to
                                       Heartbleed
          /session-ticket              Send support for TLS Session Tickets and
                                       output ticket if presented
          /extended-master-secret      Offer RFC 7627 Extended Master Secret
                                       extension
          /extended-random             Send TLS Extended Random Extension
          /no-sni                      Do not send domain name in TLS Handshake
                                       regardless of whether known
          /sct                         Request Signed Certificate Timestamps
                                       during TLS Handshake
          /keep-client-logs            Include the client-side logs in the TLS
                                       handshake
          /time:                       Explicit request time to use, instead of
                                       clock. YYYYMMDDhhmmss format.
          /certificates:               Set of certificates to present to the
                                       server
          /certificate-map:            A file mapping server names to
                                       certificates
          /root-cas:                   Set of certificates to use when
                                       verifying server certificates
          /next-protos:                A list of supported application-level
                                       protocols
          /server-name:                Server name used for certificate
                                       verification and (optionally) SNI
          /verify-server-certificate   If set, the scan will fail if the server
                                       certificate does not match the
                                       server-name, or does not chain to a
                                       trusted root.
          /cipher-suite:               A comma-delimited list of hex cipher
                                       suites to advertise.
          /min-version:                The minimum SSL/TLS version that is
                                       acceptable. 0 means that SSLv3 is the
                                       minimum.
          /max-version:                The maximum SSL/TLS version that is
                                       acceptable. 0 means use the highest
                                       supported value.
          /curve-preferences:          A list of elliptic curves used in an
                                       ECDHE handshake, in order of preference.
          /no-ecdhe                    Do not allow ECDHE handshakes
          /signature-algorithms:       Signature and hash algorithms that are
                                       acceptable
          /heartbeat-enabled           If set, include the heartbeat extension
          /dsa-enabled                 Accept server DSA keys
          /client-random:              Set an explicit Client Random (base64
                                       encoded)
          /client-hello:               Set an explicit ClientHello (base64
                                       encoded)
          /verbose                     More verbose logging, include debug
                                       fields in the scan results
          /tcp                         Use TCP instead of UDP for SIP scanning
          /tls                         Attempt to connect via a TLS wrapped
                                       connection
          /tcp-timeout:                Set connection timeout in seconds
                                       (default: 5)
```

## License and Copyright

This project uses [ZGrab2.0](https://github.com/zmap/zgrab2), which is licensed under the [License](https://github.com/zmap/zgrab2/blob/master/README.md#license).

Copyright 2016 the University of Michigan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

