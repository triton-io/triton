package setting

// setting.go stores things which may change over time.

const (
	LastAppliedLabel = "kubectl.kubernetes.io/last-applied-configuration"
	ShouldSendToFile = "sendToFile"
	AppLabel         = "app.kubernetes.io/name"
	AppInstanceLabel = "app.kubernetes.io/instance"
	PodReadinessGate = "apps.triton.io/ready"
	ApplicationPort  = "app-port"
	AppIDLabel       = "app"
	GroupIDLabel     = "group"
	ManageLabel      = "managed-by"
	TritonKey        = "triton-io"
	HarborCred       = "proharborregcred"
)
