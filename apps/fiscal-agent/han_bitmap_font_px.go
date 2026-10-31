package main

// hanBitmapFontPxFromConfig — ONLY Agent source for Mesa ticket Han base px.
// Ignores print_jobs.payload.han_bitmap_font_px (legacy cloud field).
func hanBitmapFontPxFromConfig() int {
	cfg, err := loadConfig(defaultConfigPath())
	if err != nil || cfg == nil {
		return bitmapTextDefaultFontPx
	}
	return resolveHanBitmapFontPx(cfg.HanBitmapFontPx)
}
