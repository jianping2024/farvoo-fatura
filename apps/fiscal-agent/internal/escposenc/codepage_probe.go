package escposenc

import (
	"fmt"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// CodePageProbeSample is the pt-PT accent set printed on every probe row.
const CodePageProbeSample = "ã õ ç á é í ó ú â ê ô à Ã Õ Ç É"

// CodePageCandidate is one ESC t n row on the code-page self-test slip.
type CodePageCandidate struct {
	N       byte
	Name    string
	Charmap *charmap.Charmap
}

// CodePageCandidates — Epson ESC t numbering. WPC1252 (16) is the table that already failed in stores.
var CodePageCandidates = []CodePageCandidate{
	{N: 2, Name: "CP850", Charmap: charmap.CodePage850},
	{N: 3, Name: "CP860", Charmap: charmap.CodePage860},
	{N: 16, Name: "WPC1252", Charmap: charmap.Windows1252},
	{N: 19, Name: "CP858", Charmap: charmap.CodePage858},
}

// CodePageProbeRows is the ONLY code-page probe body: per candidate, ESC t n + ASCII label +
// CodePageProbeSample encoded in that table, then LF. Restores WPC1252 at the end.
func CodePageProbeRows() []byte {
	var out []byte
	for _, c := range CodePageCandidates {
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

// HanProbeRows is the ONLY Chinese firmware probe body. Row order matters:
// A raw GBK (printers that boot in Han mode) → B FS & + GBK + FS . → C ESC 9 1 + UTF-8,
// then ESC @ + WPC1252 so a printer stuck in UTF-8/Han mode cannot taint what follows.
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
