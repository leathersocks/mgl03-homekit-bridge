package openmiio

import "testing"

func TestProductRegistry(t *testing.T) {
	product, ok := LookupProduct(ProductIDSensorHTO2)
	if !ok || product.Model != ModelSensorHTO2 || product.DefaultName == "" {
		t.Fatalf("product = %#v, ok=%v", product, ok)
	}
	if SupportedProduct(6032) {
		t.Fatal("unsupported product was accepted")
	}
}
