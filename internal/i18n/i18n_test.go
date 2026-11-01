package i18n

import (
	"reflect"
	"strings"
	"testing"
)

func TestFromPreferredLanguagesUsesFirstSupportedPrimaryTag(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want Language
	}{
		{"simplified chinese", []string{"zh-Hans-CN", "en-US"}, Chinese},
		{"traditional chinese shares catalog", []string{"zh-TW"}, Chinese},
		{"english", []string{"fr-FR", "en-GB"}, English},
		{"unknown falls back", []string{"fr-FR"}, English},
		{"empty falls back", nil, English},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FromPreferredLanguages(test.tags); got != test.want {
				t.Fatalf("FromPreferredLanguages(%v) = %v, want %v", test.tags, got, test.want)
			}
		})
	}
}

func TestCatalogsHaveMatchingNonEmptyStrings(t *testing.T) {
	for _, language := range []Language{English, Chinese} {
		messages := For(language)
		value := reflect.ValueOf(messages)
		for index := 0; index < value.NumField(); index++ {
			field := value.Field(index)
			if field.Kind() == reflect.String && strings.TrimSpace(field.String()) == "" {
				t.Fatalf("%v has an empty catalog field %s", language, value.Type().Field(index).Name)
			}
		}
	}
}

func TestCoreTextUsesTheSelectedCatalog(t *testing.T) {
	const source = "系统评估服务暂时异常：macOS 未返回可确认的结果，请稍后重试。Peck 未修改应用。"
	if got := For(Chinese).CoreText(source); got != source {
		t.Fatalf("Chinese core text changed: %q", got)
	}
	if got := For(English).CoreText(source); got == source || !strings.Contains(got, "temporarily unavailable") {
		t.Fatalf("English core text was not translated: %q", got)
	}
}
