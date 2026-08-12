package openmiio

type ProductKind string

const (
	ProductKindClimate    ProductKind = "climate"
	ProductKindToothbrush ProductKind = "toothbrush"
)

const (
	ProductIDSensorHTO2 = 5860
	ModelSensorHTO2     = "miaomiaoce.sensor_ht.o2"

	ProductIDToothbrushT700i = 6032
	ModelToothbrushT700i     = "k0918.toothbrush.t700i"
)

type bleDecoder func(events []BLEEvent, update *Update) []string

// Product describes a Xiaomi BLE model supported by the bridge. Keeping this
// metadata in a registry makes future decoders and HomeKit accessory factories
// additive instead of spreading PDID checks through the runtime.
type Product struct {
	ID           int
	Kind         ProductKind
	Model        string
	Manufacturer string
	DefaultName  string
	decode       bleDecoder
}

var products = map[int]Product{
	ProductIDSensorHTO2: {
		ID:           ProductIDSensorHTO2,
		Kind:         ProductKindClimate,
		Model:        ModelSensorHTO2,
		Manufacturer: "Miaomiaoce",
		DefaultName:  "Miaomiaoce Temperature Sensor",
		decode:       decodeSensorHTO2,
	},
	ProductIDToothbrushT700i: {
		ID:           ProductIDToothbrushT700i,
		Kind:         ProductKindToothbrush,
		Model:        ModelToothbrushT700i,
		Manufacturer: "Xiaomi",
		DefaultName:  "Xiaomi Toothbrush T700i",
		decode:       decodeToothbrushT700i,
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

func LookupProductByModel(model string) (Product, bool) {
	for _, product := range products {
		if product.Model == model {
			return product, true
		}
	}
	return Product{}, false
}
