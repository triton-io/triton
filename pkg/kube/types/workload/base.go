package workload

import (
	"fmt"
	"strconv"

	"github.com/triton-io/triton/pkg/setting"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Interface interface {
	GetContainers() []corev1.Container
	GetAppID() int
	GetGroupID() int
}

// Base is the base struct for all workload resource wrappers to provide some basic methods.
type Base struct {
	Interface
}

func (b *Base) GetAppContainerName() string {
	return GetAppContainerName(b.GetAppID(), b.GetGroupID())
}

func (b *Base) GetAppImage() string {
	c := b.GetAppContainer()
	if c == nil {
		return ""
	}
	return c.Image
}

func (b *Base) GetAppRequestCPU() string {
	if c := b.GetAppContainer(); c != nil {
		return c.Resources.Requests.Cpu().String()
	}
	return ""
}

func (b *Base) GetAppCPU() string {
	if c := b.GetAppContainer(); c != nil {
		return c.Resources.Limits.Cpu().String()
	}
	return ""
}

func (b *Base) GetAppRequestMemory() string {
	if c := b.GetAppContainer(); c != nil {
		return c.Resources.Requests.Memory().String()
	}
	return ""
}

func (b *Base) GetAppMemory() string {
	if c := b.GetAppContainer(); c != nil {
		return c.Resources.Limits.Memory().String()
	}
	return ""
}

func (b *Base) GetAppPort() int32 {
	c := b.GetAppContainer()
	if c == nil {
		return 0
	}

	ports := c.Ports
	if len(ports) == 0 {
		return 0
	}

	for _, port := range ports {
		if port.Name == "" || port.Name == setting.ApplicationPort {
			return port.ContainerPort
		}
	}

	return 0
}

func (b *Base) GetAppContainer() *corev1.Container {
	cs := b.GetContainers()
	// a workaround for old apps, remove it when no apps with old container name style exists.
	if len(cs) == 1 {
		return &(cs[0])
	}

	containerName := b.GetAppContainerName()
	for _, c := range cs {
		if c.Name == containerName {
			return &c
		}
	}
	return nil
}

func GetAppContainerName(appID, groupID int) string {
	return fmt.Sprintf("%d-container-%d", appID, groupID)
}

func GetDefaultLabels(appName, instanceName string, appID, groupID int) labels.Set {
	return labels.Set{
		setting.AppLabel:         appName,
		setting.AppInstanceLabel: instanceName,
		setting.AppIDLabel:       strconv.Itoa(appID),
		setting.GroupIDLabel:     strconv.Itoa(groupID),
		setting.ManageLabel:      setting.TritonKey,
	}
}

func GetDefaultSelector(appID, groupID int) labels.Set {
	return labels.Set{
		setting.AppIDLabel:   strconv.Itoa(appID),
		setting.GroupIDLabel: strconv.Itoa(groupID),
		// setting.AppLabel:     appName,
	}
}

func (b *Base) GetManagedSelector() labels.Selector {
	return ManagedSelector()
}

func ManagedSelector() labels.Selector {
	return labels.Set{setting.ManageLabel: setting.TritonKey}.AsSelector()
}
