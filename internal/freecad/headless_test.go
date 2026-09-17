package freecad

import (
	"context"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestParseCommandSplitsShellLikeCommands(t *testing.T) {
	cases := []struct {
		value string
		want  []string
	}{
		{"flatpak run --command=freecadcmd org.freecad.FreeCAD", []string{"flatpak", "run", "--command=freecadcmd", "org.freecad.FreeCAD"}},
		{"freecadcmd", []string{"freecadcmd"}},
		{`'/opt/Free CAD/FreeCADCmd'`, []string{"/opt/Free CAD/FreeCADCmd"}},
		{`"/opt/Free CAD/FreeCADCmd"`, []string{"/opt/Free CAD/FreeCADCmd"}},
		{`freecadcmd -Dfoo="bar baz"`, []string{"freecadcmd", `-Dfoo=bar baz`}},
		{`freecadcmd a\ b`, []string{"freecadcmd", "a b"}},
		{"  freecadcmd   --arg  ", []string{"freecadcmd", "--arg"}},
	}
	for _, testCase := range cases {
		got, err := ParseCommand(testCase.value)
		if err != nil {
			t.Errorf("ParseCommand(%q): %v", testCase.value, err)
			continue
		}
		if !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("ParseCommand(%q) = %#v, want %#v", testCase.value, got, testCase.want)
		}
	}
}

func TestParseCommandRejectsBlankAndMalformedInput(t *testing.T) {
	for _, value := range []string{"", "   ", "\t\n"} {
		got, err := ParseCommand(value)
		if err != nil || got != nil {
			t.Errorf("ParseCommand(%q) = %#v, %v; want a nil command and no error", value, got, err)
		}
	}
	for _, value := range []string{"freecadcmd 'unclosed", `freecadcmd "unclosed`, `freecadcmd trailing\`} {
		if _, err := ParseCommand(value); err == nil {
			t.Errorf("ParseCommand(%q): expected an error", value)
		}
	}
}

func TestCleanOutputDropsBlankLinesAndBannerNoise(t *testing.T) {
	output := strings.Join([]string{
		"",
		"FreeCAD 1.0.0, Libs: 1.0.0R",
		"Importing project files......",
		"Postprocessing...",
		"  0%)",
		"(C) 2001-2024 FreeCAD contributors",
		"This library is free software; you can redistribute it under the LGPL",
		"Widget geometry is 30 mm",
		"   ",
	}, "\n")

	if got, want := cleanOutput(output), "Widget geometry is 30 mm"; got != want {
		t.Errorf("cleanOutput = %q, want %q", got, want)
	}
}

func TestPythonStringEscapesPaths(t *testing.T) {
	if got, want := pythonString(`/home/o'brien/script.py`), `'/home/o\'brien/script.py'`; got != want {
		t.Errorf("pythonString = %s, want %s", got, want)
	}
	if got, want := pythonString(`C:\temp\script.py`), `'C:\\temp\\script.py'`; got != want {
		t.Errorf("pythonString = %s, want %s", got, want)
	}
}

func TestFormatGMatchesPythonFormatting(t *testing.T) {
	cases := map[float64]string{0.5: "0.5", 600: "600", 1.25: "1.25", 100000: "100000"}
	for value, want := range cases {
		if got := formatG(value); got != want {
			t.Errorf("formatG(%v) = %q, want %q", value, got, want)
		}
	}
}

func TestRunHeadlessValidatesTimeoutAndExecutable(t *testing.T) {
	for _, timeout := range []float64{0, -1} {
		result := RunHeadless(context.Background(), "pass", timeout, []string{"freecadcmd"})
		if result["error"] != "timeout must be a positive finite number" {
			t.Errorf("timeout %v: result = %#v", timeout, result)
		}
	}

	// An empty PATH makes both the PATH lookup and the flatpak probe fail.
	t.Setenv("PATH", t.TempDir())
	result := RunHeadless(context.Background(), "pass", 5, nil)
	if result["error"] != "freecadcmd not found: install FreeCAD, or pass --freecadcmd to freecad-mcp" {
		t.Errorf("result = %#v", result)
	}

	result = RunHeadless(context.Background(), "pass", 5, []string{"/nonexistent/freecadcmd"})
	if error, _ := result["error"].(string); !strings.HasPrefix(error, "could not start headless FreeCAD:") {
		t.Errorf("result = %#v", result)
	}
	if _, ok := result["returncode"]; ok {
		t.Errorf("a start failure should not report a return code: %#v", result)
	}
}

// pythonCommand returns the interpreter used to stand in for freecadcmd.
func pythonCommand(t *testing.T) []string {
	t.Helper()
	path, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	return []string{path}
}

func TestRunHeadlessExecutesScript(t *testing.T) {
	command := pythonCommand(t)

	result := RunHeadless(context.Background(), "print('Widget geometry is 30 mm')", 30, command)
	if result["success"] != true {
		t.Fatalf("result = %#v", result)
	}
	if result["returncode"] != 0 {
		t.Errorf("returncode = %v, want 0", result["returncode"])
	}
	if result["output"] != "Widget geometry is 30 mm" {
		t.Errorf("output = %q", result["output"])
	}
	if _, ok := result["error"]; ok {
		t.Errorf("unexpected error: %#v", result)
	}
}

func TestRunHeadlessReportsScriptFailures(t *testing.T) {
	command := pythonCommand(t)

	result := RunHeadless(context.Background(), "import sys\nsys.exit(3)", 30, command)
	if result["success"] != false {
		t.Errorf("success = %v, want false", result["success"])
	}
	if result["returncode"] != 3 {
		t.Errorf("returncode = %v, want 3", result["returncode"])
	}
	if result["error"] != "script failed (exit code 3)" {
		t.Errorf("error = %q", result["error"])
	}

	// A native crash that FreeCAD's own handler reports before exiting 1.
	result = RunHeadless(context.Background(), "print('Program received signal SIGSEGV, Segmentation fault.')\nraise SystemExit(1)", 30, command)
	if result["crashed"] != true {
		t.Errorf("crashed = %v, want true: %#v", result["crashed"], result)
	}
	if result["error"] != "headless FreeCAD reported a crash with SIGSEGV (exit code 1; the GUI is unaffected)" {
		t.Errorf("error = %q", result["error"])
	}
}

func TestRunHeadlessReportsKilledProcessAsCrash(t *testing.T) {
	command := pythonCommand(t)

	result := RunHeadless(context.Background(), "import os, signal\nos.kill(os.getpid(), signal.SIGSEGV)", 30, command)
	if result["crashed"] != true {
		t.Fatalf("crashed = %v, want true: %#v", result["crashed"], result)
	}
	if result["returncode"] != -11 {
		t.Errorf("returncode = %v, want -11", result["returncode"])
	}
	if result["error"] != "headless FreeCAD crashed with SIGSEGV (OCCT native crash; the GUI is unaffected)" {
		t.Errorf("error = %q", result["error"])
	}
}

func TestRunHeadlessTimesOut(t *testing.T) {
	command := pythonCommand(t)

	result := RunHeadless(context.Background(), "import time\ntime.sleep(30)", 0.5, command)
	if result["timed_out"] != true {
		t.Fatalf("timed_out = %v, want true: %#v", result["timed_out"], result)
	}
	if result["success"] != false {
		t.Errorf("success = %v, want false", result["success"])
	}
	if result["error"] != "headless FreeCAD did not finish within 0.5 s" {
		t.Errorf("error = %q", result["error"])
	}
}
