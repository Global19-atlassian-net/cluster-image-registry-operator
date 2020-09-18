package operator

import (
	"fmt"

	appsapi "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchapi "k8s.io/api/batch/v1beta1"
	metaapi "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageregistryv1 "github.com/openshift/api/imageregistry/v1"
	operatorapiv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/cluster-image-registry-operator/pkg/defaults"
	"github.com/openshift/cluster-image-registry-operator/pkg/metrics"
)

func updateCondition(cr *imageregistryv1.Config, condtype string, condstate operatorapiv1.OperatorCondition) {
	found := false
	conditions := []operatorapiv1.OperatorCondition{}

	for _, c := range cr.Status.Conditions {
		if c.Type != condtype {
			conditions = append(conditions, c)
			continue
		}
		if c.Status != condstate.Status {
			c.Status = condstate.Status
			c.LastTransitionTime = metaapi.Now()
		}
		if c.Reason != condstate.Reason {
			c.Reason = condstate.Reason
		}
		if c.Message != condstate.Message {
			c.Message = condstate.Message
		}
		conditions = append(conditions, c)
		found = true
	}

	if !found {
		conditions = append(conditions, operatorapiv1.OperatorCondition{
			Type:               condtype,
			Status:             operatorapiv1.ConditionStatus(condstate.Status),
			LastTransitionTime: metaapi.Now(),
			Reason:             condstate.Reason,
			Message:            condstate.Message,
		})
	}

	cr.Status.Conditions = conditions
}

func updatePrunerCondition(cr *imageregistryv1.ImagePruner, condtype string, condstate operatorapiv1.OperatorCondition) {
	found := false
	conditions := []operatorapiv1.OperatorCondition{}

	for _, c := range cr.Status.Conditions {
		if c.Type != condtype {
			conditions = append(conditions, c)
			continue
		}
		if c.Status != condstate.Status {
			c.Status = condstate.Status
			c.LastTransitionTime = metaapi.Now()
		}
		if c.Reason != condstate.Reason {
			c.Reason = condstate.Reason
		}
		if c.Message != condstate.Message {
			c.Message = condstate.Message
		}
		conditions = append(conditions, c)
		found = true
	}

	if !found {
		conditions = append(conditions, operatorapiv1.OperatorCondition{
			Type:               condtype,
			Status:             operatorapiv1.ConditionStatus(condstate.Status),
			LastTransitionTime: metaapi.Now(),
			Reason:             condstate.Reason,
			Message:            condstate.Message,
		})
	}

	cr.Status.Conditions = conditions
}

func isDeploymentStatusAvailable(deploy *appsapi.Deployment) bool {
	return deploy.Status.AvailableReplicas > 0
}

func isDeploymentStatusComplete(deploy *appsapi.Deployment) bool {
	replicas := int32(1)
	if deploy.Spec.Replicas != nil {
		replicas = *(deploy.Spec.Replicas)
	}
	return deploy.Status.UpdatedReplicas == replicas &&
		deploy.Status.Replicas == replicas &&
		deploy.Status.AvailableReplicas == replicas &&
		deploy.Status.ObservedGeneration >= deploy.Generation
}

func (c *Controller) setStatusRemoving(cr *imageregistryv1.Config) {
	operatorProgressing := operatorapiv1.OperatorCondition{
		Status:  operatorapiv1.ConditionTrue,
		Message: "The registry is being removed",
		Reason:  "Removing",
	}

	updateCondition(cr, operatorapiv1.OperatorStatusTypeProgressing, operatorProgressing)

	operatorProgressing = operatorapiv1.OperatorCondition{
		Status:  operatorapiv1.ConditionTrue,
		Message: "The image pruner is being removed",
		Reason:  "Removing",
	}

	updateCondition(cr, operatorapiv1.OperatorStatusTypeProgressing, operatorProgressing)
}

func (c *Controller) setStatusRemoveFailed(cr *imageregistryv1.Config, removeErr error) {
	operatorDegraded := operatorapiv1.OperatorCondition{
		Status:  operatorapiv1.ConditionTrue,
		Message: fmt.Sprintf("Unable to remove registry: %s", removeErr),
		Reason:  "RemoveFailed",
	}

	updateCondition(cr, operatorapiv1.OperatorStatusTypeDegraded, operatorDegraded)
}

func (c *ImagePrunerController) syncPrunerStatus(cr *imageregistryv1.ImagePruner, applyError error, prunerJob *batchapi.CronJob, lastJobConditions []batchv1.JobCondition) {
	if prunerJob == nil {
		prunerAvailable := operatorapiv1.OperatorCondition{
			Status:  operatorapiv1.ConditionFalse,
			Message: "Pruner CronJob does not exist",
			Reason:  "Error",
		}
		updatePrunerCondition(cr, operatorapiv1.OperatorStatusTypeAvailable, prunerAvailable)
		metrics.ImagePrunerInstallStatus(false, false)
	} else {
		prunerAvailable := operatorapiv1.OperatorCondition{
			Status:  operatorapiv1.ConditionTrue,
			Message: "Pruner CronJob has been created",
			Reason:  "AsExpected",
		}
		updatePrunerCondition(cr, operatorapiv1.OperatorStatusTypeAvailable, prunerAvailable)
	}

	var foundFailed bool
	for _, condition := range lastJobConditions {
		if condition.Type == batchv1.JobFailed {
			foundFailed = true
			prunerLastJobStatus := operatorapiv1.OperatorCondition{
				Status:  operatorapiv1.ConditionTrue,
				Message: condition.Message,
				Reason:  condition.Reason,
			}
			updatePrunerCondition(cr, "Failed", prunerLastJobStatus)
		}
	}
	if !foundFailed {
		prunerLastJobStatus := operatorapiv1.OperatorCondition{
			Status:  operatorapiv1.ConditionFalse,
			Message: "Pruner completed successfully",
			Reason:  "Complete",
		}
		updatePrunerCondition(cr, "Failed", prunerLastJobStatus)
	}

	if *cr.Spec.Suspend {
		prunerJobScheduled := operatorapiv1.OperatorCondition{
			Status:  operatorapiv1.ConditionFalse,
			Message: "The pruner job has been suspended",
			Reason:  "Suspended",
		}
		updatePrunerCondition(cr, "Scheduled", prunerJobScheduled)
		if prunerJob != nil {
			metrics.ImagePrunerInstallStatus(true, false)
		}
	} else {
		prunerJobScheduled := operatorapiv1.OperatorCondition{
			Status:  operatorapiv1.ConditionTrue,
			Message: "The pruner job has been scheduled",
			Reason:  "Scheduled",
		}
		updatePrunerCondition(cr, "Scheduled", prunerJobScheduled)
		if prunerJob != nil {
			metrics.ImagePrunerInstallStatus(true, true)
		}
	}

	if applyError != nil {
		updatePrunerCondition(cr, "Degraded", operatorapiv1.OperatorCondition{
			Status:  operatorapiv1.ConditionTrue,
			Reason:  "SyncError",
			Message: fmt.Sprintf("Error: %v", applyError),
		})
	} else {
		updatePrunerCondition(cr, "Degraded", operatorapiv1.OperatorCondition{
			Status: operatorapiv1.ConditionFalse,
			Reason: "AsExpected",
		})
	}

	metrics.ImagePrunerJobStatus(foundFailed)
}

func (c *Controller) syncStatus(cr *imageregistryv1.Config, deploy *appsapi.Deployment, applyError error) {
	operatorAvailable := operatorapiv1.OperatorCondition{
		Status:  operatorapiv1.ConditionFalse,
		Message: "",
		Reason:  "",
	}
	if cr.Spec.ManagementState == operatorapiv1.Unmanaged {
		operatorAvailable.Status = operatorapiv1.ConditionTrue
		operatorAvailable.Message = "The registry configuration is set to unmanaged mode"
		operatorAvailable.Reason = "Unmanaged"
	} else if deploy == nil {
		if e, ok := applyError.(permanentError); ok {
			operatorAvailable.Message = fmt.Sprintf("Error: %s", applyError)
			operatorAvailable.Reason = e.Reason
		} else if cr.Spec.ManagementState == operatorapiv1.Removed {
			operatorAvailable.Status = operatorapiv1.ConditionTrue
			operatorAvailable.Message = "The registry is removed"
			operatorAvailable.Reason = "Removed"
		} else {
			operatorAvailable.Message = "The deployment does not exist"
			operatorAvailable.Reason = "DeploymentNotFound"
		}
	} else if deploy.DeletionTimestamp != nil {
		operatorAvailable.Message = "The deployment is being deleted"
		operatorAvailable.Reason = "DeploymentDeleted"
	} else if !isDeploymentStatusAvailable(deploy) {
		operatorAvailable.Message = "The deployment does not have available replicas"
		operatorAvailable.Reason = "NoReplicasAvailable"
	} else if !isDeploymentStatusComplete(deploy) {
		operatorAvailable.Status = operatorapiv1.ConditionTrue
		operatorAvailable.Message = "The registry has minimum availability"
		operatorAvailable.Reason = "MinimumAvailability"
	} else {
		operatorAvailable.Status = operatorapiv1.ConditionTrue
		operatorAvailable.Message = "The registry is ready"
		operatorAvailable.Reason = "Ready"
	}

	updateCondition(cr, operatorapiv1.OperatorStatusTypeAvailable, operatorAvailable)

	operatorProgressing := operatorapiv1.OperatorCondition{
		Status:  operatorapiv1.ConditionTrue,
		Message: "",
		Reason:  "",
	}
	if cr.Spec.ManagementState == operatorapiv1.Unmanaged {
		operatorProgressing.Status = operatorapiv1.ConditionFalse
		operatorProgressing.Message = "The registry configuration is set to unmanaged mode"
		operatorProgressing.Reason = "Unmanaged"
	} else if cr.Spec.ManagementState == operatorapiv1.Removed {
		if deploy != nil {
			operatorProgressing.Message = "The deployment is being removed"
			operatorProgressing.Reason = "DeletingDeployment"
		} else {
			operatorProgressing.Status = operatorapiv1.ConditionFalse
			operatorProgressing.Message = "All registry resources are removed"
			operatorProgressing.Reason = "Removed"
		}
	} else if applyError != nil {
		if _, ok := applyError.(permanentError); ok {
			operatorProgressing.Status = operatorapiv1.ConditionFalse
		}
		operatorProgressing.Message = fmt.Sprintf("Unable to apply resources: %s", applyError)
		operatorProgressing.Reason = "Error"
	} else if deploy == nil {
		operatorProgressing.Message = "All resources are successfully applied, but the deployment does not exist"
		operatorProgressing.Reason = "WaitingForDeployment"
	} else if deploy.DeletionTimestamp != nil {
		operatorProgressing.Message = "The deployment is being deleted"
		operatorProgressing.Reason = "FinalizingDeployment"
	} else if !isDeploymentStatusComplete(deploy) {
		operatorProgressing.Message = "The deployment has not completed"
		operatorProgressing.Reason = "DeploymentNotCompleted"
	} else {
		operatorProgressing.Status = operatorapiv1.ConditionFalse
		operatorProgressing.Message = "The registry is ready"
		operatorProgressing.Reason = "Ready"
	}

	updateCondition(cr, operatorapiv1.OperatorStatusTypeProgressing, operatorProgressing)

	operatorDegraded := operatorapiv1.OperatorCondition{
		Status:  operatorapiv1.ConditionFalse,
		Message: "",
		Reason:  "",
	}
	if cr.Spec.ManagementState == operatorapiv1.Unmanaged {
		operatorDegraded.Message = "The registry configuration is set to unmanaged mode"
		operatorDegraded.Reason = "Unmanaged"
	} else if e, ok := applyError.(permanentError); ok {
		operatorDegraded.Status = operatorapiv1.ConditionTrue
		operatorDegraded.Message = fmt.Sprintf("Error: %s", applyError)
		operatorDegraded.Reason = e.Reason
	} else if cr.Spec.ManagementState == operatorapiv1.Removed {
		operatorDegraded.Message = "The registry is removed"
		operatorDegraded.Reason = "Removed"
	}

	updateCondition(cr, operatorapiv1.OperatorStatusTypeDegraded, operatorDegraded)

	operatorRemoved := operatorapiv1.OperatorCondition{
		Status:  operatorapiv1.ConditionFalse,
		Message: "",
		Reason:  "",
	}
	if cr.Spec.ManagementState == operatorapiv1.Removed {
		operatorRemoved.Status = operatorapiv1.ConditionTrue
		operatorRemoved.Message = "The registry is removed"
		operatorRemoved.Reason = "Removed"
	}

	updateCondition(cr, defaults.OperatorStatusTypeRemoved, operatorRemoved)
}
