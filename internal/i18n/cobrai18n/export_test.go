package cobrai18n

// UsageTemplateForTest exposes the unexported usageTemplate to
// package cobrai18n_test.
func UsageTemplateForTest() string { return usageTemplate() }

// SafeForTest exposes the unexported safe to package cobrai18n_test.
func SafeForTest(s string) string { return safe(s) }
