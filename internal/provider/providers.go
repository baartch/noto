package provider

// ProviderInfo describes a known provider with its display name and API endpoint.
type ProviderInfo struct {
	Name     string
	Endpoint string
}

// AvailableProviders is the curated list of providers shown in the setup dialog.
var AvailableProviders = []ProviderInfo{
	{Name: "OpenRouter", Endpoint: "https://openrouter.ai/api/v1"},
	{Name: "OpenAI", Endpoint: "https://api.openai.com/v1"},
}
