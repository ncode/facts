package engine

import (
	"strings"
	"testing"
)

func TestWindowsWMIOutput_prefersWmicWhenAvailable(t *testing.T) {
	var calls [][]string
	run := func(name string, args ...string) string {
		calls = append(calls, append([]string{name}, args...))
		if name == "wmic" {
			return "FreePhysicalMemory=1024\nTotalVisibleMemorySize=2048\n"
		}
		t.Fatalf("unexpected command %s %v", name, args)
		return ""
	}

	got := windowsWMIOutput(run, "os", "FreePhysicalMemory,TotalVisibleMemorySize")
	if !strings.Contains(got, "FreePhysicalMemory=1024") {
		t.Fatalf("output = %q, want wmic output", got)
	}
	if len(calls) != 1 || calls[0][0] != "wmic" {
		t.Fatalf("calls = %v, want single wmic call", calls)
	}
}

func TestWindowsWMIOutput_fallsBackToPowerShellCIMWhenWmicIsMissing(t *testing.T) {
	var powershellArgs []string
	run := func(name string, args ...string) string {
		switch name {
		case "wmic":
			return ""
		case "powershell":
			powershellArgs = args
			return "FreePhysicalMemory=1024\nTotalVisibleMemorySize=2048\n"
		default:
			t.Fatalf("unexpected command %s %v", name, args)
			return ""
		}
	}

	memory := parseWindowsMemory(windowsWMIOutput(run, "os", "FreePhysicalMemory,TotalVisibleMemorySize"), discardLog())
	if memory.TotalBytes == 0 {
		t.Fatalf("memory = %+v, want parsed CIM fallback values", memory)
	}
	if powershellArgs == nil {
		t.Fatal("powershell was not invoked after empty wmic output")
	}
	script := powershellArgs[len(powershellArgs)-1]
	for _, want := range []string{"Get-CimInstance -ClassName Win32_OperatingSystem", "'FreePhysicalMemory'", "'TotalVisibleMemorySize'", "ToDmtfDateTime"} {
		if !strings.Contains(script, want) {
			t.Fatalf("powershell script %q missing %q", script, want)
		}
	}
	wantFlags := []string{"-NoProfile", "-NonInteractive", "-Command"}
	for i, flag := range wantFlags {
		if powershellArgs[i] != flag {
			t.Fatalf("powershell args = %v, want flags %v", powershellArgs, wantFlags)
		}
	}
}

func TestWindowsWMIOutput_unknownAliasReturnsEmptyWithoutPowerShell(t *testing.T) {
	run := func(name string, args ...string) string {
		if name == "powershell" {
			t.Fatalf("powershell must not run for unknown alias")
		}
		return ""
	}
	if got := windowsWMIOutput(run, "nosuchalias", "Prop"); got != "" {
		t.Fatalf("output = %q, want empty", got)
	}
}

func TestWindowsWMIOutput_cimRecordsParseLikeWmic(t *testing.T) {
	run := func(name string, args ...string) string {
		if name == "wmic" {
			return ""
		}
		// Output shape produced by windowsCIMScript on a multi-CPU host.
		return strings.Join([]string{
			"Manufacturer=QEMU",
			"Model=Standard PC",
			`OEMStringArray={"vm","kvm"}`,
			"",
			"Manufacturer=QEMU",
			"Model=Standard PC",
			`OEMStringArray={"vm","kvm"}`,
			"",
		}, "\n")
	}

	records := parseWindowsWMIRecords(windowsWMIOutput(run, "computersystem", "Manufacturer,Model,OEMStringArray"))
	if len(records) != 2 {
		t.Fatalf("records = %v, want 2", records)
	}
	if records[0]["Manufacturer"] != "QEMU" || records[0]["OEMStringArray"] != `{"vm","kvm"}` {
		t.Fatalf("record = %v, want wmic-shaped values", records[0])
	}
}
