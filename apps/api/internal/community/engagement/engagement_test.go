package engagement

import (
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
)

func TestSubscribedThreshold(t *testing.T) {
	if toState(1, &communityclient.ThreadUserView{NotificationLevel: 0}).Subscribed {
		t.Error("muted is subscribed")
	}
	if toState(1, &communityclient.ThreadUserView{NotificationLevel: 1}).Subscribed {
		t.Error("normal is subscribed")
	}
	if !toState(1, &communityclient.ThreadUserView{NotificationLevel: 3}).Subscribed {
		t.Error("watching is not subscribed")
	}
}
