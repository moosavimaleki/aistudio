package chromeprocess

import "strings"

func displayFlags(headless string) []string {
	if headless == "1" || strings.EqualFold(headless, "true") {
		// Unified headless keeps the extension and the normal page runtime.
		return []string{"--headless=new", "--window-size=1280,900"}
	}
	return nil
}

// Keep page JavaScript, media, site isolation and the bridge extension intact.
// These switches reduce browser housekeeping and spare renderer processes.
func resourceFlags(mode string) []string {
	if mode == "0" || mode == "false" {
		return nil
	}
	return []string{
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-component-extensions-with-background-pages",
		"--disable-default-apps",
		"--disable-sync",
		"--disable-domain-reliability",
		"--renderer-process-limit=2",
		"--num-raster-threads=1",
	}
}
