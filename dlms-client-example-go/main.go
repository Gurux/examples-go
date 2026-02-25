package main

import (
	"fmt"
	"os"

	"github.com/Gurux/gxcommon-go"
	"github.com/Gurux/gxdlms-go/enums"
)

func main() {
	settings, err := getParameters(os.Args[1:])
	if err != nil {
		showHelp()
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	if settings == nil {
		showHelp()
		return
	}

	reader := NewGXDLMSReader(settings.client,
		settings.media,
		settings.trace,
		settings.invocationCounterLN,
		settings.WaitTime)

	if err := settings.media.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	settings.media.SetOnError(func(m gxcommon.IGXMedia, err error) {
		// log/handle error
		fmt.Fprintln(os.Stderr, "error:", err)
	})

	settings.media.SetOnTrace(func(m gxcommon.IGXMedia, e gxcommon.TraceEventArgs) {
		fmt.Printf("Trace: %s\n", e.String())
	})

	defer func() { _ = reader.Close() }()

	if len(settings.readObjects) == 0 {
		if err := reader.ReadAll(settings.outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		return
	}

	if err := reader.InitializeConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	for _, item := range settings.readObjects {
		obj := settings.client.Objects().FindByLN(enums.ObjectTypeNone, item.Key)
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
