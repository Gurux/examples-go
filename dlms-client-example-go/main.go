package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Gurux/gxcommon-go"
	"github.com/Gurux/gxdlms-go/enums"
	"github.com/Gurux/gxdlms-go/settings"
	"github.com/Gurux/gxserial-go"
)

func main() {
	conf, err := getParameters(os.Args[1:])
	if err != nil {
		showHelp()
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	if conf == nil {
		showHelp()
		return
	}

	reader := NewGXDLMSReader(conf.client,
		conf.media,
		conf.trace,
		conf.invocationCounterLN,
		conf.WaitTime)

	if err := conf.media.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if conf.media != nil {
			if _, ok := conf.media.(*gxserial.GXSerial); ok {
				//Show available serial ports.
				ports, err := gxserial.GetPortNames()
				if err == nil {
					fmt.Fprintf(os.Stderr, "Available serial ports: %s\n", strings.Join(ports, ", "))

				}
			}
		}
		return
	}

	conf.client.SetCustomPduHandler(func(e *settings.GXCustomPduArgs) error {
		fmt.Printf("Custom PDU received: %x\n", e.Data)
		e.Value = e.Data // Set parsed value, if needed.
		// Process custom PDU and return error if needed.
		return nil
	})

	conf.media.SetOnError(func(m gxcommon.IGXMedia, err error) {
		// log/handle error
		fmt.Fprintln(os.Stderr, "error:", err)
	})

	conf.media.SetOnTrace(func(m gxcommon.IGXMedia, e gxcommon.TraceEventArgs) {
		fmt.Printf("Trace: %s\n", e.String())
	})

	defer func() { _ = reader.Close() }()

	if len(conf.readObjects) == 0 {
		if err := reader.ReadAll(conf.outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		return
	}

	if err := reader.InitializeConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	for _, item := range conf.readObjects {
		obj := conf.client.Objects().FindByLN(enums.ObjectTypeNone, item.Key)
		if obj == nil {
			fmt.Fprintf(os.Stderr, "error: object not found: %s\n", item.Key)
			continue
		}
		value, err := reader.Read(obj, item.Value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s:%d failed: %v\n", item.Key, item.Value, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "%s:%d = %v\n", item.Key, item.Value, value)
	}
}
