package registry

import (
	"testing"
	"time"
)

func TestRefreshCatalogSeparatesExpiredFromCurrent(t *testing.T) {
	current := validEntry("current")
	expired := validEntry("expired")
	expired.ObservedAt = expired.ObservedAt.Add(-72 * time.Hour)
	expired.ExpiresAt = expired.ObservedAt.Add(time.Hour)
	report, err := RefreshCatalog([]IntakeEntry{expired, current})
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 2 || report.Current != 1 || report.Rejected != 1 {
		t.Fatalf("refresh = %+v", report)
	}
	if report.Entries[0].EntryID != "current" || !report.Entries[0].Valid {
		t.Fatalf("current row = %+v", report.Entries[0])
	}
	if report.Entries[1].EntryID != "expired" || report.Entries[1].Valid {
		t.Fatalf("expired row = %+v", report.Entries[1])
	}
}

func TestRefreshCatalogRejectsReplayAmongCurrentEntries(t *testing.T) {
	first := validEntry("a")
	second := validEntry("b")
	second.ChallengeNonce = first.ChallengeNonce
	report, err := RefreshCatalog([]IntakeEntry{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if report.Current != 1 || report.Rejected != 1 {
		t.Fatalf("replay refresh = %+v", report)
	}
}
