package main

type Config struct {
	Cron string `yaml:"cron" env:"SBOM_CRON" flag:"cron"`

	IgnoreAnnotations      bool   `yaml:"ignoreAnnotations" env:"SBOM_IGNORE_ANNOTATIONS" flag:"ignoreAnnotations"`
	PodLabelSelector       string `yaml:"podLabelSelector" env:"SBOM_POD_LABEL_SELECTOR" flag:"podLabelSelector"`
	NamespaceLabelSelector string `yaml:"namespaceLabelSelector" env:"SBOM_NAMESPACE_LABEL_SELECTOR" flag:"namespaceLabelSelector"`

	JobTimeout         int64    `yaml:"jobTimeout" env:"SBOM_JOB_TIMEOUT" flag:"jobTimeout"`
	FallbackPullSecret string   `yaml:"fallbackPullSecret" env:"SBOM_FALLBACK_PULL_SECRET" flag:"fallbackPullSecret"`
	RegistryProxies    []string `yaml:"registryProxy" env:"SBOM_REGISTRY_PROXY" flag:"registryProxy"`
	Verbosity          string   `env:"SBOM_VERBOSITY" flag:"verbosity" yaml:"verbosity"`

	DevGuardToken      string
	DevGuardTokenFile  string `yaml:"devGuardTokenFile" env:"DEVGUARD_TOKEN_FILE"`
	DevGuardProjectURL string `yaml:"devGuardProjectURL" env:"DEVGUARD_PROJECT_URL" flag:"projectUrl"`
	DevGuardProviderID string `yaml:"devGuardProviderId" env:"DEVGUARD_PROVIDER_ID" flag:"providerId"`
}

var (
	ConfigKeyCron = "cron"

	ConfigKeyIgnoreAnnotations      = "ignoreAnnotations"
	ConfigKeyPodLabelSelector       = "podLabelSelector"
	ConfigKeyNamespaceLabelSelector = "namespaceLabelSelector"

	ConfigKeyJobTimeout         = "jobTimeout"
	ConfigKeyFallbackPullSecret = "fallbackPullSecret"
	ConfigKeyRegistryProxy      = "registryProxy"

	ConfigDevGuardProjectURL = "projectUrl"
	ConfigDevGuardProviderID = "providerId"

	OperatorConfig *Config
)
