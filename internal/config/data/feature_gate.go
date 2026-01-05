package data

// FeatureGates controls optional features
type FeatureGates struct {
	// SSMConnect enables SSM Session Manager connections to EC2 instances
	SSMConnect bool `yaml:"ssmConnect"`

	// CloudWatchLogs enables CloudWatch log viewing
	CloudWatchLogs bool `yaml:"cloudWatchLogs"`

	// CostExplorer enables cost visibility features
	CostExplorer bool `yaml:"costExplorer"`
}

// NewFeatureGates creates FeatureGates with default settings (all disabled)
func NewFeatureGates() FeatureGates {
	return FeatureGates{
		SSMConnect:     false,
		CloudWatchLogs: false,
		CostExplorer:   false,
	}
}

// Merge overlays another FeatureGates on top of this one
// Only enabled features in other will be applied
func (f *FeatureGates) Merge(other FeatureGates) {
	if other.SSMConnect {
		f.SSMConnect = true
	}
	if other.CloudWatchLogs {
		f.CloudWatchLogs = true
	}
	if other.CostExplorer {
		f.CostExplorer = true
	}
}
