package openmiio

import (
	"math"
	"testing"
)

func TestParseSensorHTO2BLEEvent(t *testing.T) {
	payload := []byte(`{"id":1,"method":"_async.ble_event","params":{"dev":{"did":"blt.3.test","mac":"AA:BB:CC:DD:EE:FF","pdid":5860},"evt":[{"eid":19457,"edata":"3333bb41"},{"eid":19458,"edata":"2d"},{"eid":18435,"edata":"58"}],"frmCnt":36,"gwts":1636208932}}`)
	updates, err := Parse(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 {
		t.Fatalf("len = %d", len(updates))
	}
	u := updates[0]
	if u.MAC != "aa:bb:cc:dd:ee:ff" || u.ProductID != ProductIDSensorHTO2 {
		t.Fatalf("identity = %#v", u)
	}
	if u.Temperature == nil || math.Abs(*u.Temperature-23.4) > 0.01 {
		t.Fatalf("temperature = %v", u.Temperature)
	}
	if u.Humidity == nil || *u.Humidity != 45 || u.Battery == nil || *u.Battery != 88 {
		t.Fatalf("measurements = %#v", u)
	}
}

func TestIgnoreOtherBLEProduct(t *testing.T) {
	payload := []byte(`{"method":"_async.ble_event","params":{"dev":{"did":"x","mac":"AA:BB:CC:DD:EE:FF","pdid":2038},"evt":[{"eid":19458,"edata":"2d"}],"frmCnt":1}}`)
	updates, err := Parse(payload)
	if err != nil || len(updates) != 0 {
		t.Fatalf("updates=%#v err=%v", updates, err)
	}
}

func TestParseMIoTProperties(t *testing.T) {
	payload := []byte(`{"method":"properties_changed","params":[{"did":"blt.3.test","siid":3,"piid":1001,"value":21.24},{"did":"blt.3.test","siid":3,"piid":1002,"value":48},{"did":"blt.3.test","siid":2,"piid":1003,"value":79}]}`)
	updates, err := Parse(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].Temperature == nil || *updates[0].Temperature != 21.2 {
		t.Fatalf("updates = %#v", updates)
	}
}

func TestRejectOutOfRangeMIoTProperty(t *testing.T) {
	payload := []byte(`{"method":"properties_changed","params":[{"did":"blt.3.test","siid":3,"piid":1002,"value":101}]}`)
	if _, err := Parse(payload); err == nil {
		t.Fatal("expected out-of-range humidity error")
	}
}
