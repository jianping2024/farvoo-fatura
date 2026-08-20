package main

import _ "embed"

//go:embed tray_icon.ico
var trayIconICO []byte

//go:embed tray_icon_ok.ico
var trayIconOK []byte

//go:embed tray_icon_warn.ico
var trayIconWarn []byte

//go:embed tray_icon_err.ico
var trayIconErr []byte

func trayIconForLevel(level trayLevel) []byte {
	switch level {
	case trayLevelGreen:
		if len(trayIconOK) > 0 {
			return trayIconOK
		}
	case trayLevelYellow:
		if len(trayIconWarn) > 0 {
			return trayIconWarn
		}
	case trayLevelRed:
		if len(trayIconErr) > 0 {
			return trayIconErr
		}
	}
	return trayIconICO
}
