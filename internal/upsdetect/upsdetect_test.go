package upsdetect

import "testing"

func TestParseScanSingleDevice(t *testing.T) {
	out := `[nutdev1]
	driver = "usbhid-ups"
	port = "auto"
	vendorid = "0764"
	productid = "0501"
	vendor = "CPS"
	product = "CP1500PFCLCD"
`
	got := parseScan(out)
	if len(got) != 1 {
		t.Fatalf("parseScan returned %d devices, want 1", len(got))
	}
	d := got[0]
	if d.Driver != "usbhid-ups" {
		t.Errorf("driver = %q, want usbhid-ups", d.Driver)
	}
	if d.Port != "auto" {
		t.Errorf("port = %q, want auto", d.Port)
	}
	if d.VendorID != "0764" || d.ProductID != "0501" {
		t.Errorf("vendor/product id = %q/%q, want 0764/0501", d.VendorID, d.ProductID)
	}
	if got, want := d.Description(), "CPS CP1500PFCLCD"; got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestParseScanMultipleDevices(t *testing.T) {
	out := `[nutdev1]
	driver = "usbhid-ups"
	port = "auto"
[nutdev2]
	driver = "blazer_usb"
	port = "auto"
`
	got := parseScan(out)
	if len(got) != 2 {
		t.Fatalf("parseScan returned %d devices, want 2", len(got))
	}
	if got[0].Driver != "usbhid-ups" || got[1].Driver != "blazer_usb" {
		t.Errorf("drivers = %q, %q; want usbhid-ups, blazer_usb", got[0].Driver, got[1].Driver)
	}
}

func TestParseScanEmptyAndJunk(t *testing.T) {
	for name, in := range map[string]string{
		"empty":          "",
		"whitespace":     "\n\n   \n",
		"no driver":      "[nutdev1]\n\tport = \"auto\"\n", // section without a driver is dropped
		"comment only":   "# nothing here\n",
		"stray key/vals": "driver = \"x\"\n", // no section header → ignored
	} {
		if got := parseScan(in); len(got) != 0 {
			t.Errorf("%s: parseScan = %d devices, want 0 (%+v)", name, len(got), got)
		}
	}
}

func TestDescriptionPartial(t *testing.T) {
	if got := (DetectedUPS{Vendor: "CPS"}).Description(); got != "CPS" {
		t.Errorf("vendor-only Description() = %q, want CPS", got)
	}
	if got := (DetectedUPS{}).Description(); got != "" {
		t.Errorf("empty Description() = %q, want empty", got)
	}
}
