package core

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestAssessmentDiagnosticRequiresEvidence(t *testing.T) {
	tests := []struct {
		name, status, output string
		needsDistribution    bool
		blocked              bool
	}{
		{"adhoc generic needs structured check", "adhoc", "rejected", true, false},
		{"unsigned generic blocked", "unsigned", "rejected", false, true},
		{"verified signature permits quarantine-only processing", "valid", "rejected", false, false},
		{"verified signature permits surrounding whitespace", "valid", " \nrejected\n", false, false},
		{"unknown generic blocked", "unknown", "rejected", false, true},
		{"invalid generic blocked", "invalid", "rejected", false, true},
		{"empty signature blocked", "", "rejected", false, true},
		{"unknown cannot use unsigned exception", "unknown", "rejected\nsource=no usable signature", false, true},
		{"invalid cannot use developer exception", "invalid", "rejected\nsource=unnotarized developer id", false, true},
		{"unsigned distribution reason unchanged", "unsigned", "rejected\nsource=no usable signature", false, false},
		{"developer distribution reason unchanged", "valid", "rejected\nsource=unnotarized developer id", false, false},
		{"additional unknown diagnostic blocked", "adhoc", "rejected\nsource=unrecognized policy failure", false, true},
		{"verified signature with unknown diagnostic blocked", "valid", "rejected\nsource=unrecognized policy failure", false, true},
		{"service failure blocked", "adhoc", "internal error in code signing subsystem", false, true},
		{"empty output blocked", "adhoc", "", false, true},
	}
	for _, signal := range []string{"revoked", "malware", "malicious", "xprotect", "resource envelope is obsolete", "invalid signature", "a sealed resource is missing", "resource missing"} {
		for _, status := range []string{"adhoc", "valid"} {
			tests = append(tests, struct {
				name, status, output string
				needsDistribution    bool
				blocked              bool
			}{status + " blocks " + signal, status, "rejected\n" + signal, false, true})
		}
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment, needsDistribution, err := classifyAssessment(test.output, test.status, 3)
			if (err != nil) != test.blocked || needsDistribution != test.needsDistribution {
				t.Fatalf("assessment=%q, requires distribution=%v, error=%v", assessment, needsDistribution, err)
			}
			if needsDistribution && assessment != "" {
				t.Fatal("generic rejection prematurely produced a nonblocking assessment")
			}
		})
	}
	// Target names are not diagnostic text and cannot inject an allowed reason.
	path := "/Applications/rejected no usable signature.app"
	_, fallback, err := classifyAssessment(diagnosticText(path+": unrelated failure", path), "adhoc", 3)
	if err == nil || fallback {
		t.Fatal("target path was treated as an assessment verdict")
	}
	for _, status := range []string{"adhoc", "valid"} {
		for _, code := range []int{-1, 0, 1, 70} {
			_, fallback, err := classifyAssessment("rejected", status, code)
			if err == nil || fallback {
				t.Fatalf("%s with spctl exit %d was allowed", status, code)
			}
		}
	}
}

func TestAssessmentMessagesDistinguishFailureFromQuarantineOnly(t *testing.T) {
	assessment, fallback, err := classifyAssessment("internal error in code signing subsystem", "valid", 1)
	if assessment != assessmentUnavailable || fallback || !errors.Is(err, assessmentUnavailableError) {
		t.Fatalf("system assessment failure was not classified clearly: %q, fallback=%v, err=%v", assessment, fallback, err)
	}

	assessment, fallback, err = classifyAssessment("rejected", "valid", 3)
	if assessment != assessmentQuarantineOnly || fallback || err != nil {
		t.Fatalf("verified app cannot remove quarantine: %q, fallback=%v, err=%v", assessment, fallback, err)
	}
	for _, message := range []string{"系统未说明拒绝原因", "保留签名", "仅移除隔离标记", "尝试打开"} {
		if !strings.Contains(assessment, message) {
			t.Fatalf("quarantine-only assessment is missing %q: %q", message, assessment)
		}
	}

	assessment, fallback, err = classifyAssessment("rejected\nsource=unrecognized policy failure", "valid", 3)
	if assessment != assessmentRejected || fallback || !errors.Is(err, assessmentRejectedError) {
		t.Fatalf("unknown policy finding was ignored: %q, fallback=%v, err=%v", assessment, fallback, err)
	}
}

func TestAlreadyProcessedAssessmentRequiresVerifiedSignatureAndNoQuarantine(t *testing.T) {
	s := &snapshot{Entries: []entry{{Attrs: map[string][]byte{}}}}
	assessment, _, err := classifyAssessment("rejected", "valid", 3)
	if err != nil || !isAlreadyProcessed(s, "valid", assessment) {
		t.Fatal("valid app without quarantine was not recognized as already processed")
	}
	for _, test := range []struct {
		name, status string
		attrs        map[string][]byte
	}{
		{name: "adhoc signature", status: "adhoc"},
		{name: "invalid signature", status: "invalid"},
		{name: "unsigned app", status: "unsigned"},
		{name: "unknown signature", status: "unknown"},
		{name: "quarantined app", status: "valid", attrs: map[string][]byte{QuarantineAttribute: []byte("present")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := &snapshot{Entries: []entry{{Attrs: test.attrs}}}
			if isAlreadyProcessed(candidate, test.status, assessment) {
				t.Fatal("unsafe app entered already-processed path")
			}
		})
	}
	nested := &snapshot{Entries: []entry{
		{Attrs: map[string][]byte{}},
		{Path: "Contents/MacOS/app", Attrs: map[string][]byte{QuarantineAttribute: []byte("present")}},
	}}
	if isAlreadyProcessed(nested, "valid", assessment) {
		t.Fatal("nested quarantine incorrectly entered the no-op path")
	}
	if isAlreadyProcessed(nil, "valid", assessment) {
		t.Fatal("missing snapshot incorrectly entered the no-op path")
	}
	for _, msg := range []string{"rejected\nsource=unrecognized policy failure", "rejected\nmalware", "rejected\ncertificate revoked", "internal error in code signing subsystem"} {
		assessment, _, err := classifyAssessment(msg, "valid", 3)
		if err == nil || isAlreadyProcessed(s, "valid", assessment) {
			t.Fatalf("no quarantine bypassed an assessment failure: %q", msg)
		}
	}
}

func TestDistributionAssessmentAllowsOnlyCompleteKnownReport(t *testing.T) {
	path := "/Applications/Peck Fixture.app"
	issues := []distributionIssue{
		{ShortError: "Adhoc Signed App", Level: "Warning"},
		{ShortError: "Notary Ticket Missing", Level: "Fatal", File: path},
	}
	reportJSON := func(items []distributionIssue) string {
		t.Helper()
		out, err := json.Marshal(distributionReport{Output: items})
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	known := reportJSON(issues)
	// Recorded macOS output with the path replaced and advice shortened.
	recorded := `{"output":[{"SyspolicyCheckAdvice":"Sign the app with a valid Developer ID certificate for distribution.","SyspolicyCheckLongError":"This app is adhoc signed. While it may run locally, adhoc signed apps are not suitable for distribution.","SyspolicyCheckAdditionalInformation":"","SyspolicyCheckDocumentationLink":"https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution","SyspolicyCheckErrorLevel":"Warning","SyspolicyCheckShortError":"Adhoc Signed App"},{"SyspolicyCheckLongError":"A Notarization ticket is not stapled to this application.","SyspolicyCheckAdvice":"Attach a notarization ticket for distribution.","SyspolicyCheckShortError":"Notary Ticket Missing","SyspolicyCheckErrorFile":"/Applications/Peck Fixture.app","SyspolicyCheckAdditionalInformation":"","SyspolicyCheckErrorLevel":"Fatal","SyspolicyCheckDocumentationLink":"https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution."}]}`
	// Fixed, harmless subprocesses provide real ExitError values without
	// exercising the host's app assessment service or touching installed apps.
	exit70 := exec.Command("/bin/sh", "-c", "exit 70").Run()
	exit1 := exec.Command("/bin/sh", "-c", "exit 1").Run()
	if exit70 == nil || exit1 == nil {
		t.Fatal("fixture exit statuses were not reported")
	}
	tests := []struct {
		name, report string
		commandErr   error
		allowed      bool
	}{
		{"complete known distribution issues", known, exit70, true},
		{"recorded system output", recorded, exit70, true},
		{"known issues in reverse order", reportJSON([]distributionIssue{issues[1], issues[0]}), exit70, true},
		{"success exit with error report is inconsistent", known, nil, false},
		{"other exit status blocked", known, exit1, false},
		{"tool unavailable blocked", known, errors.New("no such file or directory"), false},
		{"empty output blocked", "", exit70, false},
		{"invalid json blocked", "{", exit70, false},
		{"trailing result blocked", known + "{}", exit70, false},
		{"missing output blocked", "{}", exit70, false},
		{"empty issue array blocked", `{"output":[]}`, exit70, false},
		{"missing ticket issue blocked", reportJSON(issues[:1]), exit70, false},
		{"duplicate issue blocked", reportJSON([]distributionIssue{issues[0], issues[0]}), exit70, false},
		{"additional problem blocked", reportJSON(append(append([]distributionIssue(nil), issues...), distributionIssue{ShortError: "Malware Detected", Level: "Fatal"})), exit70, false},
		{"unknown issue blocked", strings.Replace(known, "Notary Ticket Missing", "Unrecognized Policy Failure", 1), exit70, false},
		{"wrong warning level blocked", strings.Replace(known, "Warning", "Fatal", 1), exit70, false},
		{"wrong fatal level blocked", strings.Replace(known, "Fatal", "Warning", 1), exit70, false},
		{"unknown level blocked", strings.Replace(known, "Fatal", "Advisory", 1), exit70, false},
		{"different target blocked", strings.Replace(known, path, "/Applications/Other.app", 1), exit70, false},
		{"unknown top-level data blocked", strings.TrimSuffix(known, "}") + `,"additionalFailure":"malware"}`, exit70, false},
		{"unknown issue data blocked", strings.Replace(known, `"SyspolicyCheckErrorLevel":"Warning"`, `"SyspolicyCheckErrorLevel":"Warning","UnknownCheck":true`, 1), exit70, false},
		{"duplicate output blocked", strings.TrimSuffix(known, "}") + `,"output":` + strings.TrimSuffix(strings.TrimPrefix(known, `{"output":`), "}") + "}", exit70, false},
		{"duplicate issue field blocked", strings.Replace(known, `"SyspolicyCheckShortError":"Adhoc Signed App"`, `"SyspolicyCheckShortError":"Malware Detected","SyspolicyCheckShortError":"Adhoc Signed App"`, 1), exit70, false},
		{"case-variant duplicate blocked", strings.Replace(known, `"SyspolicyCheckShortError":"Adhoc Signed App"`, `"SyspolicyCheckShortError":"Malware Detected","syspolicycheckshorterror":"Adhoc Signed App"`, 1), exit70, false},
		{"oversized result blocked", known + strings.Repeat(" ", 1<<20), exit70, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment, err := classifyDistributionAssessment([]byte(test.report), path, test.commandErr)
			if (err == nil) != test.allowed {
				t.Fatalf("assessment=%q, error=%v", assessment, err)
			}
			if test.allowed && (!strings.Contains(assessment, "缺少公证票据") || !strings.Contains(assessment, "不会提供 Apple 公证或安全认证")) {
				t.Fatalf("distribution limitation missing: %q", assessment)
			}
		})
	}
}
