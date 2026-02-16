# Gurux Go Examples

This repository contains **example applications written in Go** that demonstrate how to use Gurux components—especially **GXDLMS for Go**—to communicate with DLMS/COSEM devices (electricity, gas, water meters) and/or DLMS simulators.

Gurux.DLMS for Go is a high-performance Go component for working with DLMS/COSEM devices. :contentReference[oaicite:1]{index=1}  
GXNet for Go provides TCP/UDP media used by many examples. :contentReference[oaicite:2]{index=2}

> If you are new to DLMS/COSEM, it’s recommended to read the Gurux DLMS/COSEM FAQ and basics before building your own application. :contentReference[oaicite:3]{index=3}

---

## Contents

Typical examples included in this repository:

- **DLMS Client example** (connects to a meter or simulator, reads objects)
- **DLMS Server / Simulator example** (acts like a meter, accepts client connections)
- Additional small demos (translator, HDLC/Wrapper framing, etc.) depending on the repo version

> Exact folder names may vary (for example: `client/`, `server/`, `examples/client`, `examples/server`).

---

## Prerequisites

- Go 1.20+ (recommended)
- Network access to a meter or simulator (TCP/UDP), or to a serial/optical probe if your example supports it
- Basic understanding of your target meter settings:
  - Host / Port
  - Authentication level
  - Client/Server address (HDLC) or Wrapper settings
  - Security (LLS/HLS, GMAC, keys), if applicable

---

## Dependencies

The examples typically use these Gurux Go modules:

- `github.com/Gurux/gxdlms-go` (DLMS/COSEM core) :contentReference[oaicite:4]{index=4}
- `github.com/Gurux/gxnet-go` (TCP/UDP media) :contentReference[oaicite:5]{index=5}
- `github.com/Gurux/gxcommon-go` (shared interfaces/utilities)

Install/update dependencies:

```bash
go mod tidy
