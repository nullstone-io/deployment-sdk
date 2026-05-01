package failures

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// classifyVolumeEvent handles §4 (Storage) when the trigger is an Event with a
// volume-related reason. PVC-Pending and StatefulSet-stuck cases are surfaced
// via the same path because they emit FailedMount/FailedAttachVolume events.
func classifyVolumeEvent(obj ObjectRef, ev corev1.Event) *Failure {
	switch ev.Reason {
	case "FailedMount":
		return mountFailure(obj, ev, "FailedMount")
	case "FailedAttachVolume":
		return mountFailure(obj, ev, "FailedAttachVolume")
	case "ProvisioningFailed", "FailedBinding":
		return &Failure{
			Name:        "PVCProvisioningFailed",
			Category:    CategoryStorage,
			Summary:     "PVC could not be provisioned by its StorageClass",
			Remediation: "Check the StorageClass exists and the cloud disk quota / zone is correct; prefer volumeBindingMode: WaitForFirstConsumer for zonal volumes.",
			Object:      obj,
			Signals:     Signals{EventReason: ev.Reason, EventMessage: ev.Message},
			Provider:    storageProvider(ev.Message),
			Docs:        []string{"https://kubernetes.io/docs/concepts/storage/persistent-volumes/"},
		}
	case "VolumeResizeFailed":
		return &Failure{
			Name:        "VolumeResizeFailed",
			Category:    CategoryStorage,
			Summary:     "Online volume resize failed",
			Remediation: "Inspect the CSI driver logs; some drivers require a pod restart for FS expansion.",
			Object:      obj,
			Signals:     Signals{EventReason: ev.Reason, EventMessage: ev.Message},
			Provider:    storageProvider(ev.Message),
		}
	}
	return nil
}

// mountFailure sub-classifies FailedMount / FailedAttachVolume by message content.
// Provider tag comes from message keywords (EBS / Azure Disk / GCE PD).
func mountFailure(obj ObjectRef, ev corev1.Event, reason string) *Failure {
	lower := strings.ToLower(ev.Message)
	signals := Signals{EventReason: reason, EventMessage: ev.Message}
	provider := storageProvider(ev.Message)

	switch {
	case strings.Contains(lower, "multi-attach error"):
		return &Failure{
			Name:        reason + "/MultiAttach",
			Category:    CategoryStorage,
			Summary:     "RWO volume is still attached to a previous node",
			Remediation: "Wait for the prior pod to terminate; consider force-detach if the previous node is gone.",
			Object:      obj, Signals: signals, Provider: provider,
		}
	case strings.Contains(lower, "volumelimitexceeded"):
		return &Failure{
			Name:        reason + "/VolumeLimit",
			Category:    CategoryStorage,
			Summary:     "Node has hit its per-instance attached-volume cap",
			Remediation: "Schedule onto a larger instance type, or reduce volumes per pod.",
			Object:      obj, Signals: signals, Provider: provider,
		}
	case strings.Contains(lower, "unauthorizedoperation"), strings.Contains(lower, "authorizationfailed"), strings.Contains(lower, "requires one of"):
		return &Failure{
			Name:        reason + "/IAM",
			Category:    CategoryStorage,
			Summary:     "Cloud IAM does not grant the cluster permission to attach the disk",
			Remediation: "Grant the node/cluster identity the appropriate role (EBS attach, Compute disk attach, Network Contributor).",
			Object:      obj, Signals: signals, Provider: provider,
		}
	case strings.Contains(lower, "quota_exceeded"), strings.Contains(lower, "quota exceeded"):
		return &Failure{
			Name:        reason + "/Quota",
			Category:    CategoryStorage,
			Summary:     "Cloud disk quota exceeded",
			Remediation: "Request a quota increase or release unused disks.",
			Object:      obj, Signals: signals, Provider: provider,
		}
	case strings.Contains(lower, "does not exist"):
		return &Failure{
			Name:        reason + "/Missing",
			Category:    CategoryStorage,
			Summary:     "Underlying disk no longer exists",
			Remediation: "Restore from snapshot, or recreate the PV/PVC.",
			Object:      obj, Signals: signals, Provider: provider,
		}
	case strings.Contains(lower, "timed out waiting for the condition"):
		return &Failure{
			Name:        reason + "/Timeout",
			Category:    CategoryStorage,
			Summary:     "CSI driver timed out attaching/mounting the volume",
			Remediation: "Inspect the CSI controller pod logs; check cloud-side queue for the disk operation.",
			Object:      obj, Signals: signals, Provider: provider,
		}
	}
	return &Failure{
		Name:        reason,
		Category:    CategoryStorage,
		Summary:     "Volume could not be attached/mounted",
		Remediation: "Inspect the CSI driver logs and the underlying cloud volume state.",
		Object:      obj, Signals: signals, Provider: provider,
	}
}

// storageProvider tags storage failures by recognizing provider keywords in the
// event message (azure / disk.csi.azure.com / ebs.csi.aws.com / pd.csi.storage.gke.io).
func storageProvider(msg string) Provider {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "ebs.csi.aws.com"), strings.Contains(lower, "ebs"):
		return ProviderEKS
	case strings.Contains(lower, "disk.csi.azure.com"), strings.Contains(lower, "azure"):
		return ProviderAKS
	case strings.Contains(lower, "pd.csi.storage.gke.io"), strings.Contains(lower, "gce-pd"):
		return ProviderGKE
	}
	return ProviderGeneric
}
