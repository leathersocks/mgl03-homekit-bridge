package openmiio

// Product describes a Xiaomi BLE model supported by the bridge. Keeping this
// metadata in a registry makes future decoders and HomeKit accessory factories
// additive instead of spreading PDID checks through the runtime.
type Product struct {
	ID           int
	Model        string
	Manufacturer string
	DefaultName  string
}

var products = map[int]Product{
	ProductIDSensorHTO2: {
		ID:           ProductIDSensorHTO2,
		Model:        ModelSensorHTO2,
		Manufacturer: "Miaomiaoce",
		DefaultName:  "Miaomiaoce Temperature Sensor",
	},
}

func LookupProduct(productID int) (Product, bool) {
	product, ok := products[productID]
	return product, ok
}

func SupportedProduct(productID int) bool {
	_, ok := LookupProduct(productID)
	return ok
}
