package escposenc

import (
	"fmt"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// CodePageProbeSample is the pt-PT accent set printed on every probe row.
const CodePageProbeSample = "ã õ ç á é í ó ú â ê ô à Ã Õ Ç É"

// codePageCandidates — Epson ESC t numbering; one probe row each.
var codePageCandidates = []struct {
	N       byte
	Name    string
	Charmap *charmap.Charmap
}{
	{N: 2, Name: "CP850", Charmap: charmap.CodePage850},
	{N: 3, Name: "CP860", Charmap: charmap.CodePage860},
	{N: 16, Name: "WPC1252", Charmap: charmap.Windows1252},
	{N: 19, Name: "CP858", Charmap: charmap.CodePage858},
}

// CodePageProbeRows is the ONLY code-page probe body: per candidate, ESC t n + ASCII label +
// CodePageProbeSample encoded in that table, then LF. Restores WPC1252 at the end.
func CodePageProbeRows() []byte {
	var out []byte
	for _, c := range codePageCandidates {
		out = append(out, SelectCodeTable(c.N)...)
		out = append(out, fmt.Sprintf("n=%-2d %-8s ", c.N, c.Name)...)
		sample, _ := encoding.ReplaceUnsupported(c.Charmap.NewEncoder()).Bytes([]byte(CodePageProbeSample))
		out = append(out, sample...)
		out = append(out, '\n')
	}
	return append(out, SelectCodeTable(CodeTableWPC1252)...)
}

// HanProbeSample is the Chinese set printed on every Han probe row.
const HanProbeSample = "宫保鸡丁 餐厅 打印测试"

// SizeProbe samples — short so 1×2 still fits one line on POS-80.
const (
	SizeProbePTSample    = "Preco Agua"
	SizeProbeZHSample    = "宫保鸡丁"
	SizeProbeMixSample   = "Água 少放盐"
	SizeProbeMixNote     = "Observação: 少放盐"
	SizeProbeMixMasthead = "Mesa 桌 01"
)

// GS ! n — same Font A size command as writeBody1x2 (double height = 0x01).
func selectCharSize(n byte) []byte {
	return []byte{0x1D, 0x21, n}
}

// FS ! n — Kanji/Han print mode on many Chinese ESC/POS clones (0x04 = double height).
func selectHanCharMode(n byte) []byte {
	return []byte{0x1C, 0x21, n}
}

// HanProbeRows is the ONLY Chinese firmware probe body. Row order matters:
// A raw GBK (printers that boot in Han mode) → B FS & + GBK + FS . → C ESC 9 1 + UTF-8,
// then ESC @ + WPC1252 so a printer stuck in UTF-8/Han mode cannot taint what follows.
// Store read (pirata CODE PAGE TEST): B matches Chinese reference; A and C do not.
func HanProbeRows() []byte {
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(HanProbeSample))
	var out []byte
	out = append(out, "A GBK       "...)
	out = append(out, gbk...)
	out = append(out, '\n')
	out = append(out, "B FS&+GBK   "...)
	out = append(out, 0x1C, 0x26)
	out = append(out, gbk...)
	out = append(out, 0x1C, 0x2E, '\n')
	out = append(out, "C UTF-8     "...)
	out = append(out, 0x1B, 0x39, 0x01)
	out = append(out, HanProbeSample...)
	out = append(out, '\n', 0x1B, 0x40)
	return append(out, SelectCodeTable(CodeTableWPC1252)...)
}

// SizeProbeRows compares Font A 1×2 (GS !) against Chinese B (FS&+GBK) at 1×1 / GS! 1×2 / FS! 1×2.
// Read: if ZH B GS!1x2 or ZH B FS!1x2 is about as tall as PT 1x2, that size path works for Han on this firmware.
func SizeProbeRows() []byte {
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(SizeProbeZHSample))
	pt, _ := encoding.ReplaceUnsupported(charmap.Windows1252.NewEncoder()).Bytes([]byte(SizeProbePTSample))

	var out []byte
	out = append(out, SelectCodeTable(CodeTableWPC1252)...)
	out = append(out, selectCharSize(0)...)
	out = append(out, "PT 1x1      "...)
	out = append(out, pt...)
	out = append(out, '\n')

	out = append(out, selectCharSize(0x01)...) // double height only (same as writeBody1x2)
	out = append(out, "PT 1x2      "...)
	out = append(out, pt...)
	out = append(out, '\n')
	out = append(out, selectCharSize(0)...)

	// ZH B 1×1 — encoding path that matched Chinese reference at pirata.
	out = append(out, "ZH B 1x1    "...)
	out = append(out, 0x1C, 0x26)
	out = append(out, selectCharSize(0)...)
	out = append(out, selectHanCharMode(0)...)
	out = append(out, gbk...)
	out = append(out, 0x1C, 0x2E, '\n')

	// ZH B + GS ! 1×2 (same command as Portuguese body).
	out = append(out, "ZH B GS!1x2 "...)
	out = append(out, 0x1C, 0x26)
	out = append(out, selectCharSize(0x01)...)
	out = append(out, gbk...)
	out = append(out, selectCharSize(0)...)
	out = append(out, 0x1C, 0x2E, '\n')

	// ZH B + FS ! double-height (Kanji size; some clones ignore GS ! on Han).
	out = append(out, "ZH B FS!1x2 "...)
	out = append(out, 0x1C, 0x26)
	out = append(out, selectHanCharMode(0x04)...)
	out = append(out, gbk...)
	out = append(out, selectHanCharMode(0)...)
	out = append(out, 0x1C, 0x2E, '\n')

	// MIX rows — the real auto path for pt accent + Han on one line (HanFirmwareLine).
	// Read: pt and Chinese on each row same height, no garble; 1x2 rows as tall as PT 1x2.
	out = append(out, "MIX 1x1     "...)
	out = append(out, HanFirmwareLine(SizeProbeMixSample, 0, HanSize1x1)...)
	out = append(out, '\n')

	out = append(out, selectCharSize(0x01)...)
	out = append(out, "MIX 1x2     "...)
	out = append(out, HanFirmwareLine(SizeProbeMixSample, 0x01, HanSize1x2)...)
	out = append(out, '\n')

	// Station note style: 1×2 + underline (ESC - 1 for Font A, FS ! bit 7 for Han).
	out = append(out, "MIX 1x2 U   "...)
	out = append(out, 0x1B, 0x2D, 0x01)
	out = append(out, HanFirmwareLine(SizeProbeMixNote, 0x01, HanSize1x2|0x80)...)
	out = append(out, 0x1B, 0x2D, 0x00, '\n')
	out = append(out, selectCharSize(0)...)

	// Masthead style 2×2 (GS ! 0x11 / FS ! 0x0C); 24 cols, label on its own line.
	out = append(out, "MIX 2x2:\n"...)
	out = append(out, selectCharSize(0x11)...)
	out = append(out, HanFirmwareLine(SizeProbeMixMasthead, 0x11, HanSize2x2)...)
	out = append(out, '\n')
	out = append(out, selectCharSize(0)...)

	out = append(out, 0x1B, 0x40) // ESC @ — leave Han/size modes
	return append(out, SelectCodeTable(CodeTableWPC1252)...)
}
