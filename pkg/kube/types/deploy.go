package deploy

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	tritonappsv1alpha1 "github.com/triton-io/triton/api/v1alpha1"
	podservice "github.com/triton-io/triton/pkg/services/pod"
	"github.com/triton-io/triton/pkg/setting"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

var weightedBatchPhase = map[tritonappsv1alpha1.BatchPhase]uint32{
	"":                                  0,
	tritonappsv1alpha1.BatchSmoking:     1,
	tritonappsv1alpha1.BatchSmoked:      2,
	tritonappsv1alpha1.BatchBaking:      3,
	tritonappsv1alpha1.BatchBaked:       4,
	tritonappsv1alpha1.BatchSmokeFailed: 10,
	tritonappsv1alpha1.BatchBakeFailed:  10,
}

var weightedDeployPhase = map[tritonappsv1alpha1.DeployPhase]uint32{
	"":                               0,
	tritonappsv1alpha1.Pending:       1,
	tritonappsv1alpha1.Initializing:  2,
	tritonappsv1alpha1.BatchStarted:  3,
	tritonappsv1alpha1.BatchFinished: 4,
	tritonappsv1alpha1.Success:       10,
	tritonappsv1alpha1.Failed:        10,
	tritonappsv1alpha1.Aborted:       10,
	tritonappsv1alpha1.Canceled:      10,
}

const (
	Separator = '/'
)

// Deploy is the wrapper for tritonappsv1alpha1.DeployFlow type.
type Deploy struct {
	*tritonappsv1alpha1.DeployFlow
}

func FromDeploy(d *tritonappsv1alpha1.DeployFlow) *Deploy {
	if d == nil {
		return nil
	}

	return &Deploy{
		DeployFlow: d,
	}
}

// Unwrap returns the tritonappsv1alpha1.DeployFlow object.
func (d *Deploy) Unwrap() *tritonappsv1alpha1.DeployFlow {
	return d.DeployFlow
}

// String returns the general purpose string representation
func (d Deploy) String() string {
	return fmt.Sprintf("%s%c%s", d.Namespace, Separator, d.Name)
}

func (d *Deploy) CanaryEnabled() bool {
	return d.Canary() != 0 && d.RevisionChanged()
}

func (d *Deploy) Paused() bool {
	return d.DeployFlow.Status.Paused
}

func (d *Deploy) Finished() bool {
	return d.DeployFlow.Status.Finished
}

func (d *Deploy) NotStarted() bool {
	return weightedDeployPhase[d.Status.Phase] < weightedDeployPhase[tritonappsv1alpha1.BatchStarted]
}

func (d *Deploy) Pending() bool {
	return d.DeployFlow.Status.Phase == tritonappsv1alpha1.Pending
}

func (d *Deploy) Canceled() bool {
	return d.DeployFlow.Status.Phase == tritonappsv1alpha1.Canceled
}

func (d *Deploy) Aborted() bool {
	return d.DeployFlow.Status.Phase == tritonappsv1alpha1.Aborted
}

func (d *Deploy) Failed() bool {
	return d.DeployFlow.Status.Phase == tritonappsv1alpha1.Failed
}

func (d *Deploy) Success() bool {
	return d.DeployFlow.Status.Phase == tritonappsv1alpha1.Success
}

// Interruptible indicates that the deploy can be paused or canceled.
func (d *Deploy) Interruptible() bool {
	return weightedDeployPhase[d.Status.Phase] < weightedDeployPhase[tritonappsv1alpha1.BatchFinished]
}

func (d *Deploy) RevisionChanged() bool {
	return RevisionChanged(d.Spec.Action)
}

func (d *Deploy) NoChangedDeploy() bool {
	return d.Status.ReplicasToProcess == 0
}

func (d *Deploy) GetNotContainerReadyPods() []string {
	var notContainerReadyPods []string
	f := &podservice.Filter{
		Namespace:    d.Namespace,
		InstanceName: d.Spec.Application.AppName,
	}
	pods, err := podservice.FetchPods(f, labels.Everything(), true)
	if err != nil {
		klog.Errorf("failed to get pods from cloneset %s", d.Name)
		return nil
	}
	for index := range pods {
		for _, c := range pods[index].Status.Conditions {
			if c.Type == setting.ContainersReady && c.Status != "True" {
				notContainerReadyPods = append(notContainerReadyPods, pods[index].Name)
			}
		}
	}
	return notContainerReadyPods
}

func (d *Deploy) Auto() bool {
	return d.Mode() == tritonappsv1alpha1.Auto
}

func RevisionChanged(action string) bool {
	return action == setting.Create ||
		action == setting.Update ||
		action == setting.Rollback
}

func (d *Deploy) SkipPullIn() bool {

	return !d.RevisionChanged() || d.UpdateStrategy().NoPullIn
}

func (d *Deploy) MoveForward() bool {
	if d.Paused() {
		return false
	}

	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	// in canary batch, move forward if designed phase > current phase
	// in other batch, move forward if batches >= current batch
	// if canary is enabled, default desired stage is "Smoked",
	// so if it is not specified in .spec.updateStrategy.stage, treat it as "Smoked"
	if d.CurrentBatchIsCanary() {
		stage := d.UpdateStrategy().Stage
		if stage == "" {
			stage = tritonappsv1alpha1.BatchSmoked
		}
		return weightedBatchPhase[stage] > weightedBatchPhase[c.Phase]
	}

	// for an "auto" deploy, if current batch is not canary, always move forward.
	// if current batch is 2 and current phase is batchPending and canaryEnabled do not move forward.
	if d.Auto() {
		if !(c.Batch == 2 && c.Phase == tritonappsv1alpha1.BatchPending && d.CanaryEnabled()) {
			return true
		}

	}

	return d.DesiredBatches() >= d.CurrentBatchNumber()
}

// StateSatisfied returns true if current state meets the desired state in updateStrategy/nonUpdateStrategy,
func (d *Deploy) StateSatisfied() bool {
	return StateSatisfied(d, d.UpdateStrategy().Stage, d.DesiredBatches())
}

func StateSatisfied(d *Deploy, desiredStage tritonappsv1alpha1.BatchPhase, desiredBatches int) bool {
	currentBatch := d.CurrentBatchInfo()
	if currentBatch == nil {
		return false
	}

	if desiredBatches >= d.Status.Batches && desiredStage == tritonappsv1alpha1.BatchBaked {
		return d.Finished()
	}

	delta := currentBatch.Batch - desiredBatches

	if delta > 0 {
		return true
	} else if delta < 0 {
		return false
	}

	if desiredStage == "" {
		if currentBatch.Canary {
			desiredStage = tritonappsv1alpha1.BatchSmoked
		} else {
			desiredStage = tritonappsv1alpha1.BatchBaked
		}
	}

	return weightedBatchPhase[currentBatch.Phase] >= weightedBatchPhase[desiredStage]
}

func (d *Deploy) UpdateStrategy() *tritonappsv1alpha1.DeployUpdateStrategy {
	if d.Spec.UpdateStrategy != nil {
		return d.Spec.UpdateStrategy
	}

	// if UpdateStrategy is not specified, return a default one with no canary and only one batch.
	return &tritonappsv1alpha1.DeployUpdateStrategy{
		// Stage: tritonappsv1alpha1.BatchBaked,
	}
}

func (d *Deploy) NonUpdateStrategy() *tritonappsv1alpha1.DeployNonUpdateStrategy {
	if d.Spec.NonUpdateStrategy != nil {
		return d.Spec.NonUpdateStrategy
	}

	// if NonUpdateStrategy is not specified, return a default one with no canary and only one batch.
	return &tritonappsv1alpha1.DeployNonUpdateStrategy{}
}

func (d *Deploy) Canary() int {
	return d.UpdateStrategy().Canary
}

func (d *Deploy) BatchSize() *intstr.IntOrString {
	if d.RevisionChanged() {
		return d.UpdateStrategy().BatchSize
	}
	return d.NonUpdateStrategy().BatchSize
}

func (d *Deploy) BatchSizeNum() int32 {
	batchSize, err := intstr.GetValueFromIntOrPercent(d.BatchSize(), int(*d.Spec.Application.Replicas), true)
	if err != nil {
		return 0
	}

	return int32(batchSize)
}

func (d *Deploy) DesiredPausedState() *bool {
	if d.RevisionChanged() {
		return d.UpdateStrategy().Paused
	}
	return d.NonUpdateStrategy().Paused
}

func (d *Deploy) ShouldCancel() bool {
	if d.RevisionChanged() {
		return d.UpdateStrategy().Canceled
	}
	return d.NonUpdateStrategy().Canceled
}

func (d *Deploy) DesiredBatches() int {
	if d.RevisionChanged() {
		return d.UpdateStrategy().Batches
	}
	return d.NonUpdateStrategy().Batches
}

func (d *Deploy) BatchIntervalSeconds() int32 {
	if d.RevisionChanged() {
		return d.UpdateStrategy().BatchIntervalSeconds
	}

	bi := d.NonUpdateStrategy().BatchIntervalSeconds
	// In a scale-in, if podsToDelete is set, when we delete the old pod, a new one will be created immediately.
	// If the pod creationTimestamp is the same with next batch's startedAt time (same seconds), the pod will be
	// added to that batch by mistake.
	// set scale-in default interval to 1 second to avoid it.
	if d.Spec.Action == setting.ScaleIn && bi == 0 {
		bi = 1
	}

	return bi
}

func (d *Deploy) Mode() tritonappsv1alpha1.DeployMode {
	if d.RevisionChanged() {
		return d.UpdateStrategy().Mode
	}
	return d.NonUpdateStrategy().Mode
}

func (d *Deploy) CurrentBatchIsCanary() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.Canary
}

// CurrentBatchReady returns true if all pods in this Batch is ready or failed
func (d *Deploy) CurrentBatchIsPodReady() bool {
	return d.currentBatchIsReady(setting.PodReady, setting.PodFailed)
}

// CurrentBatchReady returns true if all pods in this Batch is ready or failed
func (d *Deploy) CurrentBatchIsContainersReady() bool {
	return d.currentBatchIsReady(setting.ContainersReady, setting.PodFailed)
}

// currentBatchIsReady returns true if all pods in this Batch are under readyPhases.
func (d *Deploy) currentBatchIsReady(readyPhases ...string) bool {
	if d.Spec.Action == setting.ScaleIn {
		return true
	}

	b := d.CurrentBatchInfo()

	if len(b.Pods) != b.BatchSize {
		return false
	}

	rp := sets.NewString(readyPhases...)

	for _, p := range b.Pods {
		if !rp.Has(p.Phase) {
			return false
		}
	}

	return true
}

func (d *Deploy) CurrentBatchStarted() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.Phase != tritonappsv1alpha1.BatchPending
}

func (d *Deploy) CurrentBatchPending() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.Phase == tritonappsv1alpha1.BatchPending
}

func (d *Deploy) CurrentBatchSmoking() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.Phase == tritonappsv1alpha1.BatchSmoking
}

func (d *Deploy) CurrentBatchSmoked() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.Phase == tritonappsv1alpha1.BatchSmoked && c.PulledInAt.IsZero()
}

func (d *Deploy) CurrentBatchPulledIn() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.Phase == tritonappsv1alpha1.BatchSmoked && !c.PulledInAt.IsZero()
}

func (d *Deploy) CurrentBatchFinished() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.Phase == tritonappsv1alpha1.BatchBaked
}

func (d *Deploy) CurrentBatchFailed() bool {
	c := d.CurrentBatchInfo()
	if c == nil {
		return false
	}

	return c.FailedReplicas > 0
}

func (d *Deploy) AllBatchFinished() bool {
	s := d.Status
	return s.FinishedReplicas == int(d.Status.ReplicasToProcess)
}

func (d *Deploy) CurrentBatchNumber() int {
	c := d.CurrentBatchInfo()
	if c == nil {
		return 0
	}

	return c.Batch
}

func (d *Deploy) CurrentBatchSize() int {
	c := d.CurrentBatchInfo()
	if c == nil {
		return 0
	}

	return c.BatchSize
}

func (d *Deploy) CurrentBatchInfo() *tritonappsv1alpha1.BatchCondition {
	batches := d.Status.Conditions
	if len(batches) == 0 {
		return nil
	}

	b := batches[len(batches)-1]
	// if conditions is not in order
	//if b.Batch != currentBatch {
	//	return d.getBatchInfoByNumber(currentBatch)
	//}

	return &b
}

func (d *Deploy) CurrentBatchPhase() tritonappsv1alpha1.BatchPhase {
	c := d.CurrentBatchInfo()
	if c == nil {
		return ""
	}

	return c.Phase
}

func (d *Deploy) CurrentBatchPods() []tritonappsv1alpha1.PodInfo {
	c := d.CurrentBatchInfo()
	if c == nil {
		return nil
	}

	return c.Pods
}

func (d *Deploy) CurrentBatchPullInAt() metav1.Time {
	c := d.CurrentBatchInfo()
	if c == nil {
		return metav1.Time{}
	}

	return c.PulledInAt
}

// call NextBatchAndPhase only when current state is satisfied!
func (d *Deploy) NextBatchAndPhase() (int, tritonappsv1alpha1.BatchPhase) {
	batch := d.CurrentBatchInfo()

	if !d.CurrentBatchIsCanary() {
		// if mode is auto, move to the end
		if d.Auto() {
			return d.Status.Batches, tritonappsv1alpha1.BatchBaked
		}
		return batch.Batch, tritonappsv1alpha1.BatchBaked
	}

	switch batch.Phase {
	case tritonappsv1alpha1.BatchSmoked:
		return 1, tritonappsv1alpha1.BatchBaking
	case tritonappsv1alpha1.BatchBaking:
		return 1, tritonappsv1alpha1.BatchBaked
	default:
		return 1, tritonappsv1alpha1.BatchSmoked
	}
}

func (d *Deploy) GetUpdateRevision() string {
	ur := d.Status.UpdateRevision
	if ur == "" {
		return ""
	}
	return ur[strings.LastIndex(ur, "-")+1:]

}

func (d *Deploy) GetLastApplied() (*tritonappsv1alpha1.ApplicationSpec, error) {
	lastApplied := &tritonappsv1alpha1.ApplicationSpec{}
	a := d.GetAnnotations()
	for k, v := range a {
		if k == setting.LastAppliedLabel && v != "" {
			err := json.Unmarshal([]byte(v), lastApplied)
			return lastApplied, err
		}
	}
	return nil, nil
}

func (d *Deploy) PrepareNewDeploy() {
	d.updatePhase(tritonappsv1alpha1.Pending, false)

	// batches, _ := d.CalculateBatches()
	// d.Status.Batches = batches
}

func (d *Deploy) StartProgress() {
	d.MarkAsInitializing()
	d.DeployFlow.Status.StartedAt = metav1.Now()
}

func (d *Deploy) StartBatch() {
	var r int32
	if d.Spec.Action == setting.Restart {
		r = int32(len(d.NonUpdateStrategy().PodsToDelete))
		if r == 0 {

			r = *d.Spec.Application.Replicas
		}
	} else {
		r = int32(math.Abs(float64(*d.Spec.Application.Replicas - d.Status.UpdatedReplicas)))
	}
	d.DeployFlow.Status.ReplicasToProcess = r
	d.MarkAsBatchStarted()
}

func (d *Deploy) calculateBatches() (batches, batchSize int) {
	remainingReplicas := int(d.Status.ReplicasToProcess) - d.Status.FinishedReplicas
	if remainingReplicas == 0 {
		return d.Status.FinishedBatches, 0
	}

	batchSize, err := intstr.GetValueFromIntOrPercent(d.BatchSize(), int(*d.Spec.Application.Replicas), true)
	if err != nil || batchSize == 0 || batchSize >= remainingReplicas {
		batchSize = remainingReplicas
	}
	if d.Status.FinishedBatches == 0 {
		fixedSize := batchSize
		if d.CanaryEnabled() && d.Canary() < remainingReplicas {
			fixedSize = d.Canary()
		}

		batches = int(math.Ceil(float64(int(d.Status.ReplicasToProcess)-fixedSize)/float64(batchSize))) + 1

		return batches, fixedSize
	}

	batches = int(math.Ceil(float64(remainingReplicas)/float64(batchSize))) + d.Status.FinishedBatches

	return batches, batchSize
}

func (d *Deploy) MarkCurrentBatchAsStarted(startedAt metav1.Time) {
	c := d.CurrentBatchInfo()
	if c != nil {
		c.Phase = tritonappsv1alpha1.BatchSmoking
		c.StartedAt = startedAt
	} else {
		klog.Errorf("current batch is missing, condition is %v", d.Status.Conditions)
		return
	}
	d.SetCondition(*c)
}

func (d *Deploy) MarkCurrentBatchAsSmoked() {
	c := d.CurrentBatchInfo()
	if c != nil {
		c.Phase = tritonappsv1alpha1.BatchSmoked
	} else {
		klog.Errorf("current batch condition is missing, conditions is %v", d.Status.Conditions)
		return
	}
	d.SetCondition(*c)
}

func (d *Deploy) MarkCurrentBatchAsBaking() {
	c := d.CurrentBatchInfo()
	if c != nil {
		c.Phase = tritonappsv1alpha1.BatchBaking
	} else {
		klog.Errorf("current batch condition is missing, conditions is %v", d.Status.Conditions)
		return
	}
	d.SetCondition(*c)
}

func (d *Deploy) MarkCurrentBatchAsFinished() {
	c := d.CurrentBatchInfo()
	if c != nil {
		c.Phase = tritonappsv1alpha1.BatchBaked
		c.FinishedAt = metav1.Now()
	} else {
		klog.Errorf("current batch condition is missing, conditions is %v", d.Status.Conditions)
		return
	}
	d.SetCondition(*c)

	d.DeployFlow.Status.FinishedBatches += 1
	d.DeployFlow.Status.FinishedReplicas += c.BatchSize
	d.DeployFlow.Status.FailedReplicas += c.FailedReplicas
}

func (d *Deploy) PrepareNewBatch() {
	if d.AllBatchFinished() {
		return
	}

	batches, batchSize := d.calculateBatches()
	d.Status.Batches = batches

	// create new condition
	c := tritonappsv1alpha1.BatchCondition{
		Batch:     d.Status.FinishedBatches + 1,
		BatchSize: batchSize,
		Canary:    len(d.Status.Conditions) == 0 && d.CanaryEnabled(),
		Phase:     tritonappsv1alpha1.BatchPending,
	}
	d.SetCondition(c)
}

func (d *Deploy) SetPodsStatus(pods []string) {
	d.DeployFlow.Status.Pods = pods
}

func (d *Deploy) SetPulledInAt() {
	c := d.CurrentBatchInfo()
	if c == nil {
		return
	}
	c.PulledInAt = metav1.Now()

	d.SetCondition(*c)
}

func (d *Deploy) SetUpdatedAt(updatedAt metav1.Time) {
	d.DeployFlow.Status.UpdatedAt = updatedAt
}

func (d *Deploy) SetCondition(c tritonappsv1alpha1.BatchCondition) {
	cds := d.Status.Conditions

	found := false
	for i := range cds {
		if cds[i].Batch == c.Batch {
			cds[i] = c

			found = true
			break
		}
	}

	if !found {
		cds = append(cds, c)
	}

	d.DeployFlow.Status.Conditions = cds
}

func (d *Deploy) GetCondition() []tritonappsv1alpha1.BatchCondition {
	return d.Status.Conditions
}

func (d *Deploy) MarkAsInitializing() {
	d.updatePhase(tritonappsv1alpha1.Initializing, false)
}

func (d *Deploy) MarkAsBatchStarted() {
	d.updatePhase(tritonappsv1alpha1.BatchStarted, false)
}

func (d *Deploy) MarkAsBatchFinished() {
	d.updatePhase(tritonappsv1alpha1.BatchFinished, false)
}

func (d *Deploy) MarkAsPaused() {
	d.DeployFlow.Status.Paused = true
}

func (d *Deploy) MarkAsResumed() {
	d.DeployFlow.Status.Paused = false
}

func (d *Deploy) SetPaused(paused bool) {
	d.DeployFlow.Status.Paused = paused
}

func (d *Deploy) MarkAsFailed() {
	d.updateFinalPhase(tritonappsv1alpha1.Failed)
}

func (d *Deploy) MarkAsCanceled() {
	d.updateFinalPhase(tritonappsv1alpha1.Canceled)
}

func (d *Deploy) MarkAsAborted() {
	d.updateFinalPhase(tritonappsv1alpha1.Aborted)
}

func (d *Deploy) MarkAsSuccess() {
	d.updateFinalPhase(tritonappsv1alpha1.Success)
}

func (d *Deploy) updateFinalPhase(p tritonappsv1alpha1.DeployPhase) {
	d.updatePhase(p, true)
}

func (d *Deploy) updatePhase(p tritonappsv1alpha1.DeployPhase, finished bool) {
	d.DeployFlow.Status.Phase = p

	if finished {
		d.DeployFlow.Status.FinishedAt = metav1.Now()
		d.DeployFlow.Status.Finished = true
	}
}
