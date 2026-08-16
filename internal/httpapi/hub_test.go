package httpapi

import "testing"

func TestHubConversationDeletionSurvivesAFullWakeBuffer(t *testing.T) {
	h := newHub()
	subscriptionID, _, cancel := h.subscribe()
	defer cancel()
	for range 65 {
		h.publish(1)
	}
	h.publishConversationDeleted("org_1", "con_1")
	h.publishConversationDeleted("org_1", "con_2")

	changed := h.takeChangedConversations(subscriptionID)
	if _, hasFirst := changed["org_1"]["con_1"]; !hasFirst {
		t.Fatalf("changed conversations = %#v", changed)
	}
	if _, hasSecond := changed["org_1"]["con_2"]; !hasSecond {
		t.Fatalf("changed conversations = %#v", changed)
	}
}
