package helpers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetConditionStatus updates a condition while preserving reason/message of other conditions.
// It sets the specified condition to the given status and reason/message,
// and sets all other standard conditions (Ready, Progressing, Degraded) to False while preserving their existing reason/message.
func SetConditionStatus(conditions *[]metav1.Condition, conditionType string, status metav1.ConditionStatus, reason, message string, generation int64) {
	// Set the active condition
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	})

	// Set other conditions to False, preserving their reason/message
	otherTypes := []string{"Ready", "Progressing", "Degraded"}
	for _, t := range otherTypes {
		if t == conditionType {
			continue
		}
		if cond := meta.FindStatusCondition(*conditions, t); cond != nil {
			meta.SetStatusCondition(conditions, metav1.Condition{
				Type:               t,
				Status:             metav1.ConditionFalse,
				Reason:             cond.Reason,
				Message:            cond.Message,
				ObservedGeneration: generation,
			})
		}
	}
}
