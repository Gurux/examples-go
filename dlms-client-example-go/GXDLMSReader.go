package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Gurux/gxcommon-go"
	dlms "github.com/Gurux/gxdlms-go"
	"github.com/Gurux/gxdlms-go/enums"
	"github.com/Gurux/gxdlms-go/objects"
	"github.com/Gurux/gxdlms-go/types"
)

type GXDLMSReader struct {
	WaitTime          int
	RetryCount        int
	InvocationCounter string

	media          gxcommon.IGXMedia
	trace          gxcommon.TraceLevel
	client         *dlms.GXDLMSSecureClient
	traceFile      string
	OnNotification func(any)
}

// NewGXDLMSReader creates a new DLMS reader.
func NewGXDLMSReader(
	client *dlms.GXDLMSSecureClient,
	media gxcommon.IGXMedia,
	trace gxcommon.TraceLevel,
	invocationCounter string,
	waitTime int,
) *GXDLMSReader {
	if waitTime <= 0 {
		waitTime = 5000
	}
	return &GXDLMSReader{
		WaitTime:          waitTime,
		RetryCount:        3,
		InvocationCounter: invocationCounter,
		media:             media,
		trace:             trace,
		client:            client,
		traceFile:         "trace.txt",
	}
}

// InitializeConnection opens the transport and performs DLMS association.
func (r *GXDLMSReader) InitializeConnection() error {
	r.writeTrace(fmt.Sprintf("Standard: %s", r.client.Standard().String()))
	r.logSecurityInfo()

	if !r.media.IsOpen() {
		if err := r.media.Open(); err != nil {
			return err
		}
	}

	if err := r.updateFrameCounter(); err != nil {
		return err
	}
	if err := r.initializeOpticalHead(); err != nil {
		return err
	}
	if err := r.SNRMRequest(); err != nil {
		return err
	}

	if r.client.PreEstablishedConnection() {
		return nil
	}

	if err := r.AarqRequest(); err != nil {
		return err
	}
	r.writeTrace(fmt.Sprintf("Conformance: %s", r.client.NegotiatedConformance().String()))
	return nil
}

func (r *GXDLMSReader) logSecurityInfo() {
	c := r.client.Ciphering()
	if c == nil || c.Security() == enums.SecurityNone {
		return
	}
	r.writeTrace(fmt.Sprintf("Security: %s", c.Security().String()))
	r.writeTrace("System title: " + types.ToHex(c.SystemTitle(), true))
	r.writeTrace("Authentication key: " + types.ToHex(c.AuthenticationKey(), true))
	r.writeTrace("Block cipher key: " + types.ToHex(c.BlockCipherKey(), true))
	if dk := c.DedicatedKey(); len(dk) != 0 {
		r.writeTrace("Dedicated key: " + types.ToHex(dk, true))
	}
}

func (r *GXDLMSReader) initializeOpticalHead() error {
	// Kept as extension point for serial mode E initialization.
	return nil
}

// GetAssociationView reads association view from the meter or from cache file.
func (r *GXDLMSReader) GetAssociationView(outputFile string) (bool, error) {
	if outputFile != "" {
		if _, err := os.Stat(outputFile); err == nil {
			r.client.Objects().Clear()
			if err = r.client.Objects().LoadFromFile(outputFile); err == nil && len(*r.client.Objects()) != 0 {
				return false, nil
			}
			if err != nil {
				_ = os.Remove(outputFile)
			}
		}
	}

	frames, err := r.client.GetObjectsRequest()
	if err != nil {
		return false, err
	}
	reply := dlms.NewGXReplyData()
	if _, err = r.ReadDataBlocks(frames, reply); err != nil {
		return false, err
	}
	if _, err = r.client.ParseObjects(reply.Data, true); err != nil {
		return false, err
	}

	if !r.client.UseLogicalNameReferencing() {
		if snObj := r.client.Objects().FindBySN(0xFA00); snObj != nil {
			if sn, ok := snObj.(*objects.GXDLMSAssociationShortName); ok && sn.Version > 0 {
				_, _ = r.Read(sn, 3)
			}
		}
	}

	if outputFile != "" {
		ret := r.client.Objects().SaveToFile(outputFile, &objects.GXXmlWriterSettings{Values: false})
		if ret != nil {
			return false, err
		}
	}
	return true, nil
}

// GetScalersAndUnits reads scaler/unit attributes from register objects.
func (r *GXDLMSReader) GetScalersAndUnits() {
	objs := r.client.Objects().GetObjects2([]enums.ObjectType{
		enums.ObjectTypeRegister,
		enums.ObjectTypeExtendedRegister,
		enums.ObjectTypeDemandRegister,
	})
	for _, it := range objs {
		idx := 3
		if it.Base().ObjectType() == enums.ObjectTypeDemandRegister {
			idx = 4
		}
		if !r.client.CanRead(it.Base(), idx) {
			continue
		}
		if _, err := r.Read(it, idx); err != nil && r.trace > gxcommon.TraceLevelWarning {
			r.writeTrace(fmt.Sprintf("Failed reading scaler/unit %s:%d: %v", it.Base().LogicalName(), idx, err))
		}
	}
}

// GetProfileGenericColumns reads profile generic capture object metadata.
func (r *GXDLMSReader) GetProfileGenericColumns() {
	for _, it := range r.client.Objects().GetObjects(enums.ObjectTypeProfileGeneric) {
		if _, err := r.Read(it, 3); err != nil && r.trace > gxcommon.TraceLevelWarning {
			r.writeTrace(fmt.Sprintf("Failed reading profile columns %s: %v", it.Base().LogicalName(), err))
		}
	}
}

// ShowValue logs one read attribute value.
func (r *GXDLMSReader) ShowValue(val any, pos int) {
	if r.trace <= gxcommon.TraceLevelWarning {
		return
	}
	formatted := fmt.Sprint(val)
	if b, ok := val.([]byte); ok {
		formatted = types.ToHex(b, true)
	} else if arr, ok := val.([]any); ok {
		parts := make([]string, 0, len(arr))
		for _, item := range arr {
			if rb, ok2 := item.([]byte); ok2 {
				parts = append(parts, types.ToHex(rb, true))
			} else {
				parts = append(parts, fmt.Sprint(item))
			}
		}
		formatted = "[" + strings.Join(parts, ", ") + "]"
	}
	r.writeTrace(fmt.Sprintf("Index: %d Value: %s", pos, formatted))
}

// GetProfileGenerics reads profile generic entries metadata and sample rows.
func (r *GXDLMSReader) GetProfileGenerics() {
	for _, it := range r.client.Objects().GetObjects(enums.ObjectTypeProfileGeneric) {
		pg, ok := it.(*objects.GXDLMSProfileGeneric)
		if !ok {
			continue
		}
		if r.client.CanRead(pg.Base(), 7) {
			_, _ = r.Read(pg, 7)
		}
		if r.client.CanRead(pg.Base(), 8) {
			_, _ = r.Read(pg, 8)
		}
		if len(pg.CaptureObjects) == 0 || pg.EntriesInUse == 0 {
			continue
		}
		if rows, err := r.ReadRowsByEntry(pg, 1, 1); err == nil && r.trace > gxcommon.TraceLevelWarning {
			r.writeTrace(fmt.Sprintf("Profile %s first row: %v", pg.Base().LogicalName(), rows))
		}
	}
}

// GetCompactData reads common compact data attributes.
func (r *GXDLMSReader) GetCompactData() {
	for _, it := range r.client.Objects().GetObjects(enums.ObjectTypeCompactData) {
		for _, idx := range []int{3, 5, 2} {
			if r.client.CanRead(it.Base(), idx) {
				_, _ = r.Read(it, idx)
			}
		}
	}
}

// GetReadOut reads all readable attributes except profile generic data rows.
func (r *GXDLMSReader) GetReadOut() {
	for _, it := range *r.client.Objects() {
		if it.Base().ObjectType() == enums.ObjectTypeProfileGeneric {
			continue
		}
		for _, pos := range it.GetAttributeIndexToRead(true) {
			if !r.client.CanRead(it.Base(), pos) {
				continue
			}
			val, err := r.Read(it, pos)
			if err != nil {
				if r.trace > gxcommon.TraceLevelError {
					r.writeTrace(fmt.Sprintf("Read failed %s:%d: %v", it.Base().LogicalName(), pos, err))
				}
				continue
			}
			r.ShowValue(val, pos)
		}
	}
}

func (r *GXDLMSReader) updateFrameCounter() error {
	// Invocation counter update logic can be added here if meter requires it.
	return nil
}

// ReadAll performs complete read sequence and saves objects to file if outputFile is not empty.
func (r *GXDLMSReader) ReadAll(outputFile string) error {
	if err := r.InitializeConnection(); err != nil {
		return err
	}
	readFromDevice, err := r.GetAssociationView(outputFile)
	if err != nil {
		return err
	}
	if readFromDevice {
		r.GetScalersAndUnits()
		r.GetProfileGenericColumns()
	}
	r.GetCompactData()
	r.GetReadOut()
	r.GetProfileGenerics()
	if outputFile != "" {
		_ = r.client.Objects().SaveToFile(outputFile, &objects.GXXmlWriterSettings{
			UseMeterTime:        true,
			IgnoreDefaultValues: false,
		})
	}
	return nil
}

// SNRMRequest sends SNRM and parses UA.
func (r *GXDLMSReader) SNRMRequest() error {
	reply := dlms.NewGXReplyData()
	frame, err := r.client.SNRMRequest()
	if err != nil {
		return err
	}
	if frame == nil {
		return nil
	}
	if err := r.ReadDataBlock(frame, reply); err != nil {
		return err
	}
	if r.trace > gxcommon.TraceLevelInfo {
		r.writeTrace("Parsing UA reply")
	}
	if err := r.client.ParseUAResponse(reply.Data); err != nil {
		return err
	}
	return nil
}

// AarqRequest sends AARQ and optional HLS application association.
func (r *GXDLMSReader) AarqRequest() error {
	reply := dlms.NewGXReplyData()
	frames, err := r.client.AARQRequest()
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		return nil
	}
	for _, frame := range frames {
		reply.Clear()
		if err := r.ReadDataBlock(frame, reply); err != nil {
			return err
		}
	}
	if err := r.client.ParseAAREResponse(reply.Data); err != nil {
		return err
	}
	if r.client.Authentication() > enums.AuthenticationLow {
		hls, err := r.client.GetApplicationAssociationRequest()
		if err != nil {
			return err
		}
		for _, frame := range hls {
			reply.Clear()
			if err := r.ReadDataBlock(frame, reply); err != nil {
				return err
			}
		}
		if err := r.client.ParseApplicationAssociationResponse(reply.Data); err != nil {
			return err
		}
	}
	return nil
}

// ReadDLMSPacket sends one DLMS packet and waits until one complete response is parsed.
func (r *GXDLMSReader) ReadDLMSPacket(data []byte, reply *dlms.GXReplyData) error {
	if reply == nil {
		return errors.New("reply is nil")
	}
	if data == nil && !reply.IsStreaming() {
		return nil
	}

	notify := dlms.NewGXReplyData()
	reply.Error = 0
	eop := any(byte(0x7E))
	if r.client.InterfaceType() != enums.InterfaceTypeHDLC &&
		r.client.InterfaceType() != enums.InterfaceTypeHdlcWithModeE {
		eop = nil
	}

	unlock := r.media.GetSynchronous()
	defer unlock()

	var err error
	rd := types.NewGXByteBuffer()
	attempt := 0
	succeeded := false
	p := gxcommon.NewReceiveParameters[[]byte]()
	p.EOP = eop
	p.Count = r.client.GetFrameSize(rd)
	p.AllData = true
	p.WaitTime = r.WaitTime
	for !succeeded && attempt != 3 {
		if !reply.IsStreaming() {
			if len(data) == 0 {
				return errors.New("packet is empty")
			}
			r.writeTrace("TX:\t" + time.Now().Format("15:04:05.000") + "\t" + types.ToHex(data, true))
			if err := r.media.Send(data, ""); err != nil {
				return err
			}
			succeeded, err = r.media.Receive(p)
			if err != nil {
				return err
			}
			if !succeeded {
				attempt++
				if attempt >= r.RetryCount {
					return errors.New("failed to receive reply from the device in given time")
				}
				//If EOP is not set read one byte at time.
				if p.EOP == nil {
					p.Count = 1
				}
				//Try to read again...
				log.Printf("Data send failed. Try to resend %d/3\n", attempt)
			}
		}
	}

	err = rd.Set(p.Reply.([]byte))
	if err != nil {
		return err
	}
	attempt = 0
	//Loop until whole COSEM packet is received.
	complete := false
	for {
		complete, err = r.client.GetData(rd, reply, notify)
		if err != nil {
			return err
		}
		if complete {
			break
		}
		if notify.IsComplete() && !notify.IsMoreData() {
			if r.OnNotification != nil {
				r.OnNotification(notify.Value)
			}
			notify.Clear()
		}
		if p.EOP == nil {
			p.Count = r.client.GetFrameSize(rd)
		}
		for {
			succeeded, err = r.media.Receive(p)
			if err != nil {
				return err
			}
			if succeeded {
				break
			}
			attempt++
			if attempt >= r.RetryCount {
				return errors.New("failed to receive reply from the device in given time")
			}
			p.Reply = nil
			if err := r.media.Send(data, ""); err != nil {
				return err
			}
			//Try to read again...
			log.Printf("Data send failed. Try to resend %d/3\n", attempt)
		}
		err = rd.Set(p.Reply.([]byte))
		if err != nil {
			return err
		}
	}
	r.writeTrace("RX:\t" + time.Now().Format("15:04:05.000") + "\t" + rd.String())
	if reply.Error != 0 {
		if reply.Error == int(enums.ErrorCodeRejected) {
			time.Sleep(time.Second)
			return r.ReadDLMSPacket(data, reply)
		}
		return fmt.Errorf("dlms error %s", enums.ErrorCode(reply.Error).String())
	}
	return nil
}

// ReadDataBlocks sends one or more data blocks to meter.
func (r *GXDLMSReader) ReadDataBlocks(blocks [][]byte, reply *dlms.GXReplyData) (bool, error) {
	if blocks == nil {
		return true, nil
	}
	for _, b := range blocks {
		reply.Clear()
		if err := r.ReadDataBlock(b, reply); err != nil {
			return false, err
		}
	}
	return reply.Error == 0, nil
}

// ReadDataBlock sends one block and receives all follow-up blocks.
func (r *GXDLMSReader) ReadDataBlock(data []byte, reply *dlms.GXReplyData) error {
	if err := r.ReadDLMSPacket(data, reply); err != nil {
		return err
	}
	unlock := r.media.GetSynchronous()
	defer unlock()

	for (reply.IsMoreData() &&
		(r.client.ConnectionState() != enums.ConnectionStateNone || r.client.PreEstablishedConnection())) ||
		(reply.Error == 0 && reply.Data.Size() == 0) {
		if reply.IsStreaming() {
			data = nil
		} else {
			next, err := r.client.ReceiverReady(reply)
			if err != nil {
				return err
			}
			data = next
		}
		if err := r.ReadDLMSPacket(data, reply); err != nil {
			return err
		}
	}
	return nil
}

// Read reads one COSEM attribute.
func (r *GXDLMSReader) Read(obj objects.IGXDLMSBase, attributeIndex int) (any, error) {
	if obj == nil {
		return nil, errors.New("object is nil")
	}
	if !r.client.CanRead(obj.Base(), attributeIndex) {
		return nil, fmt.Errorf("cannot read %s index %d", obj.Base().String(), attributeIndex)
	}
	frames, err := r.client.Read(obj, attributeIndex)
	if err != nil {
		return nil, err
	}
	reply := dlms.NewGXReplyData()
	if _, err = r.ReadDataBlocks(frames, reply); err != nil {
		return nil, err
	}
	dt, err := obj.GetDataType(attributeIndex)
	if err == nil && dt == enums.DataTypeNone {
		obj.Base().SetDataType(attributeIndex, reply.DataType)
	}
	return r.client.UpdateValue(obj, attributeIndex, reply.Value, nil)
}

// ReadList reads multiple attributes in one request sequence.
func (r *GXDLMSReader) ReadList(list []types.GXKeyValuePair[objects.IGXDLMSBase, int]) error {
	frames, err := r.client.ReadList(list)
	if err != nil {
		return err
	}
	reply := dlms.NewGXReplyData()
	values := make([]any, 0, len(list))
	for _, frame := range frames {
		if err = r.ReadDataBlock(frame, reply); err != nil {
			return err
		}
		if !reply.IsMoreData() {
			if v, ok := reply.Value.([]any); ok {
				values = append(values, v...)
			}
		}
		reply.Clear()
	}
	if len(values) != len(list) {
		return fmt.Errorf("invalid reply count: got %d, expected %d", len(values), len(list))
	}
	return r.client.UpdateValues(list, values)
}

// Write writes one attribute value to the meter.
func (r *GXDLMSReader) Write(obj objects.IGXDLMSBase, attributeIndex int) error {
	if obj == nil {
		return errors.New("object is nil")
	}
	if !r.client.CanWrite(obj.Base(), attributeIndex) {
		return fmt.Errorf("cannot write %s index %d", obj.Base().String(), attributeIndex)
	}
	frames, err := r.client.Write(obj, attributeIndex)
	if err != nil {
		return err
	}
	reply := dlms.NewGXReplyData()
	_, err = r.ReadDataBlocks(frames, reply)
	return err
}

// Method invokes one COSEM method.
func (r *GXDLMSReader) Method(obj *objects.GXDLMSObject, methodIndex int, value any) error {
	if obj == nil {
		return errors.New("object is nil")
	}
	if !r.client.CanInvoke(obj, methodIndex) {
		return fmt.Errorf("cannot invoke %s method %d", obj.String(), methodIndex)
	}
	frames, err := r.client.Method(obj, methodIndex, value)
	if err != nil {
		return err
	}
	reply := dlms.NewGXReplyData()
	_, err = r.ReadDataBlocks(frames, reply)
	return err
}

// ReadRowsByEntry reads profile generic rows by entry range.
func (r *GXDLMSReader) ReadRowsByEntry(pg *objects.GXDLMSProfileGeneric, index, count uint32) ([]any, error) {
	frames, err := r.client.ReadRowsByEntry(pg, index, count)
	if err != nil {
		return nil, err
	}
	reply := dlms.NewGXReplyData()
	if _, err = r.ReadDataBlocks(frames, reply); err != nil {
		return nil, err
	}
	value, err := r.client.UpdateValue(pg, 2, reply.Value, nil)
	if err != nil {
		return nil, err
	}
	rows, _ := value.([]any)
	return rows, nil
}

// ReadRowsByRange reads profile generic rows by time range.
func (r *GXDLMSReader) ReadRowsByRange(pg *objects.GXDLMSProfileGeneric, start, end *types.GXDateTime) ([]any, error) {
	frames, err := r.client.ReadRowsByRange(pg, start, end)
	if err != nil {
		return nil, err
	}
	reply := dlms.NewGXReplyData()
	if _, err = r.ReadDataBlocks(frames, reply); err != nil {
		return nil, err
	}
	value, err := r.client.UpdateValue(pg, 2, reply.Value, nil)
	if err != nil {
		return nil, err
	}
	rows, _ := value.([]any)
	return rows, nil
}

// Release sends release request if the connection type needs it.
func (r *GXDLMSReader) Release() error {
	if r.client == nil || r.media == nil {
		return nil
	}
	if r.client.InterfaceType() == enums.InterfaceTypeWRAPPER ||
		(r.client.Ciphering().Security() != enums.SecurityNone && !r.client.PreEstablishedConnection()) {
		frames, err := r.client.ReleaseRequest()
		if err != nil {
			return err
		}
		reply := dlms.NewGXReplyData()
		_, err = r.ReadDataBlocks(frames, reply)
		return err
	}
	return nil
}

// Disconnect sends disconnect request.
func (r *GXDLMSReader) Disconnect() error {
	if r.client == nil || r.media == nil {
		return nil
	}
	err := r.Release()
	if err != nil {
		r.writeTrace(fmt.Sprintf("Release failed: %v\n", err))
		// Ignore release failures for meters that do not support release.
	}
	frame, err := r.client.DisconnectRequest()
	if err != nil {
		return err
	}
	if frame == nil {
		return nil
	}
	reply := dlms.NewGXReplyData()
	return r.ReadDLMSPacket(frame, reply)
}

// Close closes connection and media.
func (r *GXDLMSReader) Close() error {
	if r.media == nil {
		return nil
	}
	_ = r.Disconnect()
	err := r.media.Close()
	r.media = nil
	r.client = nil
	return err
}

func (r *GXDLMSReader) writeTrace(line string) {
	if r.trace > gxcommon.TraceLevelInfo {
		fmt.Println(line)
	}
	if r.traceFile == "" {
		return
	}
	f, err := os.OpenFile(r.traceFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Printf("failed to close trace file: %v\n", closeErr)
		}
	}()
	_, _ = fmt.Fprintln(f, line)
}
