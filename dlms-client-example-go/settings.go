package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Gurux/gxcommon-go"
	dlms "github.com/Gurux/gxdlms-go"
	"github.com/Gurux/gxdlms-go/enums"
	"github.com/Gurux/gxdlms-go/settings"
	"github.com/Gurux/gxdlms-go/types"
	"github.com/Gurux/gxnet-go"
)

type gxSettings struct {
	media  gxcommon.IGXMedia
	trace  gxcommon.TraceLevel
	client *dlms.GXDLMSSecureClient
	// Invocation counter (frame counter).
	invocationCounterLN string
	//Objects to read.
	readObjects []*types.GXKeyValuePair[string, int]
	//Cache file.
	outputFile string
	//Client and server certificates are exported from the meter.
	ExportSecuritySetupLN string

	//Generate new client and server certificates and import them to the server.
	GenerateSecuritySetupLN string

	WaitTime int
}

func showHelp() {
	fmt.Println("GuruxDlmsSample reads data from the DLMS/COSEM device.")
	fmt.Println("GuruxDlmsSample -h [Meter IP Address] -p [Meter Port No] -c 16 -s 1 -r SN")
	fmt.Println(" -h \t host name or IP address.")
	fmt.Println(" -p \t port number (Example: 1000).")
	fmt.Println(" -u \t UDP is used as a transport protocol.")
	fmt.Println(" -S [COM1:9600:8None1]\t serial port.")
	fmt.Println(" -a \t Authentication (None, Low, High).")
	fmt.Println(" -P \t Password for authentication.")
	fmt.Println(" -c \t Client address. (Default: 16)")
	fmt.Println(" -s \t Server address. (Default: 1)")
	fmt.Println(" -n \t Server address as serial number.")
	fmt.Println(" -l \t Logical Server address.")
	fmt.Println(" -r [sn, ln]\t Short name or Logical Name (default) referencing is used.")
	fmt.Println(" -t [Error, Warning, Info, Verbose] Trace messages.")
	fmt.Println(" -g \"0.0.1.0.0.255:1; 0.0.1.0.0.255:2\" Get selected object(s) with given attribute index.")
	fmt.Println(" -C \t Security Level. (None, Authentication, Encrypted, AuthenticationEncryption)")
	fmt.Println(" -V \t Security Suite version. (Default: Suite0). (Suite0, Suite1 or Suite2)")
	fmt.Println(" -K \t Signing (None, EphemeralUnifiedModel, OnePassDiffieHellman or StaticUnifiedModel, GeneralSigning).")
	fmt.Println(" -v \t Invocation counter data object Logical Name. Ex. 0.0.43.1.1.255")
	fmt.Println(" -I \t Auto increase invoke ID")
	fmt.Println(" -o \t Cache association view to make reading faster. Ex. -o C:\\device.xml")
	fmt.Println(" -T \t System title that is used with chiphering. Ex -T 4775727578313233")
	fmt.Println(" -M \t Meter system title that is used with chiphering. Ex -T 4775727578313233")
	fmt.Println(" -A \t Authentication key that is used with chiphering. Ex -A D0D1D2D3D4D5D6D7D8D9DADBDCDDDEDF")
	fmt.Println(" -B \t Block cipher key that is used with chiphering. Ex -B 000102030405060708090A0B0C0D0E0F")
	fmt.Println(" -b \t Broadcast Block cipher key that is used with chiphering. Ex -b 000102030405060708090A0B0C0D0E0F")
	fmt.Println(" -D \t Dedicated key that is used with chiphering. Ex -D 00112233445566778899AABBCCDDEEFF")
	fmt.Println(" -F \t Initial Frame Counter (Invocation counter) value.")
	fmt.Println(" -d \t Used DLMS standard. Ex -d India (DLMS, India, Italy, SaudiArabia, IDIS)")
	fmt.Println(" -E \t Export client and server certificates from the meter. Ex. -E 0.0.43.0.0.255.")
	fmt.Println(" -N \t Generate new client and server certificates and import them to the server. Ex. -N 0.0.43.0.0.255.")
	fmt.Println(" -G \t Use Gateway with given NetworkId and PhysicalDeviceAddress. Ex -G 0:1.")
	fmt.Println(" -i \t Used communication interface. Ex. -i WRAPPER.")
	fmt.Println(" -m \t Used PLC MAC address. Ex. -m 1.")
	fmt.Println(" -W \t General Block Transfer window size.")
	fmt.Println(" -w \t HDLC Window size. Default is 1")
	fmt.Println(" -f \t HDLC Frame size. Default is 128")
	fmt.Println(" -x \t Wait time in milliseconds. The default is 5000 ms.")
	fmt.Println(" -O \t Proposed conformance. -O \"Get,Set\"")
	fmt.Println(" -L \t Manufacturer ID (Flag ID) is used to use manufacturer depending functionality. -L LGZ")
	fmt.Println(" -R \t Data is send as a broadcast (UnConfirmed, Confirmed).")
	fmt.Println("Example:")
	fmt.Println("Read LG device using TCP/IP connection.")
	fmt.Println("GuruxDlmsSample -r SN -c 16 -s 1 -h [Meter IP Address] -p [Meter Port No]")
	fmt.Println("Read LG device using serial port connection.")
	fmt.Println("GuruxDlmsSample -r SN -c 16 -s 1 -S COM1")
	fmt.Println("Read Indian device using serial port connection.")
	fmt.Println("GuruxDlmsSample -S COM1 -c 16 -s 1 -a Low -P [password]")
	fmt.Println("Read MQTT device -h [Broker address] -q [Topic/meterId]")
}

// getParameters parses command line arguments and returns settings for the reader.
func getParameters(args []string) (*gxSettings, error) {
	var err error
	opts := gxSettings{
		trace:    gxcommon.TraceLevelInfo,
		WaitTime: 5000,
	}
	// Initialize DLMS client with default settings.
	opts.client, _ = dlms.NewGXDLMSSecureClient(true, 16, 1, enums.AuthenticationNone, nil, enums.InterfaceTypeHDLC)
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--help" || a == "-?" || a == "-help" {
			return nil, nil
		}

		if !strings.HasPrefix(a, "-") || len(a) != 2 {
			return nil, fmt.Errorf("unexpected argument: %q (expected flag like -h)", a)
		}
		flag := a[1:]
		needValue := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag -%s requires a value", flag)
			}
			i++
			return args[i], nil
		}

		switch flag {

		// Simple value flags
		case "h":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			if opts.media == nil {
				opts.media = gxnet.NewGXNet(gxnet.NetworkTypeTCP, "", 0)
			}
			if m, ok := opts.media.(*gxnet.GXNet); ok {
				m.HostName = v
			}
		case "p":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -p port %q", v)
			}
			if m, ok := opts.media.(*gxnet.GXNet); ok {
				m.Port = n
			}
		case "S":
			return nil, fmt.Errorf("serial port is not supported in this example")
			/*
				v, err := needValue()
				if err != nil {
					return nil, err
				}
				opt.SerialPort = v
			*/
		case "a":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.AuthenticationParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetAuthentication(ret)
			if err != nil {
				return nil, err
			}
		case "P":
			ret, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.SetPassword([]byte(ret))
			if err != nil {
				return nil, err
			}
		case "c":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -c client address %q", v)
			}
			err = opts.client.SetClientAddress(ret)
			if err != nil {
				return nil, err
			}
		case "s":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -s server address %q", v)
			}
			if opts.client.ServerAddress() != 1 {
				ret2, err := dlms.GetServerAddress(opts.client.ServerAddress(), ret)
				if err != nil {
					return nil, fmt.Errorf("invalid -s server address %q", v)
				}
				err = opts.client.SetServerAddress(ret2)
				if err != nil {
					return nil, err
				}
			} else {
				err = opts.client.SetServerAddress(ret)
				if err != nil {
					return nil, err
				}
			}
		case "l":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -l logical server address %q", v)
			}
			ret2, err := dlms.GetServerAddress(n, opts.client.ServerAddress())
			if err != nil {
				return nil, fmt.Errorf("invalid -l logical server address %q", v)
			}
			err = opts.client.SetServerAddress(ret2)
			if err != nil {
				return nil, err
			}
		case "r":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			switch strings.ToLower(v) {
			case "sn":
				err = opts.client.SetUseLogicalNameReferencing(true)
			case "ln":
				err = opts.client.SetUseLogicalNameReferencing(false)
			default:
				return nil, fmt.Errorf("invalid -r %q (sn, ln)", v)
			}
			if err != nil {
				return nil, err
			}
		case "t":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := gxcommon.TraceLevelParse(v)
			if err != nil {
				return nil, err
			}
			opts.trace = ret
		case "g":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			parts := strings.Split(v, ";")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				// "0.0.1.0.0.255:1"
				idx := strings.LastIndex(p, ":")
				if idx <= 0 || idx == len(p)-1 {
					return nil, fmt.Errorf("expected LN:attrIndex, got %q", p)
				}
				ln := strings.TrimSpace(p[:idx])
				attrStr := strings.TrimSpace(p[idx+1:])
				attr, err := strconv.Atoi(attrStr)
				if err != nil || attr <= 0 {
					return nil, fmt.Errorf("invalid attribute index %q in %q", attrStr, p)
				}
				opts.readObjects = append(opts.readObjects, types.NewGXKeyValuePair[string, int](ln, attr))
			}
		case "C":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.SecurityParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetSecurity(ret)
			if err != nil {
				return nil, err
			}
		case "V":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.SecuritySuiteParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetSecuritySuite(ret)
			if err != nil {
				return nil, err
			}
		case "K":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.SigningParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.Ciphering().SetSigning(ret)
			if err != nil {
				return nil, err
			}
		case "v":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			opts.invocationCounterLN = v

		case "o":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			opts.outputFile = v
		case "T":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.Ciphering().SetSystemTitle(types.HexToBytes(v))
			if err != nil {
				return nil, err
			}
		case "M":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.Ciphering().SetRecipientSystemTitle(types.HexToBytes(v))
			if err != nil {
				return nil, err
			}
		case "A":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.Ciphering().SetAuthenticationKey(types.HexToBytes(v))
			if err != nil {
				return nil, err
			}
		case "B":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.Ciphering().SetBlockCipherKey(types.HexToBytes(v))
			if err != nil {
				return nil, err
			}
		case "b":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.Ciphering().SetBroadcastBlockCipherKey(types.HexToBytes(v))
			if err != nil {
				return nil, err
			}
		case "D":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.Ciphering().SetDedicatedKey(types.HexToBytes(v))
			if err != nil {
				return nil, err
			}
		case "F":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid -F %q", v)
			}
			err = opts.client.Ciphering().SetInvocationCounter(uint32(ret))
			if err != nil {
				return nil, err
			}
		case "d":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.StandardParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetStandard(ret)
			if err != nil {
				return nil, err
			}
			if opts.client.Standard() == enums.StandardItaly ||
				opts.client.Standard() == enums.StandardIndia ||
				opts.client.Standard() == enums.StandardSaudiArabia {
				err = opts.client.SetUseUtc2NormalTime(true)
				if err != nil {
					return nil, err
				}
			}
		case "E":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			opts.ExportSecuritySetupLN = v
		case "N":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			opts.GenerateSecuritySetupLN = v
		case "G":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			tmp := strings.Split(v, ":")
			gw := &settings.GXDLMSGateway{}
			ret, err := strconv.Atoi(tmp[0])
			if err != nil {
				return nil, fmt.Errorf("invalid -G network id %q", tmp[0])
			}
			gw.NetworkID = uint8(ret)
			gw.PhysicalDeviceAddress = types.HexToBytes(tmp[1])
			err = opts.client.SetGateway(gw)
			if err != nil {
				return nil, err
			}
		case "i":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.InterfaceTypeParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetInterfaceType(ret)
			if err != nil {
				return nil, err
			}
		case "m":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -m %q", v)
			}
			opts.client.Plc().MacDestinationAddress = uint16(n)
		case "W":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -W %q", v)
			}
			err = opts.client.SetGbtWindowSize(byte(n))
			if err != nil {
				return nil, err
			}
		case "w":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -w %q", v)
			}
			err = opts.client.HdlcSettings().SetWindowSizeRX(uint8(n))
			if err != nil {
				return nil, err
			}
		case "f":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -w %q", v)
			}
			err = opts.client.HdlcSettings().SetMaxInfoRX(uint16(n))
			if err != nil {
				return nil, err
			}
		case "x":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid -x %q", v)
			}
			opts.WaitTime = n
		case "O":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.ConformanceParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetProposedConformance(ret)
			if err != nil {
				return nil, err
			}
		case "L":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			err = opts.client.SetManufacturerID(v)
			if err != nil {
				return nil, err
			}
		case "R":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := enums.ServiceClassParse(v)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetServiceClass(ret)
			if err != nil {
				return nil, err
			}
		// Bool flags (no value)
		case "u":
			//UDP.
			if opts.media == nil {
				opts.media = gxnet.NewGXNet(gxnet.NetworkTypeUDP, "", 0)
			}
			if m, ok := opts.media.(*gxnet.GXNet); ok {
				m.Protocol = gxnet.NetworkTypeUDP
			}
		case "n":
			v, err := needValue()
			if err != nil {
				return nil, err
			}
			ret, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid -F %q", v)
			}
			ret2, err := dlms.GetServerAddressFromSerialNumber(ret, 1)
			if err != nil {
				return nil, err
			}
			err = opts.client.SetServerAddress(ret2)
			if err != nil {
				return nil, err
			}
		case "I":
			err = opts.client.SetAutoIncreaseInvokeID(true)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown flag: %s", a)
		}
		i++
	}
	return &opts, nil
}
