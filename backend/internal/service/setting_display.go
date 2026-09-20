package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultDisplayLocale          = "en"
	DefaultDisplayCurrency        = "CNY"
	DefaultDisplayCurrencySymbol  = "¥"
	maxDisplayCurrencySymbolRunes = 16
)

// BuiltInDisplayLocales is the shipped language-pack order. Admin can enable a
// subset; unknown codes are dropped.
var BuiltInDisplayLocales = []string{"en", "zh", "fr", "ru"}

var builtInDisplayLocaleSet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(BuiltInDisplayLocales))
	for _, code := range BuiltInDisplayLocales {
		out[code] = struct{}{}
	}
	return out
}()

var defaultCurrencySymbols = map[string]string{
	"USD": "$",
	"CNY": "¥",
	"RMB": "¥",
	"EUR": "€",
	"GBP": "£",
	"JPY": "¥",
	"HKD": "HK$",
	"TWD": "NT$",
	"KRW": "₩",
	"AUD": "A$",
	"CAD": "C$",
	"SGD": "S$",
	"NZD": "NZ$",
	"MOP": "MOP$",
	"MYR": "RM",
	"THB": "฿",
	"PHP": "₱",
	"INR": "₹",
}

func DefaultDisplayLocalesJSON() string {
	encoded, err := json.Marshal(BuiltInDisplayLocales)
	if err != nil {
		return `["en","zh","fr","ru"]`
	}
	return string(encoded)
}

func defaultDisplaySettingValues() map[string]string {
	return map[string]string{
		SettingKeyDisplayLocales:        DefaultDisplayLocalesJSON(),
		SettingKeyDefaultLocale:         DefaultDisplayLocale,
		SettingKeyDisplayCurrency:       DefaultDisplayCurrency,
		SettingKeyDisplayCurrencySymbol: DefaultDisplayCurrencySymbol,
	}
}

func IsBuiltInDisplayLocale(code string) bool {
	_, ok := builtInDisplayLocaleSet[strings.ToLower(strings.TrimSpace(code))]
	return ok
}

func NormalizeDisplayLocales(raw []string) []string {
	seen := make(map[string]struct{}, len(BuiltInDisplayLocales))
	for _, item := range raw {
		code := strings.ToLower(strings.TrimSpace(item))
		if _, ok := builtInDisplayLocaleSet[code]; !ok {
			continue
		}
		seen[code] = struct{}{}
	}
	out := make([]string, 0, len(BuiltInDisplayLocales))
	for _, code := range BuiltInDisplayLocales {
		if _, ok := seen[code]; ok {
			out = append(out, code)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), BuiltInDisplayLocales...)
	}
	return out
}

func ParseDisplayLocalesJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]string(nil), BuiltInDisplayLocales...)
	}
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return append([]string(nil), BuiltInDisplayLocales...)
	}
	return NormalizeDisplayLocales(parsed)
}

func NormalizeDefaultLocale(raw string, locales []string) string {
	locales = NormalizeDisplayLocales(locales)
	code := strings.ToLower(strings.TrimSpace(raw))
	for _, item := range locales {
		if item == code {
			return item
		}
	}
	return locales[0]
}

func NormalizeDisplayCurrency(raw string) string {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if len(code) != 3 {
		return DefaultDisplayCurrency
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return DefaultDisplayCurrency
		}
	}
	return code
}

func DefaultSymbolForCurrency(code string) string {
	code = NormalizeDisplayCurrency(code)
	if symbol, ok := defaultCurrencySymbols[code]; ok {
		return symbol
	}
	return code
}

func NormalizeDisplayCurrencySymbol(raw, currency string) string {
	symbol := strings.TrimSpace(raw)
	if symbol == "" {
		return DefaultSymbolForCurrency(currency)
	}
	if utf8.RuneCountInString(symbol) > maxDisplayCurrencySymbolRunes {
		runes := []rune(symbol)
		symbol = string(runes[:maxDisplayCurrencySymbolRunes])
	}
	return symbol
}

func parseStoredDisplaySettings(settings map[string]string) (locales []string, defaultLocale, currency, symbol string) {
	locales = ParseDisplayLocalesJSON(settings[SettingKeyDisplayLocales])
	defaultLocale = NormalizeDefaultLocale(settings[SettingKeyDefaultLocale], locales)
	currency = NormalizeDisplayCurrency(settings[SettingKeyDisplayCurrency])
	symbol = NormalizeDisplayCurrencySymbol(settings[SettingKeyDisplayCurrencySymbol], currency)
	return locales, defaultLocale, currency, symbol
}

func marshalDisplayLocales(locales []string) (string, error) {
	encoded, err := json.Marshal(NormalizeDisplayLocales(locales))
	if err != nil {
		return "", fmt.Errorf("marshal display_locales: %w", err)
	}
	return string(encoded), nil
}

// EnsureDisplaySettings inserts the shipped display locale/currency keys when
// they are absent. Existing values are never overwritten so upgraded installs
// keep operator edits, and first-run / skipped-migration installs still get
// the historical defaults.
func (s *SettingService) EnsureDisplaySettings(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	keys := []string{
		SettingKeyDisplayLocales,
		SettingKeyDefaultLocale,
		SettingKeyDisplayCurrency,
		SettingKeyDisplayCurrencySymbol,
	}
	existing, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return fmt.Errorf("get display settings: %w", err)
	}

	defaults := defaultDisplaySettingValues()
	missing := make(map[string]string, len(keys))
	for _, key := range keys {
		if _, ok := existing[key]; ok {
			continue
		}
		missing[key] = defaults[key]
	}
	if len(missing) == 0 {
		return nil
	}
	if err := s.settingRepo.SetMultiple(ctx, missing); err != nil {
		return fmt.Errorf("inject display settings: %w", err)
	}
	return nil
}
