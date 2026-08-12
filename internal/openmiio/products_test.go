package openmiio

import "testing"

func TestProductRegistry(t *testing.T) {
	product, ok := LookupProduct(ProductIDSensorHTO2)
	if !ok || product.Model != ModelSensorHTO2 || product.DefaultName == "" {
		t.Fatalf("product = %#v, ok=%v", product, ok)
	}
	toothbrush, ok := LookupProduct(ProductIDToothbrushT700i)
	if !ok || toothbrush.Kind != ProductKindToothbrush || toothbrush.Model != ModelToothbrushT700i {
		t.Fatalf("toothbrush = %#v, ok=%v", toothbrush, ok)
	}
	byModel, ok := LookupProductByModel(ModelToothbrushT700i)
	if !ok || byModel.ID != ProductIDToothbrushT700i {
		t.Fatalf("lookup by model = %#v, ok=%v", byModel, ok)
	}
	if SupportedProduct(2038) {
		t.Fatal("unknown product was accepted")
	}
}
