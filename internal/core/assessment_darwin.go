package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
)

type distributionReport struct {
	Output []distributionIssue `json:"output"`
}

type distributionIssue struct {
	ShortError            string `json:"SyspolicyCheckShortError"`
	Level                 string `json:"SyspolicyCheckErrorLevel"`
	LongError             string `json:"SyspolicyCheckLongError"`
	AdditionalInformation string `json:"SyspolicyCheckAdditionalInformation"`
	Advice                string `json:"SyspolicyCheckAdvice"`
	DocumentationLink     string `json:"SyspolicyCheckDocumentationLink"`
	File                  string `json:"SyspolicyCheckErrorFile"`
}

// assessDistribution is only reached for a strictly verified ad-hoc signature
// when spctl reports a bare rejection. Do not infer its reason from that
// rejection: require the complete system diagnostic to identify only the two
// known distribution requirements.
func assessDistribution(ctx context.Context, path string) (string, error) {
	out, err := systemCommand(ctx, "/usr/bin/syspolicy_check", "distribution", "--json", path)
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return assessmentUnavailable, assessmentUnavailableError
	}
	return classifyDistributionAssessment(out, path, err)
}

func classifyDistributionAssessment(out []byte, path string, commandErr error) (string, error) {
	blocked := func(_ string) (string, error) {
		return assessmentUnavailable, assessmentUnavailableError
	}
	var exit *exec.ExitError
	if !errors.As(commandErr, &exit) || exit.ExitCode() != 70 {
		return blocked("未取得一致的系统分发检查结果，请稍后重新检查")
	}
	if len(out) >= 1<<20 {
		return blocked("系统分发检查结果不完整，停止处理")
	}
	if err := uniqueJSONKeys(json.NewDecoder(bytes.NewReader(out))); err != nil {
		return blocked("系统分发检查结果结构不明确，停止处理")
	}
	var report distributionReport
	decoder := json.NewDecoder(bytes.NewReader(out))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return blocked("无法读取系统分发检查结果，停止处理")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return blocked("系统分发检查包含额外结果，停止处理")
	}
	if len(report.Output) != 2 {
		return blocked("系统分发检查存在其他或不完整的问题，请联系开发者")
	}
	seenAdhoc, seenTicket := false, false
	for _, issue := range report.Output {
		if issue.File != "" && issue.File != path {
			return blocked("系统分发检查的应用路径不一致，停止处理")
		}
		switch issue.ShortError {
		case "Adhoc Signed App":
			if seenAdhoc || issue.Level != "Warning" {
				return blocked("系统分发检查的签名诊断无法确认，停止处理")
			}
			seenAdhoc = true
		case "Notary Ticket Missing":
			if seenTicket || issue.Level != "Fatal" {
				return blocked("系统分发检查的公证诊断无法确认，停止处理")
			}
			seenTicket = true
		default:
			return blocked("系统分发检查发现其他问题，停止处理；请联系开发者")
		}
	}
	if !seenAdhoc || !seenTicket {
		return blocked("系统分发检查未确认本地签名与公证状态，停止处理")
	}
	return "未通过系统分发评估；本地 ad-hoc 签名有效，缺少公证票据；处理不会提供 Apple 公证或安全认证", nil
}

// encoding/json normally accepts repeated object keys, including case variants
// that map to the same struct field. Reject those ambiguous reports before the
// schema decoder can silently replace an earlier diagnostic.
func uniqueJSONKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[strings.ToLower(name)] {
				return errors.New("duplicate or invalid JSON key")
			}
			seen[strings.ToLower(name)] = true
			if err := uniqueJSONKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
	case json.Delim('['):
		for decoder.More() {
			if err := uniqueJSONKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
	}
	return err
}
