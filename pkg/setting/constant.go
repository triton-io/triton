package setting

// constant.go stores things which never change.
const (
	PodPending         = "Pending"
	PodRunning         = "Running"
	PodReady           = "Ready"
	PodFailed          = "Failed"
	ContainersReady    = "ContainersReady"
	PodPullInFailed    = "PullInFailed"
	PodPullInSucceeded = "PullInSucceeded"

	TypePod      = "Pod"
	TypeCloneSet = "CloneSet"
	TypeDeploy   = "DeployFlow"

	// type of a deploy.
	Create     = "create"
	Update     = "update"
	Rollback   = "rollback"
	Delete     = "delete"
	Scale      = "scale"
	ScaleIn    = "scale_in"
	ScaleOut   = "scale_out"
	Restart    = "restart"
	RestartPod = "restart_pod"
)

type ResponseCode int

const (
	Success ResponseCode = iota     // 0
	Error   ResponseCode = iota + 6 // 7
)
