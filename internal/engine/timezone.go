package engine

import (
	"bytes"
	"io"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func currentTimezone(s *Session, goos string) string {
	return currentTimezoneForZone(s, goos, time.Now().Format("MST"))
}

func currentTimezoneForZone(s *Session, goos, zone string) string {
	if goos != "windows" {
		return zone
	}
	if windowsZone := currentWindowsTimezone(goos, zone, currentWindowsAPICodepage(s), func() string { return currentWindowsRegistryCodepage(s) }); windowsZone != "" {
		return windowsZone
	}
	return zone
}

func currentWindowsTimezone(goos, zone, apiCodepage string, registryCodepage func() string) string {
	if goos != "windows" || zone == "" {
		return ""
	}
	if utf8.ValidString(zone) {
		return zone
	}
	codepage := apiCodepage
	if codepage == "" {
		codepage = registryCodepage()
	}
	decoded, ok := decodeWindowsCodepage(zone, codepage)
	if !ok {
		return zone
	}
	return decoded
}

func currentWindowsAPICodepage(s *Session) string {
	if s.goos() != "windows" {
		return ""
	}
	return firstNumber(s.commandOutput("cmd", "/c", "chcp"))
}

func currentWindowsRegistryCodepage(s *Session) string {
	if s.goos() != "windows" {
		return ""
	}
	return parseWindowsACPRegistry(s.commandOutput("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Control\Nls\CodePage`, "/v", "ACP"))
}

func parseWindowsACPRegistry(input string) string {
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "ACP" {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func firstNumber(input string) string {
	for field := range strings.FieldsSeq(input) {
		field = strings.TrimRight(field, ":.")
		if _, err := strconv.Atoi(field); err == nil {
			return field
		}
	}
	return ""
}

func decodeWindowsCodepage(value, codepage string) (string, bool) {
	decoder := windowsCodepageDecoder(codepage)
	if decoder == nil {
		return "", false
	}
	reader := transform.NewReader(bytes.NewReader([]byte(value)), decoder.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

func windowsCodepageDecoder(codepage string) encoding.Encoding {
	switch strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(codepage), "CP")) {
	case "437":
		return charmap.CodePage437
	case "850":
		return charmap.CodePage850
	case "1252":
		return charmap.Windows1252
	default:
		return nil
	}
}

// timezoneCoreFacts assembles the timezone category fact for the current host.
func timezoneCoreFacts(s *Session) []ResolvedFact {
	return []ResolvedFact{
		{Name: "timezone", Value: currentTimezone(s, runtime.GOOS)},
	}
}
