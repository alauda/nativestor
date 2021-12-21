package operator

type OperatorConfig struct {
	OperatorNamespace string
	Image             string
	ServiceAccount    string
	NamespaceToWatch  string
	Parameters        map[string]string
}

const (
	OperatorSettingConfigMapName string = "topolvm-operator-setting"
	EnableRawDeviceEnv           string = "RAW_DEVICE_ENABLE"
	DiscoverAppName              string = "discover-device"
)
