package engine

import (
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type uptimeInfo struct {
	Duration time.Duration
	Known    bool
}

func uptimeString(uptime uptimeInfo) string {
	if !uptime.Known {
		return "unknown"
	}
	totalHours := int(uptime.Duration.Hours())
	days := totalHours / 24
	if days == 0 {
		minutes := int(uptime.Duration.Minutes()) % 60
		return fmt.Sprintf("%d:%02d hours", totalHours, minutes)
	}
	if days == 1 {
		return "1 day"
	}
	return strconv.Itoa(days) + " days"
}

func probeUptime(s *Session) uptimeInfo {
	return currentUptimeInfo(s, runtime.GOOS, s.readFile, s.commandOutput, time.Now)
}

func currentUptime(s *Session, goos string, readFile fileReader, run commandRunner, now func() time.Time) time.Duration {
	return currentUptimeInfo(s, goos, readFile, run, now).Duration
}

func currentUptimeInfo(s *Session, goos string, readFile fileReader, run commandRunner, now func() time.Time) uptimeInfo {
	if goos == "windows" {
		return currentWindowsUptime(goos, run, s.logr())
	}
	if goos == "linux" {
		virtual := detectLinuxVirtualization(currentLinuxVirtualizationInputWithCommands(s, run))
		return currentLinuxUptimeInfo(readFile, run, now, virtual.Name == "docker")
	}
	return currentPosixUptime(readFile, run, now)
}

func currentLinuxUptimeInfo(readFile fileReader, run commandRunner, now func() time.Time, docker bool) uptimeInfo {
	if docker {
		seconds := parseDockerElapsedTimeSeconds(run("ps", "-o", "etime=", "-p", "1"))
		if seconds > 0 {
			return uptimeInfo{Duration: time.Duration(seconds) * time.Second, Known: true}
		}
	}
	return currentPosixUptime(readFile, run, now)
}

func currentPosixUptime(readFile fileReader, run commandRunner, now func() time.Time) uptimeInfo {
	if uptime := uptimeFromProc(readFile); uptime > 0 {
		return uptimeInfo{Duration: uptime, Known: true}
	}
	if uptime := uptimeFromKernelBoottime(run("sysctl", "-n", "kern.boottime"), now); uptime > 0 {
		return uptimeInfo{Duration: uptime, Known: true}
	}
	if out := run("uptime"); out != "" {
		seconds := parseUptimeCommandSeconds(out)
		if seconds > 0 {
			return uptimeInfo{Duration: time.Duration(seconds) * time.Second, Known: true}
		}
	}
	return uptimeInfo{}
}

func currentWindowsUptime(goos string, run commandRunner, log *slog.Logger) uptimeInfo {
	if goos != "windows" {
		return uptimeInfo{}
	}
	values := parseWindowsWMIValues(windowsWMIOutput(run, "os", "LocalDateTime,LastBootUpTime"))
	if len(values) == 0 {
		log.Debug("WMI query returned no resultsfor Win32_OperatingSystem with values LocalDateTime and LastBootUpTime.")
		log.Debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	local, ok := parseWindowsWMITime(values["LocalDateTime"])
	if !ok {
		log.Debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	boot, ok := parseWindowsWMITime(values["LastBootUpTime"])
	if !ok {
		log.Debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	uptime := local.Sub(boot)
	if uptime <= 0 {
		log.Debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	return uptimeInfo{Duration: uptime, Known: true}
}

func parseWindowsWMITime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if len(value) < len("20060102150405") {
		return time.Time{}, false
	}
	date := value[:len("20060102150405")]
	offset := value[len("20060102150405"):]
	if strings.HasPrefix(offset, ".") {
		plus := strings.IndexAny(offset, "+-")
		if plus == -1 {
			return time.Time{}, false
		}
		offset = offset[plus:]
	}
	offset, ok := windowsWMIOffset(offset)
	if !ok {
		return time.Time{}, false
	}
	parsed, err := time.Parse("20060102150405-0700", date+offset)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func windowsWMIOffset(offset string) (string, bool) {
	if len(offset) >= len("-0700") {
		return offset[:len("-0700")], true
	}
	if len(offset) != len("-420") {
		return "", false
	}
	minutes, err := strconv.Atoi(offset[1:])
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%s%02d%02d", offset[:1], minutes/60, minutes%60), true
}

func uptimeFromProc(readFile fileReader) time.Duration {
	data, err := readFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func uptimeFromKernelBoottime(input string, now func() time.Time) time.Duration {
	start := strings.Index(input, "sec = ")
	if start == -1 {
		return 0
	}
	start += len("sec = ")
	end := strings.IndexByte(input[start:], ',')
	if end == -1 {
		return 0
	}
	boot, err := strconv.ParseInt(input[start:start+end], 10, 64)
	if err != nil {
		return 0
	}
	return now().Sub(time.Unix(boot, 0))
}

func parseUptimeCommandSeconds(input string) int {
	_, duration, ok := strings.Cut(input, " up ")
	if !ok {
		return 0
	}
	fields := strings.Fields(strings.NewReplacer(",", " ").Replace(duration))
	seconds := 0
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "user", "users", "load", "loadavg":
			return seconds
		}
		if hours, minutes, ok := parseUptimeHoursMinutes(field); ok {
			seconds += hours*3600 + minutes*60
			continue
		}
		if i+1 >= len(fields) {
			continue
		}
		value, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		switch fields[i+1] {
		case "day", "days":
			seconds += value * 24 * 3600
			i++
		case "hr", "hrs", "hr(s)", "hour", "hours", "hour(s)":
			seconds += value * 3600
			i++
		case "min", "mins", "min(s)", "minute", "minutes", "minute(s)":
			seconds += value * 60
			i++
		case "user", "users":
			return seconds
		}
	}
	return seconds
}

func parseDockerElapsedTimeSeconds(input string) int {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0
	}

	var days int
	if before, after, ok := strings.Cut(input, "-"); ok {
		value, err := strconv.Atoi(before)
		if err != nil {
			return 0
		}
		days = value
		input = after
	}

	parts := strings.Split(input, ":")
	seconds := days * 24 * 3600
	switch len(parts) {
	case 1:
		value, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		return seconds + value
	case 2:
		minutes, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		value, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0
		}
		return seconds + minutes*60 + value
	case 3:
		hours, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		minutes, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0
		}
		value, err := strconv.Atoi(parts[2])
		if err != nil {
			return 0
		}
		return seconds + hours*3600 + minutes*60 + value
	default:
		return 0
	}
}

func parseUptimeHoursMinutes(input string) (int, int, bool) {
	hoursText, minutesText, ok := strings.Cut(input, ":")
	if !ok {
		return 0, 0, false
	}
	hours, err := strconv.Atoi(hoursText)
	if err != nil {
		return 0, 0, false
	}
	minutes, err := strconv.Atoi(minutesText)
	if err != nil {
		return 0, 0, false
	}
	return hours, minutes, true
}

func probeLoadAverages(s *Session) map[string]any {
	return currentLoadAverages(runtime.GOOS, s.readFile, s.commandOutput)
}

func currentLoadAverages(goos string, readFile fileReader, run commandRunner) map[string]any {
	switch goos {
	case "darwin", "freebsd", "netbsd", "openbsd":
		out := run("sysctl", "-n", "vm.loadavg")
		if out == "" {
			return emptyLoadAverages()
		}
		return parseLoadAverages(out)
	case "linux":
		data, err := readFile("/proc/loadavg")
		if err != nil {
			return emptyLoadAverages()
		}
		return parseLoadAverages(string(data))
	default:
		return emptyLoadAverages()
	}
}

func parseLoadAverages(input string) map[string]any {
	fields := strings.Fields(strings.Trim(input, "{} \t\r\n"))
	if len(fields) < 3 {
		return emptyLoadAverages()
	}

	averages := make(map[string]any, 3)
	for i, key := range []string{"1m", "5m", "15m"} {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return emptyLoadAverages()
		}
		averages[key] = value
	}
	return averages
}

func emptyLoadAverages() map[string]any {
	return map[string]any{"1m": nil, "5m": nil, "15m": nil}
}

// uptimeCoreFacts assembles the uptime category facts (the system_uptime fields
// and the load_averages fact) for the current host.
func uptimeCoreFacts(s *Session) []ResolvedFact {
	uptime := s.cachedUptime()
	loadAverages := s.cachedLoadAverages()
	return []ResolvedFact{
		{Name: "load_averages", Value: loadAverages},
		{Name: "system_uptime.days", Value: int(uptime.Duration.Hours()) / 24},
		{Name: "system_uptime.hours", Value: int(uptime.Duration.Hours())},
		{Name: "system_uptime.seconds", Value: int(uptime.Duration.Seconds())},
		{Name: "system_uptime.uptime", Value: uptimeString(uptime)},
	}
}
