//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDisplayLocales(t *testing.T) {
	require.Equal(t, BuiltInDisplayLocales, NormalizeDisplayLocales(nil))
	require.Equal(t, BuiltInDisplayLocales, NormalizeDisplayLocales([]string{"de", "ja"}))
	require.Equal(t, []string{"en", "zh"}, NormalizeDisplayLocales([]string{"ZH", " en ", "de", "zh"}))
	require.Equal(t, []string{"fr"}, NormalizeDisplayLocales([]string{"fr"}))
}

func TestNormalizeDefaultLocale(t *testing.T) {
	require.Equal(t, "zh", NormalizeDefaultLocale("ZH", []string{"en", "zh"}))
	require.Equal(t, "en", NormalizeDefaultLocale("fr", []string{"en", "zh"}))
	require.Equal(t, "fr", NormalizeDefaultLocale("", []string{"fr"}))
}

func TestNormalizeDisplayCurrencyAndSymbol(t *testing.T) {
	require.Equal(t, "USD", NormalizeDisplayCurrency(" usd "))
	require.Equal(t, DefaultDisplayCurrency, NormalizeDisplayCurrency("US"))
	require.Equal(t, DefaultDisplayCurrency, NormalizeDisplayCurrency("us1"))
	require.Equal(t, "$", NormalizeDisplayCurrencySymbol("", "USD"))
	require.Equal(t, "US$", NormalizeDisplayCurrencySymbol(" US$ ", "USD"))
	require.Equal(t, "¥", NormalizeDisplayCurrencySymbol("", "CNY"))
	require.Equal(t, "XYZ", NormalizeDisplayCurrencySymbol("", "XYZ"))
	long := stringsRepeat("¤", 20)
	require.Equal(t, stringsRepeat("¤", 16), NormalizeDisplayCurrencySymbol(long, "USD"))
}

func stringsRepeat(s string, n int) string {
	out := make([]rune, 0, n)
	r := []rune(s)[0]
	for i := 0; i < n; i++ {
		out = append(out, r)
	}
	return string(out)
}

func TestParseDisplayLocalesJSONFallsBack(t *testing.T) {
	require.Equal(t, BuiltInDisplayLocales, ParseDisplayLocalesJSON(""))
	require.Equal(t, BuiltInDisplayLocales, ParseDisplayLocalesJSON("not-json"))
	require.Equal(t, []string{"en", "ru"}, ParseDisplayLocalesJSON(`["ru","en","de"]`))
}

func TestEnsureDisplaySettingsInsertsMissingKeysOnly(t *testing.T) {
	repo := &forwardedIPMigrationRepoStub{values: map[string]string{
		SettingKeyDisplayCurrency: "USD",
	}}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.EnsureDisplaySettings(context.Background()))
	require.Equal(t, "USD", repo.values[SettingKeyDisplayCurrency])
	require.JSONEq(t, DefaultDisplayLocalesJSON(), repo.values[SettingKeyDisplayLocales])
	require.Equal(t, DefaultDisplayLocale, repo.values[SettingKeyDefaultLocale])
	require.Equal(t, DefaultDisplayCurrencySymbol, repo.values[SettingKeyDisplayCurrencySymbol])
	require.NotContains(t, repo.updates, SettingKeyDisplayCurrency)

	require.NoError(t, svc.EnsureDisplaySettings(context.Background()))
	require.Equal(t, "USD", repo.values[SettingKeyDisplayCurrency])
}

func TestParseSettingsUsesDisplayDefaultsWhenMissing(t *testing.T) {
	svc := NewSettingService(&forwardedIPMigrationRepoStub{values: map[string]string{}}, &config.Config{})
	parsed := svc.parseSettings(map[string]string{})
	require.Equal(t, BuiltInDisplayLocales, parsed.DisplayLocales)
	require.Equal(t, DefaultDisplayLocale, parsed.DefaultLocale)
	require.Equal(t, DefaultDisplayCurrency, parsed.DisplayCurrency)
	require.Equal(t, DefaultDisplayCurrencySymbol, parsed.DisplayCurrencySymbol)
}

func TestParseSettingsNormalizesStoredDisplayValues(t *testing.T) {
	svc := NewSettingService(&forwardedIPMigrationRepoStub{values: map[string]string{}}, &config.Config{})
	parsed := svc.parseSettings(map[string]string{
		SettingKeyDisplayLocales:        `["fr","de","en"]`,
		SettingKeyDefaultLocale:         "zh",
		SettingKeyDisplayCurrency:       "eur",
		SettingKeyDisplayCurrencySymbol: "",
	})
	require.Equal(t, []string{"en", "fr"}, parsed.DisplayLocales)
	require.Equal(t, "en", parsed.DefaultLocale)
	require.Equal(t, "EUR", parsed.DisplayCurrency)
	require.Equal(t, "€", parsed.DisplayCurrencySymbol)
}

func TestGetPublicSettingsExposesDisplayDefaults(t *testing.T) {
	svc := NewSettingService(&forwardedIPMigrationRepoStub{values: map[string]string{}}, &config.Config{})
	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, BuiltInDisplayLocales, settings.DisplayLocales)
	require.Equal(t, DefaultDisplayLocale, settings.DefaultLocale)
	require.Equal(t, DefaultDisplayCurrency, settings.DisplayCurrency)
	require.Equal(t, DefaultDisplayCurrencySymbol, settings.DisplayCurrencySymbol)

	raw, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	payload, ok := raw.(*PublicSettingsInjectionPayload)
	require.True(t, ok)
	require.Equal(t, BuiltInDisplayLocales, payload.DisplayLocales)
	require.Equal(t, DefaultDisplayCurrencySymbol, payload.DisplayCurrencySymbol)
}

func TestUpdateSettingsPersistsNormalizedDisplayFields(t *testing.T) {
	repo := &forwardedIPMigrationRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DisplayLocales:        []string{"zh", "fr", "de"},
		DefaultLocale:         "en",
		DisplayCurrency:       "usd",
		DisplayCurrencySymbol: " US$ ",
	})
	require.NoError(t, err)
	require.JSONEq(t, `["zh","fr"]`, repo.updates[SettingKeyDisplayLocales])
	require.Equal(t, "zh", repo.updates[SettingKeyDefaultLocale])
	require.Equal(t, "USD", repo.updates[SettingKeyDisplayCurrency])
	require.Equal(t, "US$", repo.updates[SettingKeyDisplayCurrencySymbol])
}
